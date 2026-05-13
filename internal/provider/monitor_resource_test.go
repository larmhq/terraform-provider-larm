package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("LARM_API_KEY") == "" {
		t.Fatal("LARM_API_KEY must be set for acceptance tests")
	}
}

func TestAccMonitorResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_monitor" "tf_test" {
  name       = "TF Acc Test"
  check_type = "http"
  config = jsonencode({
    url                   = "https://example.com"
    method                = "GET"
    expected_status_codes = [200]
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_monitor.tf_test", "name", "TF Acc Test"),
					resource.TestCheckResourceAttr("larm_monitor.tf_test", "check_type", "http"),
					resource.TestCheckResourceAttr("larm_monitor.tf_test", "enabled", "true"),
					resource.TestCheckResourceAttr("larm_monitor.tf_test", "interval_seconds", "180"),
					resource.TestCheckResourceAttrSet("larm_monitor.tf_test", "id"),
				),
			},
			{
				Config: `
resource "larm_monitor" "tf_test" {
  name             = "TF Acc Test (renamed)"
  check_type       = "http"
  interval_seconds = 300
  config = jsonencode({
    url                   = "https://example.com"
    method                = "GET"
    expected_status_codes = [200]
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_monitor.tf_test", "name", "TF Acc Test (renamed)"),
					resource.TestCheckResourceAttr("larm_monitor.tf_test", "interval_seconds", "300"),
				),
			},
			{
				ResourceName:            "larm_monitor.tf_test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{
					// The API normalizes some fields; ignore them on import-verify if needed.
				},
			},
		},
	})
}

func TestAccMonitorResource_heartbeat(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "larm_monitor" "tf_heartbeat" {
  name       = "TF Acc Heartbeat"
  check_type = "heartbeat"
  config = jsonencode({
    expected_interval  = 300
    consecutive_misses = 2
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("larm_monitor.tf_heartbeat", "heartbeat_token"),
					resource.TestCheckResourceAttr("larm_monitor.tf_heartbeat", "check_type", "heartbeat"),
				),
			},
		},
	})
}
