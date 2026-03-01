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
					resource.TestCheckResourceAttr("sqlserver_resource_pool.pool", "name", "my_pool"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.pool", "min_memory_percent", "10"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.pool", "max_memory_percent", "90"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.pool", "min_cpu_percent", "5"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.pool", "max_cpu_percent", "80"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.pool", "cap_cpu_percent", "80"),

					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "name", "my_group"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "pool_name", "my_pool"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "importance", "High"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "request_max_memory_grant_percent", "30"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "request_max_cpu_time_sec", "10"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "request_memory_grant_timeout_sec", "5"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.wg", "max_dop", "2"),
				),
			},
		},
	})
}

const testAccResourceResourceGovernorConfig_full = `
resource "sqlserver_resource_pool" "pool" {
  name                = "my_pool"
  min_memory_percent  = 10
  max_memory_percent  = 90
  min_cpu_percent     = 5
  max_cpu_percent     = 80
  cap_cpu_percent     = 80
}

resource "sqlserver_workload_group" "wg" {
  name      = "my_group"
  pool_name = sqlserver_resource_pool.pool.name
  importance = "High"
  request_max_memory_grant_percent = 30
  request_max_cpu_time_sec = 10
  request_memory_grant_timeout_sec = 5
  max_dop = 2
}

resource "sqlserver_classifier_function" "cf" {
  name       = "my_classifier"
  definition = "RETURN 'default';"
}

resource "sqlserver_resource_governor" "test" {
  enabled = false
  classifier_function = sqlserver_classifier_function.cf.name
}
