package sqlserver

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccLogin_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "basic", false, map[string]interface{}{"login_name": "login_basic", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoginExists("sqlserver_login.basic"),
					testAccCheckLoginWorks("sqlserver_login.basic"),
					resource.TestCheckResourceAttr("sqlserver_login.basic", "sql_login.0.login_name", "login_basic"),
					resource.TestCheckResourceAttr("sqlserver_login.basic", "sql_login.0.password", "valueIsH8kd$¡"),
					resource.TestCheckResourceAttrSet("sqlserver_login.basic", "principal_id"),
				),
			},
		},
	})
}

// func TestAccLogin_Local_Basic_SID(t *testing.T) {
// 	resource.Test(t, resource.TestCase{
// 		PreCheck:          func() { testAccPreCheck(t) },
// 		IsUnitTest:        runLocalAccTests,
// 		ProviderFactories: testAccProviders,
// 		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
// 		Steps: []resource.TestStep{
// 			{
// 				Config: testAccCheckLogin(t, "basic", false, map[string]interface{}{"login_name": "login_basic", "password": "valueIsH8kd$¡", "sid": "0xB7BDEF7990D03541BAA2AD73E4FF18E8"}),
// 				Check: resource.ComposeTestCheckFunc(
// 					testAccCheckLoginExists("sqlserver_login.basic"),
// 					testAccCheckLoginWorks("sqlserver_login.basic"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "sql_login.0.login_name", "login_basic"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "sql_login.0.password", "valueIsH8kd$¡"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "sid", "0xB7BDEF7990D03541BAA2AD73E4FF18E8"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.#", "1"),
// 					resource.TestCheckResourceAttrSet("sqlserver_login.basic", "principal_id"),
// 				),
// 			},
// 		},
// 	})
// }

func TestAccLogin_Azure_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "basic", true, map[string]interface{}{"login_name": "login_basic", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoginExists("sqlserver_login.basic"),
					resource.TestCheckResourceAttr("sqlserver_login.basic", "sql_login.0.login_name", "login_basic"),
					resource.TestCheckResourceAttr("sqlserver_login.basic", "sql_login.0.password", "valueIsH8kd$¡"),
					resource.TestCheckResourceAttrSet("sqlserver_login.basic", "principal_id"),
				),
			},
		},
	})
}

// func TestAccLogin_Azure_Basic_SID(t *testing.T) {
// 	resource.Test(t, resource.TestCase{
// 		PreCheck:          func() { testAccPreCheck(t) },
// 		ProviderFactories: testAccProviders,
// 		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
// 		Steps: []resource.TestStep{
// 			{
// 				Config: testAccCheckLogin(t, "basic", true, map[string]interface{}{"login_name": "login_basic", "password": "valueIsH8kd$¡", "sid": "0x01060000000000640000000000000000BAF5FC800B97EF49AC6FD89469C4987F"}),
// 				Check: resource.ComposeTestCheckFunc(
// 					testAccCheckLoginExists("sqlserver_login.basic"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "login_name", "login_basic"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "password", "valueIsH8kd$¡"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "sid", "0x01060000000000640000000000000000BAF5FC800B97EF49AC6FD89469C4987F"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.#", "1"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.host", os.Getenv("TF_ACC_SQL_SERVER")),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.port", "1433"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.azure_login.#", "1"),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.azure_login.0.tenant_id", os.Getenv("TF_SQLSERVER_TENANT_ID")),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.azure_login.0.client_id", os.Getenv("TF_SQLSERVER_CLIENT_ID")),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.azure_login.0.client_secret", os.Getenv("TF_CLIENT_SECRET")),
// 					resource.TestCheckResourceAttr("sqlserver_login.basic", "server.0.login.#", "0"),
// 					resource.TestCheckResourceAttrSet("sqlserver_login.basic", "principal_id"),
// 				),
// 			},
// 		},
// 	})
// }

func TestAccLogin_Local_UpdateLoginName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_update", false, map[string]interface{}{"login_name": "login_update_pre", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.login_name", "login_update_pre"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
					testAccCheckLoginWorks("sqlserver_login.test_update"),
				),
			},
			{
				Config: testAccCheckLogin(t, "test_update", false, map[string]interface{}{"login_name": "login_update_post", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.login_name", "login_update_post"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
					testAccCheckLoginWorks("sqlserver_login.test_update"),
				),
			},
		}})
}

func TestAccLogin_Local_UpdatePassword(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_update", false, map[string]interface{}{"login_name": "login_update", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.password", "valueIsH8kd$¡"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
					testAccCheckLoginWorks("sqlserver_login.test_update"),
				),
			},
			{
				Config: testAccCheckLogin(t, "test_update", false, map[string]interface{}{"login_name": "login_update", "password": "otherIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.password", "otherIsH8kd$¡"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
					testAccCheckLoginWorks("sqlserver_login.test_update"),
				),
			},
		}})
}

func TestAccLogin_Azure_UpdateLoginName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_update", true, map[string]interface{}{"login_name": "login_update_pre", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.login_name", "login_update_pre"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
				),
			},
			{
				Config: testAccCheckLogin(t, "test_update", true, map[string]interface{}{"login_name": "login_update_post", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.login_name", "login_update_post"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
				),
			},
		}})
}

func TestAccLogin_Azure_UpdatePassword(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogin(t, "test_update", true, map[string]interface{}{"login_name": "login_update", "password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.password", "valueIsH8kd$¡"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
				),
			},
			{
				Config: testAccCheckLogin(t, "test_update", true, map[string]interface{}{"login_name": "login_update", "password": "otherIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_login.test_update", "sql_login.0.password", "otherIsH8kd$¡"),
					testAccCheckLoginExists("sqlserver_login.test_update"),
				),
			},
		}})
}

func testAccCheckLogin(t *testing.T, name string, azure bool, data map[string]interface{}) string {
	text := `provider "sqlserver" {
             login {}
			}
	
	 		 resource "sqlserver_login" "{{ .name }}" {
	         sql_login {
             login_name = "{{ .login_name }}"
             password   = "{{ .password }}"
             }
             {{ with .sid }}sid = "{{ . }}"{{ end }}
           }`
	data["name"] = name
	data["azure"] = azure
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckLoginDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_login" {
			continue
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		loginName := rs.Primary.Attributes["sql_login.0.login_name"]
		login, err := connector.GetLogin(loginName)
		if login != nil {
			return fmt.Errorf("login still exists")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckLoginExists(resource string, checks ...Check) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_login" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_login", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		loginName := rs.Primary.Attributes["sql_login.0.login_name"]
		login, err := connector.GetLogin(loginName)
		if login == nil {
			return fmt.Errorf("login does not exist")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}

		var actual interface{}
		for _, check := range checks {
			if (check.op == "" || check.op == "==") && check.expected != actual {
				return fmt.Errorf("expected %s == %s, got %s", check.name, check.expected, actual)
			}
			if check.op == "!=" && check.expected == actual {
				return fmt.Errorf("expected %s != %s, got %s", check.name, check.expected, actual)
			}
		}
		return nil
	}
}

func testAccCheckLoginWorks(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_login" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_login", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}
		connector, err := getTestLoginConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}
		systemUser, err := connector.GetSystemUser()
		if err != nil {
			return err
		}
		if systemUser != rs.Primary.Attributes["sql_login.0.login_name"] {
			return fmt.Errorf("expected to log in as [%s], got [%s]", rs.Primary.Attributes["sql_login.0.login_name"], systemUser)
		}
		return nil
	}
}
