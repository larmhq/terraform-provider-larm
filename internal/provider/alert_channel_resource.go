package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                     = &alertChannelResource{}
	_ resource.ResourceWithConfigure        = &alertChannelResource{}
	_ resource.ResourceWithConfigValidators = &alertChannelResource{}
	_ resource.ResourceWithImportState      = &alertChannelResource{}
)

// alertChannelTypes is the ordered list of channel types supported by the provider.
// SMS is intentionally omitted — it requires out-of-band phone confirmation.
var alertChannelTypes = []string{
	"webhook",
	"slack",
	"discord",
	"email",
	"ilert",
	"incident_io",
	"grafana_irm",
	"mattermost",
	"pagerduty",
	"pushover",
	"teams",
	"ntfy",
	"telegram",
}

// NewAlertChannelResource is the constructor referenced by the provider.
func NewAlertChannelResource() resource.Resource {
	return &alertChannelResource{}
}

type alertChannelResource struct {
	client *larmgo.ClientWithResponses
}

type alertChannelModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Type                  types.String `tfsdk:"type"`
	Enabled               types.Bool   `tfsdk:"enabled"`
	DefaultForNewMonitors types.Bool   `tfsdk:"default_for_new_monitors"`
	BrokenAt              types.String `tfsdk:"broken_at"`
	BrokenReason          types.String `tfsdk:"broken_reason"`
	InsertedAt            types.String `tfsdk:"inserted_at"`
	UpdatedAt             types.String `tfsdk:"updated_at"`

	Webhook    *webhookConfigModel    `tfsdk:"webhook"`
	Slack      *slackConfigModel      `tfsdk:"slack"`
	Discord    *discordConfigModel    `tfsdk:"discord"`
	Email      *emailConfigModel      `tfsdk:"email"`
	Ilert      *ilertConfigModel      `tfsdk:"ilert"`
	IncidentIo *incidentIoConfigModel `tfsdk:"incident_io"`
	GrafanaIrm *grafanaIrmConfigModel `tfsdk:"grafana_irm"`
	Mattermost *mattermostConfigModel `tfsdk:"mattermost"`
	Pagerduty  *pagerdutyConfigModel  `tfsdk:"pagerduty"`
	Pushover   *pushoverConfigModel   `tfsdk:"pushover"`
	Teams      *teamsConfigModel      `tfsdk:"teams"`
	Ntfy       *ntfyConfigModel       `tfsdk:"ntfy"`
	Telegram   *telegramConfigModel   `tfsdk:"telegram"`
}

type webhookConfigModel struct {
	URL             types.String `tfsdk:"url"`
	Headers         types.Map    `tfsdk:"headers"`
	PayloadTemplate types.String `tfsdk:"payload_template"`
}

type slackConfigModel struct {
	IntegrationID types.String `tfsdk:"integration_id"`
	ChannelID     types.String `tfsdk:"channel_id"`
	ChannelName   types.String `tfsdk:"channel_name"`
}

type discordConfigModel struct {
	WebhookURL types.String `tfsdk:"webhook_url"`
}

type emailConfigModel struct {
	Recipients types.List `tfsdk:"recipients"`
}

type ilertConfigModel struct {
	APIKey types.String `tfsdk:"api_key"`
}

type incidentIoConfigModel struct {
	AlertSourceURL types.String `tfsdk:"alert_source_url"`
	APIToken       types.String `tfsdk:"api_token"`
}

type grafanaIrmConfigModel struct {
	IntegrationURL types.String `tfsdk:"integration_url"`
}

type mattermostConfigModel struct {
	WebhookURL types.String `tfsdk:"webhook_url"`
}

type pagerdutyConfigModel struct {
	IntegrationKey types.String `tfsdk:"integration_key"`
}

type pushoverConfigModel struct {
	UserKey  types.String `tfsdk:"user_key"`
	APIToken types.String `tfsdk:"api_token"`
}

type teamsConfigModel struct {
	WebhookURL types.String `tfsdk:"webhook_url"`
}

type ntfyConfigModel struct {
	ServerURL types.String `tfsdk:"server_url"`
	Topic     types.String `tfsdk:"topic"`
}

type telegramConfigModel struct {
	BotToken types.String `tfsdk:"bot_token"`
	ChatID   types.String `tfsdk:"chat_id"`
}

func (r *alertChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_channel"
}

func (r *alertChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: alertChannelResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Alert channel ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable channel name.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Channel type. One of `" + strings.Join(alertChannelTypes, "`, `") + "`. Changing this forces replacement.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(alertChannelTypes...),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the channel is enabled. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"default_for_new_monitors": schema.BoolAttribute{
				MarkdownDescription: "Whether new monitors should attach this channel by default. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"broken_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the channel was last marked broken; null otherwise.",
				Computed:            true,
			},
			"broken_reason": schema.StringAttribute{
				MarkdownDescription: "Reason the channel is broken; null otherwise.",
				Computed:            true,
			},
			"inserted_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 last-update timestamp.",
				Computed:            true,
			},
			"webhook": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `webhook` channels. Set when `type = \"webhook\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "HTTPS endpoint to POST alerts to.",
						Required:            true,
						Validators:          []validator.String{httpsURLValidator()},
					},
					"headers": schema.MapAttribute{
						MarkdownDescription: "Additional HTTP headers to send with each request. Header values may contain bearer tokens, so the entire map is treated as sensitive.",
						Optional:            true,
						Sensitive:           true,
						ElementType:         types.StringType,
					},
					"payload_template": schema.StringAttribute{
						MarkdownDescription: "Optional EEx template overriding the default alert payload.",
						Optional:            true,
					},
				},
			},
			"slack": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `slack` channels. Set when `type = \"slack\"`. The Slack workspace must first be connected in the Larm dashboard; pass the resulting integration ID here.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"integration_id": schema.StringAttribute{
						MarkdownDescription: "Identifier of the Slack OAuth integration in Larm.",
						Required:            true,
					},
					"channel_id": schema.StringAttribute{
						MarkdownDescription: "Slack channel ID (e.g. `C0123456789`).",
						Required:            true,
					},
					"channel_name": schema.StringAttribute{
						MarkdownDescription: "Display name of the Slack channel (e.g. `#alerts`).",
						Required:            true,
					},
				},
			},
			"discord": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `discord` channels. Set when `type = \"discord\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "Discord webhook URL, in the form `https://discord.com/api/webhooks/{id}/{token}`.",
						Required:            true,
						Sensitive:           true,
						Validators:          []validator.String{discordWebhookURLValidator()},
					},
				},
			},
			"email": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `email` channels. Set when `type = \"email\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"recipients": schema.ListAttribute{
						MarkdownDescription: "Email addresses to notify.",
						Required:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"ilert": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `ilert` channels. Set when `type = \"ilert\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key": schema.StringAttribute{
						MarkdownDescription: "ilert alert source API key.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"incident_io": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `incident_io` channels. Set when `type = \"incident_io\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"alert_source_url": schema.StringAttribute{
						MarkdownDescription: "Alert source URL from incident.io. Must start with `https://api.incident.io/`.",
						Required:            true,
						Validators:          []validator.String{incidentIoURLValidator()},
					},
					"api_token": schema.StringAttribute{
						MarkdownDescription: "incident.io API token.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"grafana_irm": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `grafana_irm` channels. Set when `type = \"grafana_irm\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"integration_url": schema.StringAttribute{
						MarkdownDescription: "Grafana IRM integration webhook URL.",
						Required:            true,
						Sensitive:           true,
						Validators:          []validator.String{httpsURLValidator()},
					},
				},
			},
			"mattermost": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `mattermost` channels. Set when `type = \"mattermost\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "Mattermost incoming-webhook URL.",
						Required:            true,
						Sensitive:           true,
						Validators:          []validator.String{httpsURLValidator()},
					},
				},
			},
			"pagerduty": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `pagerduty` channels. Set when `type = \"pagerduty\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"integration_key": schema.StringAttribute{
						MarkdownDescription: "PagerDuty Events API v2 integration key (32-character lowercase hex).",
						Required:            true,
						Sensitive:           true,
						Validators:          []validator.String{pagerdutyIntegrationKeyValidator()},
					},
				},
			},
			"pushover": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `pushover` channels. Set when `type = \"pushover\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"user_key": schema.StringAttribute{
						MarkdownDescription: "Pushover user (or group) key.",
						Required:            true,
						Sensitive:           true,
					},
					"api_token": schema.StringAttribute{
						MarkdownDescription: "Pushover application API token.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"teams": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `teams` channels (Microsoft Teams). Set when `type = \"teams\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "Microsoft Teams incoming-webhook URL.",
						Required:            true,
						Sensitive:           true,
						Validators:          []validator.String{httpsURLValidator()},
					},
				},
			},
			"ntfy": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `ntfy` channels. Set when `type = \"ntfy\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"server_url": schema.StringAttribute{
						MarkdownDescription: "ntfy server URL (e.g. `https://ntfy.sh`).",
						Required:            true,
						Validators:          []validator.String{httpsURLValidator()},
					},
					"topic": schema.StringAttribute{
						MarkdownDescription: "ntfy topic name.",
						Required:            true,
					},
				},
			},
			"telegram": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for `telegram` channels. Set when `type = \"telegram\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"bot_token": schema.StringAttribute{
						MarkdownDescription: "Telegram bot token, in the form `123456789:ABCdef…`.",
						Required:            true,
						Sensitive:           true,
						Validators:          []validator.String{telegramBotTokenValidator()},
					},
					"chat_id": schema.StringAttribute{
						MarkdownDescription: "Numeric chat ID (e.g. `-1001234567890`) or channel username (e.g. `@alerts`).",
						Required:            true,
					},
				},
			},
		},
	}
}

func (r *alertChannelResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	exprs := make([]path.Expression, 0, len(alertChannelTypes))
	for _, t := range alertChannelTypes {
		exprs = append(exprs, path.MatchRoot(t))
	}
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(exprs...),
		&typeBlockMatchValidator{},
	}
}

func (r *alertChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, diags := clientFromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if client != nil {
		r.client = client
	}
}

func (r *alertChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := alertChannelModelToInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.CreateAlertChannelWithResponse(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Create alert channel: request failed", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Create alert channel: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	state := alertChannelAPIToModel(&apiResp.JSON201.Data, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read alert channel: invalid ID in state", err.Error())
		return
	}

	apiResp, err := r.client.GetAlertChannelWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Read alert channel: request failed", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Read alert channel: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	newState := alertChannelAPIToModel(&apiResp.JSON200.Data, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *alertChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Update alert channel: invalid ID in state", err.Error())
		return
	}

	input, diags := alertChannelModelToInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.UpdateAlertChannelWithResponse(ctx, id, input)
	if err != nil {
		resp.Diagnostics.AddError("Update alert channel: request failed", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Update alert channel: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
		return
	}

	newState := alertChannelAPIToModel(&apiResp.JSON200.Data, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *alertChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Delete alert channel: invalid ID in state", err.Error())
		return
	}

	apiResp, err := r.client.DeleteAlertChannelWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Delete alert channel: request failed", err.Error())
		return
	}
	switch apiResp.StatusCode() {
	case http.StatusNoContent, http.StatusNotFound:
		return
	default:
		resp.Diagnostics.AddError("Delete alert channel: unexpected response", responseError(apiResp.StatusCode(), apiResp.Body))
	}
}

func (r *alertChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// alertChannelAPIToModel maps the API response into the Terraform state shape, carrying
// over the typed config block from `prev` because the API never returns config (it would
// leak secrets). On Create/Update, prev = plan; on Read, prev = current state.
func alertChannelAPIToModel(api *larmgo.AlertChannel, prev alertChannelModel) alertChannelModel {
	model := alertChannelModel{
		ID:                    types.StringValue(api.Id.String()),
		Name:                  types.StringValue(api.Name),
		Type:                  types.StringValue(string(api.Type)),
		Enabled:               types.BoolValue(api.Enabled),
		DefaultForNewMonitors: types.BoolValue(api.DefaultForNewMonitors),
		BrokenAt:              optionalTime(api.BrokenAt),
		BrokenReason:          optionalString(api.BrokenReason),
		InsertedAt:            formatTime(api.InsertedAt),
		UpdatedAt:             formatTime(api.UpdatedAt),

		Webhook:    prev.Webhook,
		Slack:      prev.Slack,
		Discord:    prev.Discord,
		Email:      prev.Email,
		Ilert:      prev.Ilert,
		IncidentIo: prev.IncidentIo,
		GrafanaIrm: prev.GrafanaIrm,
		Mattermost: prev.Mattermost,
		Pagerduty:  prev.Pagerduty,
		Pushover:   prev.Pushover,
		Teams:      prev.Teams,
		Ntfy:       prev.Ntfy,
		Telegram:   prev.Telegram,
	}
	return model
}

// alertChannelModelToInput builds an SDK AlertChannelInput from plan data.
func alertChannelModelToInput(ctx context.Context, m alertChannelModel) (larmgo.AlertChannelInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	name := m.Name.ValueString()
	channelType := larmgo.AlertChannelType(m.Type.ValueString())
	enabled := m.Enabled.ValueBool()
	defaultForNewMonitors := m.DefaultForNewMonitors.ValueBool()

	configMap, configDiags := alertChannelConfigMap(ctx, m)
	diags.Append(configDiags...)
	if diags.HasError() {
		return larmgo.AlertChannelInput{}, diags
	}

	return larmgo.AlertChannelInput{
		Name:                  &name,
		Type:                  &channelType,
		Enabled:               &enabled,
		DefaultForNewMonitors: &defaultForNewMonitors,
		Config:                &configMap,
	}, diags
}

// alertChannelConfigMap returns the JSON-serializable config payload corresponding to the
// channel type set on the model. Returns a diag if the matching nested block is null
// (which means typeBlockMatchValidator failed to catch a programmer error).
func alertChannelConfigMap(ctx context.Context, m alertChannelModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch m.Type.ValueString() {
	case "webhook":
		if m.Webhook == nil {
			return nil, missingBlockDiag("webhook")
		}
		cfg := map[string]interface{}{"url": m.Webhook.URL.ValueString()}
		if !m.Webhook.Headers.IsNull() && !m.Webhook.Headers.IsUnknown() {
			var headers map[string]string
			diags.Append(m.Webhook.Headers.ElementsAs(ctx, &headers, false)...)
			if diags.HasError() {
				return nil, diags
			}
			cfg["headers"] = headers
		}
		if !m.Webhook.PayloadTemplate.IsNull() && !m.Webhook.PayloadTemplate.IsUnknown() {
			cfg["payload_template"] = m.Webhook.PayloadTemplate.ValueString()
		}
		return cfg, diags

	case "slack":
		if m.Slack == nil {
			return nil, missingBlockDiag("slack")
		}
		return map[string]interface{}{
			"integration_id": m.Slack.IntegrationID.ValueString(),
			"channel_id":     m.Slack.ChannelID.ValueString(),
			"channel_name":   m.Slack.ChannelName.ValueString(),
		}, diags

	case "discord":
		if m.Discord == nil {
			return nil, missingBlockDiag("discord")
		}
		return map[string]interface{}{
			"webhook_url": m.Discord.WebhookURL.ValueString(),
		}, diags

	case "email":
		if m.Email == nil {
			return nil, missingBlockDiag("email")
		}
		var recipients []string
		diags.Append(m.Email.Recipients.ElementsAs(ctx, &recipients, false)...)
		if diags.HasError() {
			return nil, diags
		}
		return map[string]interface{}{"recipients": recipients}, diags

	case "ilert":
		if m.Ilert == nil {
			return nil, missingBlockDiag("ilert")
		}
		return map[string]interface{}{"api_key": m.Ilert.APIKey.ValueString()}, diags

	case "incident_io":
		if m.IncidentIo == nil {
			return nil, missingBlockDiag("incident_io")
		}
		return map[string]interface{}{
			"alert_source_url": m.IncidentIo.AlertSourceURL.ValueString(),
			"api_token":        m.IncidentIo.APIToken.ValueString(),
		}, diags

	case "grafana_irm":
		if m.GrafanaIrm == nil {
			return nil, missingBlockDiag("grafana_irm")
		}
		return map[string]interface{}{
			"integration_url": m.GrafanaIrm.IntegrationURL.ValueString(),
		}, diags

	case "mattermost":
		if m.Mattermost == nil {
			return nil, missingBlockDiag("mattermost")
		}
		return map[string]interface{}{
			"webhook_url": m.Mattermost.WebhookURL.ValueString(),
		}, diags

	case "pagerduty":
		if m.Pagerduty == nil {
			return nil, missingBlockDiag("pagerduty")
		}
		return map[string]interface{}{
			"integration_key": m.Pagerduty.IntegrationKey.ValueString(),
		}, diags

	case "pushover":
		if m.Pushover == nil {
			return nil, missingBlockDiag("pushover")
		}
		return map[string]interface{}{
			"user_key":  m.Pushover.UserKey.ValueString(),
			"api_token": m.Pushover.APIToken.ValueString(),
		}, diags

	case "teams":
		if m.Teams == nil {
			return nil, missingBlockDiag("teams")
		}
		return map[string]interface{}{
			"webhook_url": m.Teams.WebhookURL.ValueString(),
		}, diags

	case "ntfy":
		if m.Ntfy == nil {
			return nil, missingBlockDiag("ntfy")
		}
		return map[string]interface{}{
			"server_url": m.Ntfy.ServerURL.ValueString(),
			"topic":      m.Ntfy.Topic.ValueString(),
		}, diags

	case "telegram":
		if m.Telegram == nil {
			return nil, missingBlockDiag("telegram")
		}
		return map[string]interface{}{
			"bot_token": m.Telegram.BotToken.ValueString(),
			"chat_id":   m.Telegram.ChatID.ValueString(),
		}, diags
	}

	diags.AddAttributeError(
		path.Root("type"),
		"Unsupported alert channel type",
		fmt.Sprintf("Type %q is not supported by the provider. Supported types: %v.", m.Type.ValueString(), alertChannelTypes),
	)
	return nil, diags
}

func missingBlockDiag(channelType string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddAttributeError(
		path.Root(channelType),
		fmt.Sprintf("Missing %q block", channelType),
		fmt.Sprintf("`type = %q` was set but no `%s { ... }` block was provided.", channelType, channelType),
	)
	return diags
}
