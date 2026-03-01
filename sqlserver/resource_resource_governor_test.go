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
