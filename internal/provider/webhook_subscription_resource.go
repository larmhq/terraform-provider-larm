package provider

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	larmgo "github.com/larmhq/larm-go/client"
)

var (
	_ resource.Resource                = &webhookSubscriptionResource{}
	_ resource.ResourceWithConfigure   = &webhookSubscriptionResource{}
	_ resource.ResourceWithImportState = &webhookSubscriptionResource{}
)

// webhookEvents enumerates the four event names the backend recognises
// (see Larm.Webhooks.WebhookSubscription @valid_events).
var webhookEvents = []string{
	"monitor.state_changed",
	"monitor.created",
	"monitor.updated",
	"monitor.deleted",
}

var webhookURLHTTPSRegex = regexp.MustCompile(`^https://`)

const webhookSubscriptionResourceMarkdown = "Manages a Larm webhook subscription — an HTTPS endpoint that receives signed payloads when monitors change state.\n\n" +
	"**Notes:**\n\n" +
	"- **The signing secret is returned exactly once.** Terraform captures it on create and stores it in state. The Larm API does not expose secrets on read; if state loses the value, recreate the subscription to obtain a new one.\n" +
	"- **After `terraform import`, `secret` will be null.** Recreate the subscription if you need access to the signing secret.\n" +
	"- **Free-plan organizations cannot create subscriptions.** The API returns 403; upgrade the organization or use a different one."

// NewWebhookSubscriptionResource is the constructor referenced by the provider.
func NewWebhookSubscriptionResource() resource.Resource {
	return &webhookSubscriptionResource{}
}

type webhookSubscriptionResource struct {
	client *larmgo.ClientWithResponses
}

type webhookSubscriptionModel struct {
	ID         types.String `tfsdk:"id"`
	URL        types.String `tfsdk:"url"`
	Events     types.Set    `tfsdk:"events"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Secret     types.String `tfsdk:"secret"`
	InsertedAt types.String `tfsdk:"inserted_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func (r *webhookSubscriptionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_subscription"
}

func (r *webhookSubscriptionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: webhookSubscriptionResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Webhook subscription ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "HTTPS endpoint that receives event payloads.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(webhookURLHTTPSRegex, "must start with https://"),
				},
			},
			"events": schema.SetAttribute{
				MarkdownDescription: "Events the endpoint subscribes to. Allowed values: `" + strings.Join(webhookEvents, "`, `") + "`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(webhookEvents...)),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the subscription is enabled. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "HMAC-SHA256 signing secret. The API returns this only on create — preserved verbatim in state across reads and updates, and never re-fetchable.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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

func (r *webhookSubscriptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, diags := clientFromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if client != nil {
		r.client = client
	}
}

func (r *webhookSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookSubscriptionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := webhookModelToInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.CreateWebhookWithResponse(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Create webhook subscription: request failed", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Create webhook subscription: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	state := webhookCreateAPIToModel(&apiResp.JSON201.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookSubscriptionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read webhook subscription: invalid ID in state", err.Error())
		return
	}

	apiResp, err := r.client.GetWebhookWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Read webhook subscription: request failed", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Read webhook subscription: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	newState := webhookAPIToModel(&apiResp.JSON200.Data, state.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *webhookSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state webhookSubscriptionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Update webhook subscription: invalid ID in state", err.Error())
		return
	}

	input, diags := webhookModelToInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.UpdateWebhookWithResponse(ctx, id, input)
	if err != nil {
		resp.Diagnostics.AddError("Update webhook subscription: request failed", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Update webhook subscription: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	newState := webhookAPIToModel(&apiResp.JSON200.Data, state.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *webhookSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookSubscriptionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Delete webhook subscription: invalid ID in state", err.Error())
		return
	}

	apiResp, err := r.client.DeleteWebhookWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Delete webhook subscription: request failed", err.Error())
		return
	}
	switch apiResp.StatusCode() {
	case http.StatusNoContent, http.StatusNotFound:
		return
	default:
		resp.Diagnostics.AddError("Delete webhook subscription: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
	}
}

func (r *webhookSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// webhookCreateAPIToModel maps the create-response shape (carries `secret`)
// into the Terraform state. Only path that populates Secret.
func webhookCreateAPIToModel(api *larmgo.WebhookSubscriptionWithSecret) webhookSubscriptionModel {
	return webhookSubscriptionModel{
		ID:         types.StringValue(api.Id.String()),
		URL:        types.StringValue(api.Url),
		Events:     webhookEventsToSet(api.Events),
		Enabled:    types.BoolValue(api.Enabled),
		Secret:     types.StringValue(api.Secret),
		InsertedAt: formatTime(api.InsertedAt),
		UpdatedAt:  formatTime(api.UpdatedAt),
	}
}

// webhookAPIToModel maps the get/update response (no secret) into state,
// preserving the prior Secret value verbatim because the API never returns it
// again and the server never rotates it.
func webhookAPIToModel(api *larmgo.WebhookSubscription, priorSecret types.String) webhookSubscriptionModel {
	return webhookSubscriptionModel{
		ID:         types.StringValue(api.Id.String()),
		URL:        types.StringValue(api.Url),
		Events:     webhookEventsToSet(api.Events),
		Enabled:    types.BoolValue(api.Enabled),
		Secret:     priorSecret,
		InsertedAt: formatTime(api.InsertedAt),
		UpdatedAt:  formatTime(api.UpdatedAt),
	}
}

func webhookEventsToSet(events []larmgo.WebhookEvent) types.Set {
	elems := make([]attr.Value, 0, len(events))
	for _, e := range events {
		elems = append(elems, types.StringValue(string(e)))
	}
	set, _ := types.SetValue(types.StringType, elems)
	return set
}

func webhookModelToInput(ctx context.Context, m webhookSubscriptionModel) (larmgo.WebhookSubscriptionInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	url := m.URL.ValueString()
	enabled := m.Enabled.ValueBool()

	var eventStrs []string
	diags.Append(m.Events.ElementsAs(ctx, &eventStrs, false)...)
	if diags.HasError() {
		return larmgo.WebhookSubscriptionInput{}, diags
	}
	events := make([]larmgo.WebhookEvent, 0, len(eventStrs))
	for _, e := range eventStrs {
		events = append(events, larmgo.WebhookEvent(e))
	}

	return larmgo.WebhookSubscriptionInput{
		Url:     &url,
		Events:  &events,
		Enabled: &enabled,
	}, diags
}
