package provider

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccStatusPageResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}

	slug := fmt.Sprintf("tf-acc-%d", time.Now().UnixNano())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with mixed top-level groups and components
			{
				Config: fmt.Sprintf(`
resource "larm_status_page" "tf_test" {
  name = "TF Acc Test"
  slug = %q

  components = [
    {
      type = "group"
      name = "Core Services"
      components = [
        { name = "API" },
        { name = "Database" }
      ]
    },
    {
      type = "component"
      name = "Email"
    },
    {
      type = "group"
      name = "Web"
      components = [
        { name = "Dashboard" }
      ]
    }
  ]
}
`, slug),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "name", "TF Acc Test"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "slug", slug),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.#", "3"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.type", "group"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.name", "Core Services"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.components.#", "2"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.components.0.name", "API"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.components.1.name", "Database"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.1.type", "component"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.1.name", "Email"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.2.type", "group"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.2.name", "Web"),
					resource.TestCheckResourceAttrSet("larm_status_page.tf_test", "id"),
					resource.TestCheckResourceAttrSet("larm_status_page.tf_test", "url"),
				),
			},
			// Reorder: move Email to position 0, change a group name
			{
				Config: fmt.Sprintf(`
resource "larm_status_page" "tf_test" {
  name = "TF Acc Test"
  slug = %q

  components = [
    {
      type = "component"
      name = "Email"
    },
    {
      type = "group"
      name = "Core"
      components = [
        { name = "API" }
      ]
    }
  ]
}
`, slug),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.#", "2"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.type", "component"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.0.name", "Email"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.1.type", "group"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.1.name", "Core"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "components.1.components.#", "1"),
				),
			},
			// Import — verify the whole structure round-trips
			{
				ResourceName:            "larm_status_page.tf_test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{},
			},
		},
	})
}

func TestAccStatusPageResource_emptyComponents(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped; set TF_ACC=1 to run")
	}

	slug := fmt.Sprintf("tf-acc-empty-%d", time.Now().UnixNano())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "larm_status_page" "tf_test" {
  name = "TF Empty"
  slug = %q
}
`, slug),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "name", "TF Empty"),
					resource.TestCheckResourceAttr("larm_status_page.tf_test", "slug", slug),
				),
			},
		},
	})
}
