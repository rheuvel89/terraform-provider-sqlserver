package sqlserver

import (
	"testing"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceResourcePool_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceResourcePoolConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_resource_pool.test", "name", "testpool"),
				),
			},
		},
	})
}

const testAccResourceResourcePoolConfig_basic = `
resource "sqlserver_resource_pool" "test" {
  name = "testpool"
  min_memory_percent = 0
  max_memory_percent = 100
  min_cpu_percent = 0
  max_cpu_percent = 100
  cap_cpu_percent = 100
}
`
