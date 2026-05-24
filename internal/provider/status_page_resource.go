package provider

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

var (
	_ resource.Resource                = &statusPageResource{}
	_ resource.ResourceWithConfigure   = &statusPageResource{}
	_ resource.ResourceWithImportState = &statusPageResource{}
)

const (
	entryTypeGroup     = "group"
	entryTypeComponent = "component"

	downStatusDegraded = "degraded_performance"
	downStatusPartial  = "partial_outage"
	downStatusMajor    = "major_outage"
)

// NewStatusPageResource is the constructor referenced by the provider.
func NewStatusPageResource() resource.Resource {
	return &statusPageResource{}
}

type statusPageResource struct {
	client *larmgo.ClientWithResponses
}

type statusPageModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Slug               types.String `tfsdk:"slug"`
	Description        types.String `tfsdk:"description"`
	PrimaryColor       types.String `tfsdk:"primary_color"`
	Theme              types.String `tfsdk:"theme"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	SubscribersEnabled types.Bool   `tfsdk:"subscribers_enabled"`
	URL                types.String `tfsdk:"url"`
	CustomDomain       types.String `tfsdk:"custom_domain"`
	DomainStatus       types.String `tfsdk:"domain_status"`
	LogoLightURL       types.String `tfsdk:"logo_light_url"`
	LogoDarkURL        types.String `tfsdk:"logo_dark_url"`
	InsertedAt         types.String `tfsdk:"inserted_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
	Components         types.List   `tfsdk:"components"`
}

func (r *statusPageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

func (r *statusPageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Larm status page and its full structure (groups, components, monitor links) as one resource. " +
			"The `components` attribute is the ordered tree shown on the page. " +
			"Top-level entries are either groups (which contain components) or ungrouped components; they interleave in the order written.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Status page ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name.",
				Required:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL slug. Used in the page's public URL (`<slug>.status.larm.dev`). " +
					"Must be lowercase alphanumeric with hyphens, 3-63 chars, starting and ending with a letter or number. " +
					"Changing the slug rewrites the public URL.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 63),
					stringvalidator.RegexMatches(slugRegex, "must be lowercase alphanumeric with hyphens, starting and ending with a letter or number"),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description displayed on the page.",
				Optional:            true,
			},
			"primary_color": schema.StringAttribute{
				MarkdownDescription: "Brand color as a hex string (`#RRGGBB`).",
				Optional:            true,
			},
			"theme": schema.StringAttribute{
				MarkdownDescription: "Page theme. One of `light`, `dark`, `system`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("system"),
				Validators: []validator.String{
					stringvalidator.OneOf("light", "dark", "system"),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the page is published. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"subscribers_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether email subscribers can sign up to the page. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Computed public URL of the page.",
				Computed:            true,
			},
			"custom_domain": schema.StringAttribute{
				MarkdownDescription: "Custom domain configured for the page, if any. Manage via the dashboard.",
				Computed:            true,
			},
			"domain_status": schema.StringAttribute{
				MarkdownDescription: "Status of the custom domain provisioning, if any.",
				Computed:            true,
			},
			"logo_light_url": schema.StringAttribute{
				MarkdownDescription: "URL of the logo shown on light backgrounds. Manage via the dashboard.",
				Computed:            true,
			},
			"logo_dark_url": schema.StringAttribute{
				MarkdownDescription: "URL of the logo shown on dark backgrounds. Manage via the dashboard.",
				Computed:            true,
			},
			"inserted_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
			"components": schema.ListNestedAttribute{
				MarkdownDescription: "Ordered tree of groups and ungrouped components. List order is display order on the page.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: topLevelEntryAttributes(),
				},
			},
		},
	}
}

func topLevelEntryAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			MarkdownDescription: "Entry kind. One of `group`, `component`.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(entryTypeGroup, entryTypeComponent),
			},
		},
		"id": schema.StringAttribute{
			MarkdownDescription: "Entry ID (computed).",
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Display name.",
			Required:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "Component description. Only meaningful when `type = \"component\"`.",
			Optional:            true,
		},
		"components": schema.ListNestedAttribute{
			MarkdownDescription: "Components inside this group (in display order). Only valid when `type = \"group\"`.",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: innerComponentAttributes(),
			},
			Validators: []validator.List{
				listvalidator.SizeAtLeast(0),
			},
		},
		"monitors": schema.ListNestedAttribute{
			MarkdownDescription: "Monitors linked to this component. Only valid when `type = \"component\"`.",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: monitorAttributes(),
			},
		},
	}
}

func innerComponentAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Display name.",
			Required:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "Component description.",
			Optional:            true,
		},
		"monitors": schema.ListNestedAttribute{
			MarkdownDescription: "Monitors linked to this component.",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: monitorAttributes(),
			},
		},
	}
}

func monitorAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"monitor_id": schema.StringAttribute{
			MarkdownDescription: "Monitor ID to link.",
			Required:            true,
		},
		"down_status": schema.StringAttribute{
			MarkdownDescription: "Status to display when this monitor is down. One of `degraded_performance`, `partial_outage`, `major_outage`. Defaults to `major_outage`.",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString(downStatusMajor),
			Validators: []validator.String{
				stringvalidator.OneOf(downStatusDegraded, downStatusPartial, downStatusMajor),
			},
		},
	}
}

func (r *statusPageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, diags := clientFromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.client = client
}

func (r *statusPageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusPageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := statusPageInputFromPlan(plan)

	createResp, err := r.client.CreateStatusPageWithResponse(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create status page", err.Error())
		return
	}

	if createResp.StatusCode() != http.StatusCreated {
		resp.Diagnostics.AddError("Create status page: unexpected response", responseError(createResp.StatusCode(), createResp.Body))
		return
	}

	var created statusPageEnvelope
	if err := json.Unmarshal(createResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Failed to decode create response", err.Error())
		return
	}

	id, err := uuid.Parse(created.Data.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid status page ID from API", err.Error())
		return
	}

	// If the plan declared a structure, replace the (empty) structure with it.
	finalPage := created.Data
	if !plan.Components.IsNull() && !plan.Components.IsUnknown() {
		structurePayload, diags := structureBodyFromPlan(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		structuredPage, diags := r.replaceStructure(ctx, id, structurePayload)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		finalPage = structuredPage
	}

	state, diags := stateFromAPI(ctx, finalPage)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *statusPageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid status page ID in state", err.Error())
		return
	}

	getResp, err := r.client.GetStatusPageWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read status page", err.Error())
		return
	}

	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError("Read status page: unexpected response", responseError(getResp.StatusCode(), getResp.Body))
		return
	}

	var fetched statusPageEnvelope
	if err := json.Unmarshal(getResp.Body, &fetched); err != nil {
		resp.Diagnostics.AddError("Failed to decode read response", err.Error())
		return
	}

	newState, diags := stateFromAPI(ctx, fetched.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *statusPageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan statusPageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid status page ID in state", err.Error())
		return
	}

	// Update page metadata.
	input := statusPageInputFromPlan(plan)
	updateResp, err := r.client.UpdateStatusPageWithResponse(ctx, id, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update status page", err.Error())
		return
	}
	if updateResp.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError("Update status page: unexpected response", responseError(updateResp.StatusCode(), updateResp.Body))
		return
	}

	// Replace the structure. We always send the full structure so removals are honored.
	structurePayload, diags := structureBodyFromPlan(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updatedPage, diags := r.replaceStructure(ctx, id, structurePayload)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := stateFromAPI(ctx, updatedPage)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *statusPageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid status page ID in state", err.Error())
		return
	}

	deleteResp, err := r.client.DeleteStatusPageWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete status page", err.Error())
		return
	}

	if deleteResp.StatusCode() != http.StatusNoContent && deleteResp.StatusCode() != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete status page: unexpected response", responseError(deleteResp.StatusCode(), deleteResp.Body))
	}
}

func (r *statusPageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ── HTTP helpers ─────────────────────────────────────────────────────────────

// replaceStructure calls PUT /status-pages/:id/structure and returns the decoded page.
func (r *statusPageResource) replaceStructure(
	ctx context.Context,
	id uuid.UUID,
	body larmgo.ReplaceStatusPageStructureJSONRequestBody,
) (statusPageData, diag.Diagnostics) {
	var diags diag.Diagnostics

	structResp, err := r.client.ReplaceStatusPageStructureWithResponse(ctx, id, body)
	if err != nil {
		diags.AddError("Failed to replace status page structure", err.Error())
		return statusPageData{}, diags
	}
	if structResp.StatusCode() != http.StatusOK {
		diags.AddError("Replace structure: unexpected response", responseError(structResp.StatusCode(), structResp.Body))
		return statusPageData{}, diags
	}

	var envelope statusPageEnvelope
	if err := json.Unmarshal(structResp.Body, &envelope); err != nil {
		diags.AddError("Failed to decode structure response", err.Error())
		return statusPageData{}, diags
	}
	return envelope.Data, diags
}

// statusPageInputFromPlan builds the metadata-only payload for create/update.
// The structure (components) is sent separately via the structure endpoint.
func statusPageInputFromPlan(plan statusPageModel) larmgo.StatusPageInput {
	input := larmgo.StatusPageInput{
		Name: plan.Name.ValueStringPointer(),
		Slug: plan.Slug.ValueStringPointer(),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}
	if !plan.PrimaryColor.IsNull() && !plan.PrimaryColor.IsUnknown() {
		input.PrimaryColor = plan.PrimaryColor.ValueStringPointer()
	}
	if !plan.Theme.IsNull() && !plan.Theme.IsUnknown() {
		theme := larmgo.StatusPageTheme(plan.Theme.ValueString())
		input.Theme = &theme
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		input.Enabled = plan.Enabled.ValueBoolPointer()
	}
	if !plan.SubscribersEnabled.IsNull() && !plan.SubscribersEnabled.IsUnknown() {
		input.SubscribersEnabled = plan.SubscribersEnabled.ValueBoolPointer()
	}
	return input
}
