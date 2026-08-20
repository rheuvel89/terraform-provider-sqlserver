package sqlserver

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccLogin_Local_BasicImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_import", false, map[string]interface{}{"login_name": "login_import", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoginExists("sqlserver_login.test_import"),
					resource.TestCheckResourceAttr("sqlserver_login.test_import", "sql_login.0.login_name", "login_import"),
					resource.TestCheckResourceAttrSet("sqlserver_login.test_import", "principal_id"),
					resource.TestCheckResourceAttr("sqlserver_login.test_import", "is_disabled", "false"),
				),
			},
			{
				ResourceName:            "sqlserver_login.test_import",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sql_login.0.password"},
				ImportStateIdFunc:       testAccImportStateId("sqlserver_login.test_import", false),
			},
		},
	})
}

func TestAccLogin_Local_ImportWithRoles(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_import_roles", false, map[string]interface{}{"login_name": "login_import_roles", "password": "valueIsH8kd$¡", "roles": "[\"dbcreator\",\"diskadmin\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoginExists("sqlserver_login.test_import_roles", Check{"roles", "==", []string{"dbcreator", "diskadmin"}}),
					resource.TestCheckResourceAttr("sqlserver_login.test_import_roles", "roles.#", "2"),
				),
			},
			{
				ResourceName:            "sqlserver_login.test_import_roles",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sql_login.0.password"},
				ImportStateIdFunc:       testAccImportStateId("sqlserver_login.test_import_roles", false),
			},
		},
	})
}

func TestAccLogin_Local_ImportDisabled(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_import_disabled", false, map[string]interface{}{"login_name": "login_import_disabled", "password": "valueIsH8kd$¡", "is_disabled": "true"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoginExists("sqlserver_login.test_import_disabled"),
					resource.TestCheckResourceAttr("sqlserver_login.test_import_disabled", "is_disabled", "true"),
				),
			},
			{
				ResourceName:            "sqlserver_login.test_import_disabled",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sql_login.0.password"},
				ImportStateIdFunc:       testAccImportStateId("sqlserver_login.test_import_disabled", false),
			},
		},
	})
}

func TestAccLogin_Azure_BasicImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_import_azure", true, map[string]interface{}{"login_name": "login_import_azure", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoginExists("sqlserver_login.test_import_azure"),
				),
			},
			{
				ResourceName:            "sqlserver_login.test_import_azure",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sql_login.0.password"},
				ImportStateIdFunc:       testAccImportStateId("sqlserver_login.test_import_azure", true),
			},
		},
	})
}
