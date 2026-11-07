package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// statusPageEnvelope is the `{ "data": ... }` wrapper the API returns.
type statusPageEnvelope struct {
	Data statusPageData `json:"data"`
}

// statusPageData is the JSON shape of a status page detail response. We decode
// it manually (instead of using larmgo's generated types) because the
// generated `StatusPage.Components` is `[]StatusPageTreeEntry` — a `oneOf`
// wrapper that requires per-element discrimination. Custom unmarshalling here
// is simpler than fighting the union helpers.
type statusPageData struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Slug               string      `json:"slug"`
	Description        *string     `json:"description,omitempty"`
	PrimaryColor       *string     `json:"primary_color,omitempty"`
	Theme              *string     `json:"theme,omitempty"`
	Enabled            bool        `json:"enabled"`
	SubscribersEnabled *bool       `json:"subscribers_enabled,omitempty"`
	URL                *string     `json:"url,omitempty"`
	CustomDomain       *string     `json:"custom_domain,omitempty"`
	DomainStatus       *string     `json:"domain_status,omitempty"`
	LogoLightURL       *string     `json:"logo_light_url,omitempty"`
	LogoDarkURL        *string     `json:"logo_dark_url,omitempty"`
	InsertedAt         time.Time   `json:"inserted_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	Components         []treeEntry `json:"components"`
}

// treeEntry is one node in the polymorphic components tree. `Type` discriminates.
type treeEntry struct {
	Type        string         `json:"type"`
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Position    int            `json:"position"`
	Components  []treeEntry    `json:"components,omitempty"`
	Monitors    []monitorEntry `json:"monitors,omitempty"`
	InsertedAt  time.Time      `json:"inserted_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type monitorEntry struct {
	MonitorID  string `json:"monitor_id"`
	DownStatus string `json:"down_status"`
}

func monitorObjectType() attr.Type {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"monitor_id":  types.StringType,
			"down_status": types.StringType,
		},
	}
}

func innerComponentObjectType() attr.Type {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":          types.StringType,
			"name":        types.StringType,
			"description": types.StringType,
			"monitors":    types.ListType{ElemType: monitorObjectType()},
		},
	}
}

func topLevelEntryObjectType() attr.Type {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"type":        types.StringType,
			"id":          types.StringType,
			"name":        types.StringType,
			"description": types.StringType,
			"components":  types.ListType{ElemType: innerComponentObjectType()},
			"monitors":    types.ListType{ElemType: monitorObjectType()},
		},
	}
}

// stateFromAPI converts the decoded API response into the Terraform state model.
func stateFromAPI(page statusPageData) (statusPageModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	componentsValue, d := componentsListFromAPI(page.Components)
	diags.Append(d...)
	if diags.HasError() {
		return statusPageModel{}, diags
	}

	model := statusPageModel{
		ID:                 types.StringValue(page.ID),
		Name:               types.StringValue(page.Name),
		Slug:               types.StringValue(page.Slug),
		Description:        optionalString(page.Description),
		PrimaryColor:       optionalString(page.PrimaryColor),
		Theme:              optionalString(page.Theme),
		Enabled:            types.BoolValue(page.Enabled),
		SubscribersEnabled: optionalBool(page.SubscribersEnabled),
		URL:                optionalString(page.URL),
		CustomDomain:       optionalString(page.CustomDomain),
		DomainStatus:       optionalString(page.DomainStatus),
		LogoLightURL:       optionalString(page.LogoLightURL),
		LogoDarkURL:        optionalString(page.LogoDarkURL),
		InsertedAt:         formatTime(page.InsertedAt),
		UpdatedAt:          formatTime(page.UpdatedAt),
		Components:         componentsValue,
	}
	return model, diags
}

func optionalBool(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

func componentsListFromAPI(entries []treeEntry) (types.List, diag.Diagnostics) {
	values := make([]attr.Value, 0, len(entries))
	for _, entry := range entries {
		objValue, diags := topLevelEntryValue(entry)
		if diags.HasError() {
			return types.ListNull(topLevelEntryObjectType()), diags
		}
		values = append(values, objValue)
	}
	return types.ListValueMust(topLevelEntryObjectType(), values), nil
}

func topLevelEntryValue(entry treeEntry) (attr.Value, diag.Diagnostics) {
	innerComponents := types.ListValueMust(innerComponentObjectType(), []attr.Value{})
	monitors := types.ListValueMust(monitorObjectType(), []attr.Value{})

	switch entry.Type {
	case entryTypeGroup:
		innerValues := make([]attr.Value, 0, len(entry.Components))
		for _, child := range entry.Components {
			innerValues = append(innerValues, innerComponentValue(child))
		}
		innerComponents = types.ListValueMust(innerComponentObjectType(), innerValues)

	case entryTypeComponent:
		monitors = monitorListValue(entry.Monitors)
	}

	return types.ObjectValue(topLevelEntryObjectType().(types.ObjectType).AttrTypes, map[string]attr.Value{
		"type":        types.StringValue(entry.Type),
		"id":          types.StringValue(entry.ID),
		"name":        types.StringValue(entry.Name),
		"description": optionalString(entry.Description),
		"components":  innerComponents,
		"monitors":    monitors,
	})
}

func innerComponentValue(c treeEntry) attr.Value {
	return types.ObjectValueMust(
		innerComponentObjectType().(types.ObjectType).AttrTypes,
		map[string]attr.Value{
			"id":          types.StringValue(c.ID),
			"name":        types.StringValue(c.Name),
			"description": optionalString(c.Description),
			"monitors":    monitorListValue(c.Monitors),
		},
	)
}

func monitorListValue(monitors []monitorEntry) types.List {
	values := make([]attr.Value, 0, len(monitors))
	for _, m := range monitors {
		values = append(values, types.ObjectValueMust(
			monitorObjectType().(types.ObjectType).AttrTypes,
			map[string]attr.Value{
				"monitor_id":  types.StringValue(m.MonitorID),
				"down_status": types.StringValue(m.DownStatus),
			},
		))
	}
	return types.ListValueMust(monitorObjectType(), values)
}

// topLevelEntryPlan is the in-memory shape of one top-level entry from the plan.
type topLevelEntryPlan struct {
	Type        types.String `tfsdk:"type"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Components  types.List   `tfsdk:"components"`
	Monitors    types.List   `tfsdk:"monitors"`
}

type innerComponentPlan struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Monitors    types.List   `tfsdk:"monitors"`
}

type monitorPlan struct {
	MonitorID  types.String `tfsdk:"monitor_id"`
	DownStatus types.String `tfsdk:"down_status"`
}

// priorIDs hands out the IDs of existing groups and components by name, so
// entries that are reordered or moved between groups keep their identity on
// the backend. Plan IDs can't be used: list elements are matched by index, so
// a reorder would pair an entry with another entry's ID.
type priorIDs struct {
	groups     idsByName
	components idsByName
}

// idsByName queues IDs per name; duplicate names are handed out in list order.
type idsByName map[string][]uuid.UUID

func priorIDsFromState(ctx context.Context, components types.List) (priorIDs, diag.Diagnostics) {
	var diags diag.Diagnostics
	ids := priorIDs{groups: idsByName{}, components: idsByName{}}
	if components.IsNull() || components.IsUnknown() {
		return ids, diags
	}

	var entries []topLevelEntryPlan
	diags.Append(components.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return ids, diags
	}

	for _, entry := range entries {
		switch entry.Type.ValueString() {
		case entryTypeGroup:
			diags.Append(ids.groups.add(entry.Name, entry.ID)...)

			var children []innerComponentPlan
			if !entry.Components.IsNull() {
				diags.Append(entry.Components.ElementsAs(ctx, &children, false)...)
			}
			for _, child := range children {
				diags.Append(ids.components.add(child.Name, child.ID)...)
			}

		case entryTypeComponent:
			diags.Append(ids.components.add(entry.Name, entry.ID)...)
		}
		if diags.HasError() {
			return ids, diags
		}
	}
	return ids, diags
}

func (m idsByName) add(name, id types.String) diag.Diagnostics {
	var diags diag.Diagnostics
	parsed, err := uuid.Parse(id.ValueString())
	if err != nil {
		diags.AddError("Invalid status page entry ID in state", err.Error())
		return diags
	}
	m[name.ValueString()] = append(m[name.ValueString()], parsed)
	return diags
}

// take returns the next unused prior ID for name, or nil when the entry is new.
func (m idsByName) take(name types.String) *uuid.UUID {
	queue := m[name.ValueString()]
	if len(queue) == 0 {
		return nil
	}
	m[name.ValueString()] = queue[1:]
	return &queue[0]
}

// structureBodyFromPlan converts the plan's components into the API request body,
// reusing IDs from prior state so the backend updates existing rows.
func structureBodyFromPlan(ctx context.Context, plan statusPageModel, ids priorIDs) (larmgo.ReplaceStatusPageStructureJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := larmgo.ReplaceStatusPageStructureJSONRequestBody{}

	if plan.Components.IsNull() || plan.Components.IsUnknown() {
		body.Components = []larmgo.StatusPageStructureEntry{}
		return body, diags
	}

	var entries []topLevelEntryPlan
	diags.Append(plan.Components.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return body, diags
	}

	apiEntries := make([]larmgo.StatusPageStructureEntry, 0, len(entries))
	for _, entry := range entries {
		apiEntry, d := planEntryToAPI(ctx, entry, ids)
		diags.Append(d...)
		if diags.HasError() {
			return body, diags
		}
		apiEntries = append(apiEntries, apiEntry)
	}
	body.Components = apiEntries
	return body, diags
}

func planEntryToAPI(ctx context.Context, entry topLevelEntryPlan, ids priorIDs) (larmgo.StatusPageStructureEntry, diag.Diagnostics) {
	var diags diag.Diagnostics
	var apiEntry larmgo.StatusPageStructureEntry

	switch entry.Type.ValueString() {
	case entryTypeGroup:
		groupEntry := larmgo.StatusPageStructureGroupEntry{
			Type: larmgo.StatusPageStructureGroupEntryType(entryTypeGroup),
			Name: entry.Name.ValueString(),
			Id:   ids.groups.take(entry.Name),
		}

		var children []innerComponentPlan
		if !entry.Components.IsNull() && !entry.Components.IsUnknown() {
			diags.Append(entry.Components.ElementsAs(ctx, &children, false)...)
			if diags.HasError() {
				return apiEntry, diags
			}
		}

		apiChildren := make([]larmgo.StatusPageStructureComponentEntry, 0, len(children))
		for _, child := range children {
			c, d := innerComponentPlanToAPI(ctx, child, ids)
			diags.Append(d...)
			if diags.HasError() {
				return apiEntry, diags
			}
			apiChildren = append(apiChildren, c)
		}
		groupEntry.Components = apiChildren

		if err := apiEntry.FromStatusPageStructureGroupEntry(groupEntry); err != nil {
			diags.AddError("Failed to encode group entry", err.Error())
		}

	case entryTypeComponent:
		comp := larmgo.StatusPageStructureComponentEntry{
			Type: larmgo.StatusPageStructureComponentEntryType(entryTypeComponent),
			Name: entry.Name.ValueString(),
			Id:   ids.components.take(entry.Name),
		}
		if !entry.Description.IsNull() && !entry.Description.IsUnknown() {
			comp.Description = entry.Description.ValueStringPointer()
		}

		if err := setComponentMonitors(ctx, &comp, entry.Monitors); err != nil {
			diags.AddError("Invalid component monitor", err.Error())
			return apiEntry, diags
		}

		if err := apiEntry.FromStatusPageStructureComponentEntry(comp); err != nil {
			diags.AddError("Failed to encode component entry", err.Error())
		}
	}

	return apiEntry, diags
}

func innerComponentPlanToAPI(ctx context.Context, child innerComponentPlan, ids priorIDs) (larmgo.StatusPageStructureComponentEntry, diag.Diagnostics) {
	var diags diag.Diagnostics
	comp := larmgo.StatusPageStructureComponentEntry{
		Type: larmgo.StatusPageStructureComponentEntryType(entryTypeComponent),
		Name: child.Name.ValueString(),
		Id:   ids.components.take(child.Name),
	}
	if !child.Description.IsNull() && !child.Description.IsUnknown() {
		comp.Description = child.Description.ValueStringPointer()
	}

	if err := setComponentMonitors(ctx, &comp, child.Monitors); err != nil {
		diags.AddError("Invalid component monitor", err.Error())
	}

	return comp, diags
}

// setComponentMonitors converts a plan monitor list into the anonymous struct
// shape required by larmgo's generated `StatusPageStructureComponentEntry.Monitors`
// field. The struct field name `MonitorId` is fixed by the oapi-codegen output;
// we use JSON round-tripping to assign without naming a struct literal of the
// generated type ourselves (and to keep this code free of `//nolint` markers).
func setComponentMonitors(ctx context.Context, comp *larmgo.StatusPageStructureComponentEntry, list types.List) error {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}

	var monitors []monitorPlan
	if diags := list.ElementsAs(ctx, &monitors, false); diags.HasError() {
		return fmt.Errorf("failed to read monitors from plan: %s", diags)
	}
	if len(monitors) == 0 {
		return nil
	}

	type wire struct {
		MonitorID  string  `json:"monitor_id"`
		DownStatus *string `json:"down_status,omitempty"`
	}
	wired := make([]wire, 0, len(monitors))
	for _, m := range monitors {
		w := wire{MonitorID: m.MonitorID.ValueString()}
		if !m.DownStatus.IsNull() && !m.DownStatus.IsUnknown() {
			ds := m.DownStatus.ValueString()
			w.DownStatus = &ds
		}
		wired = append(wired, w)
	}

	encoded, err := json.Marshal(wired)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, &comp.Monitors)
}
