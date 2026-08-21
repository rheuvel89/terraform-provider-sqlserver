package sqlserver

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDatabase_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckDatabaseDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDatabase(t, "basic", map[string]interface{}{"name": "tf_test_db_basic"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseExists("sqlserver_database.basic"),
					resource.TestCheckResourceAttr("sqlserver_database.basic", "name", "tf_test_db_basic"),
					resource.TestCheckResourceAttrSet("sqlserver_database.basic", "collation"),
					resource.TestCheckResourceAttrSet("sqlserver_database.basic", "recovery_model"),
					resource.TestCheckResourceAttrSet("sqlserver_database.basic", "compatibility_level"),
					resource.TestCheckResourceAttrSet("sqlserver_database.basic", "owner"),
				),
			},
		},
	})
}

func TestAccDatabase_Local_WithOptions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckDatabaseDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDatabase(t, "with_options", map[string]interface{}{
					"name":                "tf_test_db_opts",
					"recovery_model":      "SIMPLE",
					"compatibility_level": 130,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseExists("sqlserver_database.with_options"),
					resource.TestCheckResourceAttr("sqlserver_database.with_options", "name", "tf_test_db_opts"),
					resource.TestCheckResourceAttr("sqlserver_database.with_options", "recovery_model", "SIMPLE"),
					resource.TestCheckResourceAttr("sqlserver_database.with_options", "compatibility_level", "130"),
					resource.TestCheckResourceAttrSet("sqlserver_database.with_options", "collation"),
					resource.TestCheckResourceAttrSet("sqlserver_database.with_options", "owner"),
				),
			},
		},
	})
}

func TestAccDatabase_Local_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckDatabaseDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDatabase(t, "update", map[string]interface{}{
					"name":                "tf_test_db_update",
					"recovery_model":      "FULL",
					"compatibility_level": 130,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseExists("sqlserver_database.update"),
					resource.TestCheckResourceAttr("sqlserver_database.update", "name", "tf_test_db_update"),
					resource.TestCheckResourceAttr("sqlserver_database.update", "recovery_model", "FULL"),
					resource.TestCheckResourceAttr("sqlserver_database.update", "compatibility_level", "130"),
				),
			},
			{
				Config: testAccCheckDatabase(t, "update", map[string]interface{}{
					"name":                "tf_test_db_update",
					"recovery_model":      "SIMPLE",
					"compatibility_level": 150,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseExists("sqlserver_database.update"),
					resource.TestCheckResourceAttr("sqlserver_database.update", "name", "tf_test_db_update"),
					resource.TestCheckResourceAttr("sqlserver_database.update", "recovery_model", "SIMPLE"),
					resource.TestCheckResourceAttr("sqlserver_database.update", "compatibility_level", "150"),
				),
			},
		},
	})
}

func testAccCheckDatabase(t *testing.T, name string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
              login {}
            }

            resource "sqlserver_database" "{{ .rname }}" {
              name = "{{ .db_name }}"
              {{ with .recovery_model }}recovery_model = "{{ . }}"{{ end }}
              {{ with .compatibility_level }}compatibility_level = {{ . }}{{ end }}
            }`
	data["rname"] = name
	data["db_name"] = data["name"]

	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckDatabaseDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_database" {
			continue
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		dbName := rs.Primary.Attributes["name"]
		db, err := connector.GetDatabase(dbName)
		if db != nil {
			return fmt.Errorf("database [%s] still exists", dbName)
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckDatabaseExists(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_database" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_database", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		dbName := rs.Primary.Attributes["name"]
		db, err := connector.GetDatabase(dbName)
		if db == nil {
			return fmt.Errorf("database [%s] does not exist", dbName)
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
		return nil
	}
}
