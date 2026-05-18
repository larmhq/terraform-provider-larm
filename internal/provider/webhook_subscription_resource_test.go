package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWebhookSubscriptionResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_webhook_subscription" "tf_webhook" {
  url    = "https://example.com/larm-events"
  events = ["monitor.state_changed"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_webhook_subscription.tf_webhook", "url", "https://example.com/larm-events"),
					resource.TestCheckResourceAttr("larm_webhook_subscription.tf_webhook", "events.#", "1"),
					resource.TestCheckResourceAttr("larm_webhook_subscription.tf_webhook", "enabled", "true"),
					resource.TestCheckResourceAttrSet("larm_webhook_subscription.tf_webhook", "id"),
					resource.TestCheckResourceAttrSet("larm_webhook_subscription.tf_webhook", "secret"),
				),
			},
			{
				// Update events list (in-place) — secret must remain unchanged in state.
				Config: `
resource "larm_webhook_subscription" "tf_webhook" {
  url    = "https://example.com/larm-events"
  events = ["monitor.state_changed", "monitor.created", "monitor.deleted"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_webhook_subscription.tf_webhook", "events.#", "3"),
				),
			},
			{
				// Toggle enabled, in-place.
				Config: `
resource "larm_webhook_subscription" "tf_webhook" {
  url     = "https://example.com/larm-events"
  events  = ["monitor.state_changed", "monitor.created", "monitor.deleted"]
  enabled = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_webhook_subscription.tf_webhook", "enabled", "false"),
				),
			},
			{
				// Import: the secret is never returned by the API, so we can't verify it survives.
				ResourceName:      "larm_webhook_subscription.tf_webhook",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"secret",
				},
			},
		},
	})
}
