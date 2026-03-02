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

func TestAccClassifierFunction_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckClassifierFunctionDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckClassifierFunction(t, "basic", map[string]interface{}{
					"name":        "TestClassifier",
					"schema_name": "dbo",
					"function_body": `DECLARE @grp_name SYSNAME
    SET @grp_name = 'default'
    RETURN @grp_name`,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClassifierFunctionExists("sqlserver_classifier_function.basic"),
					resource.TestCheckResourceAttr("sqlserver_classifier_function.basic", "name", "TestClassifier"),
					resource.TestCheckResourceAttr("sqlserver_classifier_function.basic", "schema_name", "dbo"),
					resource.TestCheckResourceAttr("sqlserver_classifier_function.basic", "fully_qualified_name", "dbo.TestClassifier"),
					resource.TestCheckResourceAttrSet("sqlserver_classifier_function.basic", "object_id"),
				),
			},
		},
	})
}

func TestAccClassifierFunction_Local_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckClassifierFunctionDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckClassifierFunction(t, "update_test", map[string]interface{}{
					"name":        "UpdateClassifier",
					"schema_name": "dbo",
					"function_body": `DECLARE @grp_name SYSNAME
    SET @grp_name = 'default'
    RETURN @grp_name`,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClassifierFunctionExists("sqlserver_classifier_function.update_test"),
				),
			},
			{
				Config: testAccCheckClassifierFunction(t, "update_test", map[string]interface{}{
					"name":        "UpdateClassifier",
					"schema_name": "dbo",
					"function_body": `DECLARE @grp_name SYSNAME
    IF APP_NAME() LIKE 'Report%'
        SET @grp_name = 'ReportingGroup'
    ELSE
        SET @grp_name = 'default'
    RETURN @grp_name`,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClassifierFunctionExists("sqlserver_classifier_function.update_test"),
				),
			},
		},
	})
}

func testAccCheckClassifierFunction(t *testing.T, name string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
               login {}
             }

             resource "sqlserver_classifier_function" "{{ .resource_name }}" {
               name          = "{{ .name }}"
               schema_name   = "{{ .schema_name }}"
               function_body = <<-EOF
{{ .function_body }}
EOF
             }`
	data["resource_name"] = name
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckClassifierFunctionDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_classifier_function" {
			continue
		}

		connector, err := getTestClassifierFunctionConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		schemaName := rs.Primary.Attributes["schema_name"]
		name := rs.Primary.Attributes["name"]
		fn, err := connector.GetClassifierFunction(schemaName, name)
		if fn != nil {
			return fmt.Errorf("classifier function still exists")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckClassifierFunctionExists(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_classifier_function" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_classifier_function", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestClassifierFunctionConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		schemaName := rs.Primary.Attributes["schema_name"]
		name := rs.Primary.Attributes["name"]
		fn, err := connector.GetClassifierFunction(schemaName, name)
		if fn == nil {
			return fmt.Errorf("classifier function does not exist")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}

		return nil
	}
}

type TestClassifierFunctionConnector interface {
	GetClassifierFunction(schemaName, name string) (*model.ClassifierFunction, error)
}

type testClassifierFunctionConnector struct {
	c interface{}
}

func getTestClassifierFunctionConnector(a map[string]string) (TestClassifierFunctionConnector, error) {
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

	return testClassifierFunctionConnector{c: connector}, nil
}

func (t testClassifierFunctionConnector) GetClassifierFunction(schemaName, name string) (*model.ClassifierFunction, error) {
	return t.c.(ClassifierFunctionConnector).GetClassifierFunction(context.Background(), schemaName, name)
}
