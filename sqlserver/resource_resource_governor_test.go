package sqlserver

import (
	"testing"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceResourceGovernor_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceResourceGovernorConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_resource_governor.test", "enabled", "true"),
				),
			},
		},
	})
}

const testAccResourceResourceGovernorConfig_basic = `
resource "sqlserver_resource_governor" "test" {
  enabled = true
}
`

func TestAccResourceResourceGovernor_full(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceResourceGovernorConfig_full,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_resource_governor.test", "enabled", "false"),
					resource.TestCheckResourceAttr("sqlserver_resource_governor.test", "classifier_function", "my_classifier"),
				),
			},
		},
	})
}

const testAccResourceResourceGovernorConfig_full = `
resource "sqlserver_classifier_function" "cf" {
  name       = "my_classifier"
  definition = "RETURN 'default';"
}

resource "sqlserver_resource_governor" "test" {
  enabled = false
  classifier_function = sqlserver_classifier_function.cf.name
}
