package provider

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

var (
	_ resource.Resource                = &monitorResource{}
	_ resource.ResourceWithConfigure   = &monitorResource{}
	_ resource.ResourceWithImportState = &monitorResource{}
)

// NewMonitorResource is the constructor referenced by the provider.
func NewMonitorResource() resource.Resource {
	return &monitorResource{}
}

type monitorResource struct {
	client *larmgo.ClientWithResponses
}

type monitorModel struct {
	ID                 types.String         `tfsdk:"id"`
	Name               types.String         `tfsdk:"name"`
	CheckType          types.String         `tfsdk:"check_type"`
	Enabled            types.Bool           `tfsdk:"enabled"`
	IntervalSeconds    types.Int64          `tfsdk:"interval_seconds"`
	TimeoutMs          types.Int64          `tfsdk:"timeout_ms"`
	Config             jsontypes.Normalized `tfsdk:"config"`
	ConfirmDownMinutes types.Int64          `tfsdk:"confirm_down_minutes"`
	ConfirmUpMinutes   types.Int64          `tfsdk:"confirm_up_minutes"`
	ConfirmDownAfter   types.Int64          `tfsdk:"confirm_down_after"`
	ConfirmUpAfter     types.Int64          `tfsdk:"confirm_up_after"`
	AlertChannelIDs    types.Set            `tfsdk:"alert_channel_ids"`
	CurrentState       types.String         `tfsdk:"current_state"`
	HeartbeatToken     types.String         `tfsdk:"heartbeat_token"`
	InsertedAt         types.String         `tfsdk:"inserted_at"`
	UpdatedAt          types.String         `tfsdk:"updated_at"`
}

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Larm monitor. The `config` attribute is polymorphic per `check_type` — use `jsonencode({...})` to pass the configuration map.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Monitor ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable monitor name.",
				Required:            true,
			},
			"check_type": schema.StringAttribute{
				MarkdownDescription: "Type of check. One of `http`, `tcp`, `dns`, `heartbeat`, `synthetic`. Changing this forces replacement.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("http", "tcp", "dns", "heartbeat", "synthetic"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the monitor is enabled. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "Seconds between checks. Defaults to `180`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(180),
			},
			"timeout_ms": schema.Int64Attribute{
				MarkdownDescription: "Per-check timeout in milliseconds. Defaults to `10000`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(10000),
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "Check-type-specific configuration as a JSON-encoded string. Use `jsonencode({...})`. Compared semantically, so formatting differences do not produce diffs.",
				Required:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
			"confirm_down_minutes": schema.Int64Attribute{
				MarkdownDescription: "Minutes a non-synthetic monitor must be failing before going DOWN.",
				Optional:            true,
				Computed:            true,
			},
			"confirm_up_minutes": schema.Int64Attribute{
				MarkdownDescription: "Minutes a non-synthetic monitor must be passing before going UP.",
				Optional:            true,
				Computed:            true,
			},
			"confirm_down_after": schema.Int64Attribute{
				MarkdownDescription: "Consecutive failed runs a synthetic monitor needs before going DOWN.",
				Optional:            true,
				Computed:            true,
			},
			"confirm_up_after": schema.Int64Attribute{
				MarkdownDescription: "Consecutive passed runs a synthetic monitor needs before going UP.",
				Optional:            true,
				Computed:            true,
			},
			"alert_channel_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of alert channels notified for this monitor.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
			"current_state": schema.StringAttribute{
				MarkdownDescription: "Current evaluator state. One of `pending`, `up`, `down`, `degraded`, `stale`.",
				Computed:            true,
			},
			"heartbeat_token": schema.StringAttribute{
				MarkdownDescription: "Server-generated heartbeat token. Only present for `heartbeat` monitors.",
				Computed:            true,
				Sensitive:           true,
			},
			"inserted_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 last-update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, diags := clientFromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if client != nil {
		r.client = client
	}
}

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := modelToInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.CreateMonitorWithResponse(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Create monitor: request failed", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Create monitor: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	state, diags := monitorToModel(ctx, &apiResp.JSON201.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read monitor: invalid ID in state", err.Error())
		return
	}

	apiResp, err := r.client.GetMonitorWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Read monitor: request failed", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Read monitor: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	newState, diags := monitorToModel(ctx, &apiResp.JSON200.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state monitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Update monitor: invalid ID in state", err.Error())
		return
	}

	input, diags := modelToInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.UpdateMonitorWithResponse(ctx, id, input)
	if err != nil {
		resp.Diagnostics.AddError("Update monitor: request failed", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Update monitor: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	newState, diags := monitorToModel(ctx, &apiResp.JSON200.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Delete monitor: invalid ID in state", err.Error())
		return
	}

	apiResp, err := r.client.DeleteMonitorWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Delete monitor: request failed", err.Error())
		return
	}
	switch apiResp.StatusCode() {
	case http.StatusNoContent, http.StatusNotFound:
		return
	default:
		resp.Diagnostics.AddError("Delete monitor: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
	}
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// monitorToModel maps an SDK Monitor into the Terraform state shape.
func monitorToModel(ctx context.Context, m *larmgo.Monitor) (monitorModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	configBytes, err := json.Marshal(m.Config)
	if err != nil {
		diags.AddError("Encoding monitor config", err.Error())
		return monitorModel{}, diags
	}

	model := monitorModel{
		ID:                 types.StringValue(m.Id.String()),
		Name:               types.StringValue(m.Name),
		CheckType:          types.StringValue(string(m.CheckType)),
		Enabled:            types.BoolValue(m.Enabled),
		IntervalSeconds:    types.Int64Value(int64(m.IntervalSeconds)),
		TimeoutMs:          types.Int64Value(int64(m.TimeoutMs)),
		Config:             jsontypes.NewNormalizedValue(string(configBytes)),
		InsertedAt:         formatTime(m.InsertedAt),
		UpdatedAt:          formatTime(m.UpdatedAt),
		ConfirmDownMinutes: optionalInt(m.ConfirmDownMinutes),
		ConfirmUpMinutes:   optionalInt(m.ConfirmUpMinutes),
		ConfirmDownAfter:   optionalInt(m.ConfirmDownAfter),
		ConfirmUpAfter:     optionalInt(m.ConfirmUpAfter),
		HeartbeatToken:     optionalString(m.HeartbeatToken),
	}

	if m.CurrentState != nil {
		model.CurrentState = types.StringValue(string(*m.CurrentState))
	} else {
		model.CurrentState = types.StringNull()
	}

	channelSet, setDiags := alertChannelIDsToSet(ctx, m.AlertChannelIds)
	diags.Append(setDiags...)
	model.AlertChannelIDs = channelSet

	return model, diags
}

// modelToInput builds an SDK MonitorInput from plan data.
func modelToInput(ctx context.Context, m monitorModel) (larmgo.MonitorInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	name := m.Name.ValueString()
	checkType := larmgo.CheckType(m.CheckType.ValueString())

	var configMap map[string]interface{}
	if !m.Config.IsNull() && !m.Config.IsUnknown() {
		if err := json.Unmarshal([]byte(m.Config.ValueString()), &configMap); err != nil {
			diags.AddError("Decoding config attribute", err.Error())
			return larmgo.MonitorInput{}, diags
		}
	}

	enabled := m.Enabled.ValueBool()
	input := larmgo.MonitorInput{
		Name:      &name,
		CheckType: &checkType,
		Config:    &configMap,
		Enabled:   &enabled,
	}

	if !m.IntervalSeconds.IsNull() && !m.IntervalSeconds.IsUnknown() {
		v := int(m.IntervalSeconds.ValueInt64())
		input.IntervalSeconds = &v
	}
	if !m.TimeoutMs.IsNull() && !m.TimeoutMs.IsUnknown() {
		v := int(m.TimeoutMs.ValueInt64())
		input.TimeoutMs = &v
	}
	if !m.ConfirmDownMinutes.IsNull() && !m.ConfirmDownMinutes.IsUnknown() {
		v := int(m.ConfirmDownMinutes.ValueInt64())
		input.ConfirmDownMinutes = &v
	}
	if !m.ConfirmUpMinutes.IsNull() && !m.ConfirmUpMinutes.IsUnknown() {
		v := int(m.ConfirmUpMinutes.ValueInt64())
		input.ConfirmUpMinutes = &v
	}
	if !m.ConfirmDownAfter.IsNull() && !m.ConfirmDownAfter.IsUnknown() {
		v := int(m.ConfirmDownAfter.ValueInt64())
		input.ConfirmDownAfter = &v
	}
	if !m.ConfirmUpAfter.IsNull() && !m.ConfirmUpAfter.IsUnknown() {
		v := int(m.ConfirmUpAfter.ValueInt64())
		input.ConfirmUpAfter = &v
	}

	if !m.AlertChannelIDs.IsNull() && !m.AlertChannelIDs.IsUnknown() {
		var strs []string
		setDiags := m.AlertChannelIDs.ElementsAs(ctx, &strs, false)
		diags.Append(setDiags...)
		if diags.HasError() {
			return larmgo.MonitorInput{}, diags
		}
		ids := make([]uuid.UUID, 0, len(strs))
		for _, s := range strs {
			id, err := uuid.Parse(s)
			if err != nil {
				diags.AddError("Parsing alert_channel_ids entry", err.Error())
				return larmgo.MonitorInput{}, diags
			}
			ids = append(ids, id)
		}
		input.AlertChannelIds = &ids
	}

	return input, diags
}

func alertChannelIDsToSet(_ context.Context, ids *[]uuid.UUID) (types.Set, diag.Diagnostics) {
	if ids == nil {
		return types.SetNull(types.StringType), nil
	}
	elems := make([]attr.Value, 0, len(*ids))
	for _, id := range *ids {
		elems = append(elems, types.StringValue(id.String()))
	}
	return types.SetValue(types.StringType, elems)
}
