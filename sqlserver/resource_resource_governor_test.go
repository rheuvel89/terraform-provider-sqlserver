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

func TestAccResourceGovernor_Local_Enable(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckResourceGovernorDisabled(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourceGovernor(t, "enable", map[string]interface{}{
					"enabled": true,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGovernorEnabled("sqlserver_resource_governor.enable"),
					resource.TestCheckResourceAttr("sqlserver_resource_governor.enable", "enabled", "true"),
				),
			},
		},
	})
}

func TestAccResourceGovernor_Local_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckResourceGovernorDisabled(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourceGovernor(t, "update", map[string]interface{}{
					"enabled": true,
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_resource_governor.update", "enabled", "true"),
				),
			},
			{
				Config: testAccCheckResourceGovernor(t, "update", map[string]interface{}{
					"enabled": false,
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_resource_governor.update", "enabled", "false"),
				),
			},
		},
	})
}

func testAccCheckResourceGovernor(t *testing.T, name string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
               login {}
             }

             resource "sqlserver_resource_governor" "{{ .name }}" {
               enabled             = {{ .enabled }}
               {{ with .classifier_function }}classifier_function = "{{ . }}"{{ end }}
             }`
	data["name"] = name
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckResourceGovernorDisabled(state *terraform.State) error {
	connector, err := getTestResourceGovernorConnector(nil)
	if err != nil {
		return err
	}

	rg, err := connector.GetResourceGovernor()
	if err != nil {
		return fmt.Errorf("expected no error, got %s", err)
	}
	if rg.IsEnabled {
		return fmt.Errorf("resource governor should be disabled after destroy")
	}
	return nil
}

func testAccCheckResourceGovernorEnabled(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_resource_governor" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_resource_governor", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestResourceGovernorConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		rg, err := connector.GetResourceGovernor()
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
		if !rg.IsEnabled {
			return fmt.Errorf("resource governor is not enabled")
		}

		return nil
	}
}

type TestResourceGovernorConnector interface {
	GetResourceGovernor() (*model.ResourceGovernor, error)
}

type testResourceGovernorConnector struct {
	c interface{}
}

func getTestResourceGovernorConnector(a map[string]string) (TestResourceGovernorConnector, error) {
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

	return testResourceGovernorConnector{c: connector}, nil
}

func (t testResourceGovernorConnector) GetResourceGovernor() (*model.ResourceGovernor, error) {
	return t.c.(ResourceGovernorConnector).GetResourceGovernor(context.Background())
}
