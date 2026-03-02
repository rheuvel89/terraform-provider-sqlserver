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

func TestAccResourcePool_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckResourcePoolDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourcePool(t, "basic", map[string]interface{}{
					"name":               "TestPool",
					"min_cpu_percent":    10,
					"max_cpu_percent":    50,
					"min_memory_percent": 10,
					"max_memory_percent": 50,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcePoolExists("sqlserver_resource_pool.basic"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.basic", "name", "TestPool"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.basic", "min_cpu_percent", "10"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.basic", "max_cpu_percent", "50"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.basic", "min_memory_percent", "10"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.basic", "max_memory_percent", "50"),
					resource.TestCheckResourceAttrSet("sqlserver_resource_pool.basic", "pool_id"),
				),
			},
		},
	})
}

func TestAccResourcePool_Local_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckResourcePoolDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourcePool(t, "update_test", map[string]interface{}{
					"name":            "UpdatePool",
					"min_cpu_percent": 10,
					"max_cpu_percent": 50,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcePoolExists("sqlserver_resource_pool.update_test"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.update_test", "min_cpu_percent", "10"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.update_test", "max_cpu_percent", "50"),
				),
			},
			{
				Config: testAccCheckResourcePool(t, "update_test", map[string]interface{}{
					"name":            "UpdatePool",
					"min_cpu_percent": 20,
					"max_cpu_percent": 80,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcePoolExists("sqlserver_resource_pool.update_test"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.update_test", "min_cpu_percent", "20"),
					resource.TestCheckResourceAttr("sqlserver_resource_pool.update_test", "max_cpu_percent", "80"),
				),
			},
		},
	})
}

func testAccCheckResourcePool(t *testing.T, name string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
               login {}
             }

             resource "sqlserver_resource_pool" "{{ .resource_name }}" {
               name               = "{{ .name }}"
               min_cpu_percent    = {{ .min_cpu_percent }}
               max_cpu_percent    = {{ .max_cpu_percent }}
               {{ with .min_memory_percent }}min_memory_percent = {{ . }}{{ end }}
               {{ with .max_memory_percent }}max_memory_percent = {{ . }}{{ end }}
               {{ with .cap_cpu_percent }}cap_cpu_percent = {{ . }}{{ end }}
             }`
	data["resource_name"] = name
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckResourcePoolDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_resource_pool" {
			continue
		}

		connector, err := getTestResourcePoolConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		name := rs.Primary.Attributes["name"]
		pool, err := connector.GetResourcePool(name)
		if pool != nil {
			return fmt.Errorf("resource pool still exists")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckResourcePoolExists(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_resource_pool" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_resource_pool", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestResourcePoolConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		name := rs.Primary.Attributes["name"]
		pool, err := connector.GetResourcePool(name)
		if pool == nil {
			return fmt.Errorf("resource pool does not exist")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}

		return nil
	}
}

type TestResourcePoolConnector interface {
	GetResourcePool(name string) (*model.ResourcePool, error)
}

type testResourcePoolConnector struct {
	c interface{}
}

func getTestResourcePoolConnector(a map[string]string) (TestResourcePoolConnector, error) {
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

	return testResourcePoolConnector{c: connector}, nil
}

func (t testResourcePoolConnector) GetResourcePool(name string) (*model.ResourcePool, error) {
	return t.c.(ResourcePoolConnector).GetResourcePool(context.Background(), name)
}
