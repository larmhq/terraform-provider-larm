package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAlertChannelResource_webhook(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_alert_channel" "tf_webhook" {
  name = "TF Acc Webhook"
  type = "webhook"

  webhook = {
    url = "https://ops.example.com/larm"

    headers = {
      Authorization = "Bearer xyz"
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "name", "TF Acc Webhook"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "type", "webhook"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "enabled", "true"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "webhook.url", "https://ops.example.com/larm"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "webhook.headers.Authorization", "Bearer xyz"),
					resource.TestCheckResourceAttrSet("larm_alert_channel.tf_webhook", "id"),
				),
			},
			{
				Config: `
resource "larm_alert_channel" "tf_webhook" {
  name    = "TF Acc Webhook (renamed)"
  type    = "webhook"
  enabled = false

  webhook = {
    url = "https://ops.example.com/larm-v2"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "name", "TF Acc Webhook (renamed)"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "enabled", "false"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_webhook", "webhook.url", "https://ops.example.com/larm-v2"),
					resource.TestCheckNoResourceAttr("larm_alert_channel.tf_webhook", "webhook.headers"),
				),
			},
			{
				ResourceName:      "larm_alert_channel.tf_webhook",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// Config fields aren't returned by the API; state preserves user input which import can't repopulate.
					"webhook",
				},
			},
		},
	})
}

func TestAccAlertChannelResource_slack(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	integrationID := os.Getenv("LARM_TEST_SLACK_INTEGRATION_ID")
	if integrationID == "" {
		t.Skip("LARM_TEST_SLACK_INTEGRATION_ID must be set for the Slack acceptance test")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_alert_channel" "tf_slack" {
  name = "TF Acc Slack"
  type = "slack"

  slack = {
    integration_id = "` + integrationID + `"
    channel_id     = "C0123456789"
    channel_name   = "#tf-acc"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_alert_channel.tf_slack", "type", "slack"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_slack", "slack.channel_name", "#tf-acc"),
				),
			},
		},
	})
}

func TestAccAlertChannelResource_pagerduty(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_alert_channel" "tf_pagerduty" {
  name = "TF Acc PagerDuty"
  type = "pagerduty"

  pagerduty = {
    integration_key = "0123456789abcdef0123456789abcdef"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_alert_channel.tf_pagerduty", "type", "pagerduty"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_pagerduty", "pagerduty.integration_key", "0123456789abcdef0123456789abcdef"),
				),
			},
		},
	})
}

func TestAccAlertChannelResource_email(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_alert_channel" "tf_email" {
  name = "TF Acc Email"
  type = "email"

  email = {
    recipients = ["oncall@example.com", "sre@example.com"]
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_alert_channel.tf_email", "type", "email"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_email", "email.recipients.#", "2"),
					resource.TestCheckResourceAttr("larm_alert_channel.tf_email", "email.recipients.0", "oncall@example.com"),
				),
			},
		},
	})
}
