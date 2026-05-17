package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const alertChannelResourceMarkdown = "Manages a Larm alert channel — a destination that monitor outages and recoveries are dispatched to.\n\n" +
	"The provider supports 13 channel types (`webhook`, `slack`, `discord`, `email`, `ilert`, `incident_io`, `grafana_irm`, `mattermost`, `pagerduty`, `pushover`, `teams`, `ntfy`, `telegram`). " +
	"Each type has its own typed configuration block — exactly one block must be set, and it must match the `type` attribute.\n\n" +
	"**Notes:**\n\n" +
	"- **SMS channels are not supported by the provider.** Phone numbers require out-of-band confirmation; manage SMS channels via the Larm dashboard.\n" +
	"- **Channel configuration is never refreshed from the API.** Config fields hold secrets and are intentionally write-only on the wire. Terraform keeps the values you supplied in state; out-of-band edits to a channel's config in the dashboard will not be detected.\n" +
	"- **Changing `type` forces replacement.** Different channel types have different config shapes.\n" +
	"- **After `terraform import`, the typed config block must be re-declared in HCL.** The first apply after import sends the declared config to the API, which is a no-op if it matches what is already stored."

var (
	httpsURLRegex                = regexp.MustCompile(`^https://`)
	incidentIoURLRegex           = regexp.MustCompile(`^https://api\.incident\.io/`)
	discordWebhookURLRegex       = regexp.MustCompile(`^https://(discord\.com|discordapp\.com)/api/webhooks/\d+/[\w-]+$`)
	pagerdutyIntegrationKeyRegex = regexp.MustCompile(`^[0-9a-f]{32}$`)
	telegramBotTokenRegex        = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)
)

func httpsURLValidator() validator.String {
	return stringvalidator.RegexMatches(httpsURLRegex, "must start with https://")
}

func incidentIoURLValidator() validator.String {
	return stringvalidator.RegexMatches(incidentIoURLRegex, "must start with https://api.incident.io/")
}

func discordWebhookURLValidator() validator.String {
	return stringvalidator.RegexMatches(discordWebhookURLRegex, "must match https://discord.com/api/webhooks/{id}/{token}")
}

func pagerdutyIntegrationKeyValidator() validator.String {
	return stringvalidator.RegexMatches(pagerdutyIntegrationKeyRegex, "must be a 32-character lowercase hex string")
}

func telegramBotTokenValidator() validator.String {
	return stringvalidator.RegexMatches(telegramBotTokenRegex, "must match format 123456789:ABCdef…")
}

// typeBlockMatchValidator ensures that exactly the nested block matching `type` is set.
// resourcevalidator.ExactlyOneOf already enforces that exactly one block is set; this
// validator enforces that the one set is the one named by `type`.
type typeBlockMatchValidator struct{}

func (v *typeBlockMatchValidator) Description(_ context.Context) string {
	return "the typed config block must match the `type` attribute"
}

func (v *typeBlockMatchValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *typeBlockMatchValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var typeAttr types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &typeAttr)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if typeAttr.IsNull() || typeAttr.IsUnknown() {
		return
	}

	expected := typeAttr.ValueString()

	for _, t := range alertChannelTypes {
		var block types.Object
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(t), &block)...)
		if resp.Diagnostics.HasError() {
			return
		}
		set := !block.IsNull() && !block.IsUnknown()

		switch {
		case t == expected && !set:
			resp.Diagnostics.AddAttributeError(
				path.Root(t),
				fmt.Sprintf("Missing %q block", t),
				fmt.Sprintf("`type = %q` requires a `%s { ... }` block.", expected, t),
			)
		case t != expected && set:
			resp.Diagnostics.AddAttributeError(
				path.Root(t),
				fmt.Sprintf("Unexpected %q block", t),
				fmt.Sprintf("`type = %q` was set; the `%s { ... }` block must not be provided.", expected, t),
			)
		}
	}
}
