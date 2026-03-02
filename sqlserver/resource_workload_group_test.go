package sqlserver

import (
	"context"
	"fmt"
	"os"
	"terraform-provider-sqlserver/sql"
	"terraform-provider-sqlserver/sqlserver/model"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccWorkloadGroup_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckWorkloadGroupDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckWorkloadGroup(t, "basic", map[string]interface{}{
					"pool_name":                        "TestPoolWG",
					"group_name":                       "TestGroup",
					"importance":                       "MEDIUM",
					"request_max_memory_grant_percent": 25,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkloadGroupExists("sqlserver_workload_group.basic"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.basic", "name", "TestGroup"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.basic", "resource_pool_name", "TestPoolWG"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.basic", "importance", "MEDIUM"),
					resource.TestCheckResourceAttrSet("sqlserver_workload_group.basic", "group_id"),
				),
			},
		},
	})
}

func TestAccWorkloadGroup_Local_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckWorkloadGroupDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckWorkloadGroup(t, "update_test", map[string]interface{}{
					"pool_name":  "UpdatePoolWG",
					"group_name": "UpdateGroup",
					"importance": "LOW",
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkloadGroupExists("sqlserver_workload_group.update_test"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.update_test", "importance", "LOW"),
				),
			},
			{
				Config: testAccCheckWorkloadGroup(t, "update_test", map[string]interface{}{
					"pool_name":  "UpdatePoolWG",
					"group_name": "UpdateGroup",
					"importance": "HIGH",
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkloadGroupExists("sqlserver_workload_group.update_test"),
					resource.TestCheckResourceAttr("sqlserver_workload_group.update_test", "importance", "HIGH"),
				),
			},
		},
	})
}

func testAccCheckWorkloadGroup(t *testing.T, name string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
               login {}
             }

             resource "sqlserver_resource_pool" "{{ .name }}" {
               name = "{{ .pool_name }}"
             }

             resource "sqlserver_workload_group" "{{ .name }}" {
               name               = "{{ .group_name }}"
               resource_pool_name = sqlserver_resource_pool.{{ .name }}.name
               importance         = "{{ .importance }}"
               {{ with .request_max_memory_grant_percent }}request_max_memory_grant_percent = {{ . }}{{ end }}
               {{ with .request_max_cpu_time_sec }}request_max_cpu_time_sec = {{ . }}{{ end }}
               {{ with .max_dop }}max_dop = {{ . }}{{ end }}
               {{ with .group_max_requests }}group_max_requests = {{ . }}{{ end }}
             }`
	data["name"] = name
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckWorkloadGroupDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_workload_group" {
			continue
		}

		connector, err := getTestWorkloadGroupConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		name := rs.Primary.Attributes["name"]
		group, err := connector.GetWorkloadGroup(name)
		if group != nil {
			return fmt.Errorf("workload group still exists")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckWorkloadGroupExists(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_workload_group" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_workload_group", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestWorkloadGroupConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		name := rs.Primary.Attributes["name"]
		group, err := connector.GetWorkloadGroup(name)
		if group == nil {
			return fmt.Errorf("workload group does not exist")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}

		return nil
	}
}

type TestWorkloadGroupConnector interface {
	GetWorkloadGroup(name string) (*model.WorkloadGroup, error)
}

type testWorkloadGroupConnector struct {
	c interface{}
}

func getTestWorkloadGroupConnector(a map[string]string) (TestWorkloadGroupConnector, error) {
	host := os.Getenv("TF_SQLSERVER_HOST")
	port, ok := os.LookupEnv("TF_SQLSERVER_PORT")
	if !ok {
		port = DefaultPort
	}

	connector := &sql.Connector{
		Host:    host,
		Port:    port,
		Timeout: 60 * time.Second,
	}

	if username, ok := os.LookupEnv("TF_SQLSERVER_USERNAME"); ok {
		connector.Login = &sql.LoginUser{
			Username: username,
			Password: os.Getenv("TF_SQLSERVER_PASSWORD"),
		}
	}

	return testWorkloadGroupConnector{c: connector}, nil
}

func (t testWorkloadGroupConnector) GetWorkloadGroup(name string) (*model.WorkloadGroup, error) {
	return t.c.(WorkloadGroupConnector).GetWorkloadGroup(context.Background(), name)
}
