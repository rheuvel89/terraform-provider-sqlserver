package sqlserver

import (
	"testing"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceWorkloadGroup_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceWorkloadGroupConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_workload_group.test", "name", "testgroup"),
				),
			},
		},
	})
}

const testAccResourceWorkloadGroupConfig_basic = `
resource "sqlserver_resource_pool" "test" {
  name = "testpool"
}

resource "sqlserver_workload_group" "test" {
  name = "testgroup"
  pool_name = sqlserver_resource_pool.test.name
}
`
