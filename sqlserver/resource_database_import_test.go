package sqlserver

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDatabase_Local_BasicImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckDatabaseDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDatabase(t, "test_import", map[string]interface{}{"name": "tf_test_db_import"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseExists("sqlserver_database.test_import"),
					resource.TestCheckResourceAttr("sqlserver_database.test_import", "name", "tf_test_db_import"),
				),
			},
			{
				ResourceName:      "sqlserver_database.test_import",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccImportStateId("sqlserver_database.test_import", false),
			},
		},
	})
}

func TestAccDatabase_Local_ImportWithOptions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckDatabaseDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDatabase(t, "test_import_opts", map[string]interface{}{
					"name":                "tf_test_db_import_opts",
					"recovery_model":      "SIMPLE",
					"compatibility_level": 130,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatabaseExists("sqlserver_database.test_import_opts"),
					resource.TestCheckResourceAttr("sqlserver_database.test_import_opts", "name", "tf_test_db_import_opts"),
					resource.TestCheckResourceAttr("sqlserver_database.test_import_opts", "recovery_model", "SIMPLE"),
					resource.TestCheckResourceAttr("sqlserver_database.test_import_opts", "compatibility_level", "130"),
				),
			},
			{
				ResourceName:      "sqlserver_database.test_import_opts",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccImportStateId("sqlserver_database.test_import_opts", false),
			},
		},
	})
}
