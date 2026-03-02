package sqlserver

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccUser_Local_Instance(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "instance", "login", map[string]interface{}{"username": "instance", "login_name": "user_instance", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.instance"),
					testAccCheckDatabaseUserWorks("sqlserver_user.instance", "user_instance", "valueIsH8kd$¡"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "database", "master"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "instance_user.0.username", "instance"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "instance_user.0.login_name", "user_instance"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "authentication_type", "INSTANCE"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "roles.#", "1"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "roles.0", "db_owner"),
					resource.TestCheckResourceAttrSet("sqlserver_user.instance", "principal_id"),
					resource.TestCheckNoResourceAttr("sqlserver_user.instance", "database_user.#"),
				),
			},
		},
	})
}

func TestAccMultipleUsers_Local_Instance(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckMultipleUsers(t, "instance", "login", map[string]interface{}{"username": "instance", "login_name": "user_instance", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}, 4),
				Check:  resource.ComposeTestCheckFunc(getMultipleUsersExistAccCheck(4)...),
			},
		},
	})
}

func TestAccUser_Azure_Instance(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "instance", "azure", map[string]interface{}{"database": "testdb", "username": "instance", "login_name": "user_instance", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.instance"),
					testAccCheckDatabaseUserWorks("sqlserver_user.instance", "user_instance", "valueIsH8kd$¡"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "database", "testdb"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "instance_user.0.username", "instance"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "instance_user.0.login_name", "user_instance"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "authentication_type", "INSTANCE"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "roles.#", "1"),
					resource.TestCheckResourceAttr("sqlserver_user.instance", "roles.0", "db_owner"),
					resource.TestCheckResourceAttrSet("sqlserver_user.instance", "principal_id"),
					resource.TestCheckNoResourceAttr("sqlserver_user.instance", "database_user.#"),
				),
			},
		},
	})
}

func TestAccUser_Azure_Database(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "database", "azure", map[string]interface{}{"database": "testdb", "username": "database_user", "password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.database"),
					testAccCheckDatabaseUserWorks("sqlserver_user.database", "database_user", "valueIsH8kd$¡"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "database", "testdb"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "database_user.0.username", "database_user"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "database_user.0.password", "valueIsH8kd$¡"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "authentication_type", "DATABASE"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "roles.#", "1"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "roles.0", "db_owner"),
					resource.TestCheckResourceAttrSet("sqlserver_user.database", "principal_id"),
				),
			},
		},
	})
}

func TestAccUser_AzureadChain_Database(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "database", "fedauth", map[string]interface{}{"database": "testdb", "username": "database_user", "password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.database"),
					testAccCheckDatabaseUserWorks("sqlserver_user.database", "database_user", "valueIsH8kd$¡"),
				),
			},
		},
	})
}

func TestAccUser_AzureadMSI_Database(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "database", "msi", map[string]interface{}{"database": "testdb", "username": "database_user", "password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.database"),
					testAccCheckDatabaseUserWorks("sqlserver_user.database", "database_user", "valueIsH8kd$¡"),
				),
			},
		},
	})
}

func TestAccUser_Azure_External(t *testing.T) {
	tenantId := os.Getenv("TF_SQLSERVER_TENANT_ID")
	clientId := os.Getenv("TF_ACC_AZURE_USER_CLIENT_ID")
	clientUser := os.Getenv("TF_ACC_AZURE_USER_CLIENT_USER")
	clientSecret := os.Getenv("TF_ACC_AZURE_USER_CLIENT_SECRET")
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "database", "azure", map[string]interface{}{"database": "testdb", "username": clientUser, "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.database"),
					testAccCheckExternalUserWorks("sqlserver_user.database", tenantId, clientId, clientSecret),
					resource.TestCheckResourceAttr("sqlserver_user.database", "database", "testdb"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "external_user.0.username", clientUser),
					resource.TestCheckResourceAttr("sqlserver_user.database", "authentication_type", "EXTERNAL"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "roles.#", "1"),
					resource.TestCheckResourceAttr("sqlserver_user.database", "roles.0", "db_owner"),
					resource.TestCheckResourceAttrSet("sqlserver_user.database", "principal_id"),
					resource.TestCheckNoResourceAttr("sqlserver_user.database", "database_user.#"),
				),
			},
		},
	})
}

func TestAccUser_AzureadChain_External(t *testing.T) {
	tenantId := os.Getenv("TF_SQLSERVER_TENANT_ID")
	clientId := os.Getenv("TF_ACC_AZURE_USER_CLIENT_ID")
	clientUser := os.Getenv("TF_ACC_AZURE_USER_CLIENT_USER")
	clientSecret := os.Getenv("TF_ACC_AZURE_USER_CLIENT_SECRET")
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "database", "fedauth", map[string]interface{}{"database": "testdb", "username": clientUser, "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.database"),
					testAccCheckExternalUserWorks("sqlserver_user.database", tenantId, clientId, clientSecret),
				),
			},
		},
	})
}

func TestAccUser_AzureadMSI_External(t *testing.T) {
	tenantId := os.Getenv("TF_SQLSERVER_TENANT_ID")
	clientId := os.Getenv("TF_ACC_AZURE_USER_CLIENT_ID")
	clientUser := os.Getenv("TF_ACC_AZURE_USER_CLIENT_USER")
	clientSecret := os.Getenv("TF_ACC_AZURE_USER_CLIENT_SECRET")
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckUserDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "database", "msi", map[string]interface{}{"database": "testdb", "username": clientUser, "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists("sqlserver_user.database"),
					testAccCheckExternalUserWorks("sqlserver_user.database", tenantId, clientId, clientSecret),
				),
			},
		},
	})
}

func TestAccUser_Local_Update_Roles(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "update", "login", map[string]interface{}{"username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "0"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
			{
				Config: testAccCheckUser(t, "update", "login", map[string]interface{}{"username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\",\"db_datawriter\"]"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "2"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.0", "db_datawriter"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.1", "db_owner"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{"db_owner", "db_datawriter"}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
			{
				Config: testAccCheckUser(t, "update", "login", map[string]interface{}{"username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡", "roles": "[\"db_datawriter\",\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "2"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.0", "db_datawriter"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.1", "db_owner"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{"db_owner", "db_datawriter"}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
			{
				Config: testAccCheckUser(t, "update", "login", map[string]interface{}{"username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "1"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.0", "db_owner"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{"db_owner"}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
		},
	})
}

func TestAccUser_Azure_Update_Roles(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckLoginDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUser(t, "update", "azure", map[string]interface{}{"database": "testdb", "username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "0"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
			{
				Config: testAccCheckUser(t, "update", "azure", map[string]interface{}{"database": "testdb", "username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\",\"db_datawriter\"]"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "2"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.0", "db_datawriter"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.1", "db_owner"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{"db_owner", "db_datawriter"}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
			{
				Config: testAccCheckUser(t, "update", "azure", map[string]interface{}{"database": "testdb", "username": "test_update", "login_name": "user_update", "login_password": "valueIsH8kd$¡", "roles": "[\"db_owner\"]"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.#", "1"),
					resource.TestCheckResourceAttr("sqlserver_user.update", "roles.0", "db_owner"),
					testAccCheckUserExists("sqlserver_user.update", Check{"roles", "==", []string{"db_owner"}}),
					testAccCheckDatabaseUserWorks("sqlserver_user.update", "user_update", "valueIsH8kd$¡"),
				),
			},
		},
	})
}

func testAccCheckUser(t *testing.T, name string, login string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
             login {}
			}
	
			{{ if .login_name }}
           resource "sqlserver_login" "{{ .name }}" {
			 sql_login {
               login_name = "{{ .login_name }}"
               password   = "{{ .login_password }}"
			 }
           }
           {{ end }}
           resource "sqlserver_user" "{{ .name }}" {
             {{ with .database }}database = "{{ . }}"{{ end }}
             {{ if .login_name }}
             instance_user {
               username   = "{{ .username }}"
               login_name = "{{ .login_name }}"
             }
             {{ else if .password }}
             database_user {
               username = "{{ .username }}"
               password = "{{ .password }}"
             }
             {{ else }}
             external_user {
               username = "{{ .username }}"
             }
             {{ end }}
             {{ with .roles }}roles = {{ . }}{{ end }}
           }`
	data["name"] = name
	data["login"] = login
	switch login {
	case "fedauth", "msi", "azure":
		data["host"] = os.Getenv("TF_ACC_SQL_SERVER")
	case "login":
		data["host"] = "localhost"
	default:
		t.Fatalf("login expected to be one of 'login', 'azure', 'msi', 'fedauth', got %s", login)
	}
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckMultipleUsers(t *testing.T, name string, login string, data map[string]interface{}, count int) string {
	text := `provider "sqlserver" {
             login {}
			}
			 
			{{ if .login_name }}
           resource "sqlserver_login" "{{ .name }}" {
             count = {{ .count }}
             sql_login {
               login_name = "{{ .login_name }}-${count.index}"
               password   = "{{ .login_password }}"
             }
           }
           {{ end }}
           resource "sqlserver_user" "{{ .name }}" {
             count = {{ .count }}
            
             {{ with .database }}database = "{{ . }}"{{ end }}
             {{ if .login_name }}
             instance_user {
               username   = "{{ .username }}-${count.index}"
               login_name = "{{ .login_name }}-${count.index}"
             }
             {{ else if .password }}
             database_user {
               username = "{{ .username }}-${count.index}"
               password = "{{ .password }}"
             }
             {{ else }}
             external_user {
               username = "{{ .username }}-${count.index}"
             }
             {{ end }}
             {{ with .roles }}roles = {{ . }}{{ end }}
			  depends_on = [{{ if .login_name }}sqlserver_login.{{ .name }}{{ end }}]
           }`
	data["name"] = name
	data["login"] = login
	data["count"] = count
	if login == "fedauth" || login == "msi" || login == "azure" {
		data["host"] = os.Getenv("TF_ACC_SQL_SERVER")
	} else if login == "login" {
		data["host"] = "localhost"
	} else {
		t.Fatalf("login expected to be one of 'login', 'azure', 'msi', 'fedauth', got %s", login)
	}
	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckUserDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_user" {
			continue
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		database := rs.Primary.Attributes["database"]
		username := getUsernameFromAttributes(rs.Primary.Attributes)
		login, err := connector.GetUser(database, username)
		if login != nil {
			return fmt.Errorf("user still exists")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

// getUsernameFromAttributes extracts username from the nested user block in test attributes
func getUsernameFromAttributes(attrs map[string]string) string {
	if username, ok := attrs["instance_user.0.username"]; ok && username != "" {
		return username
	}
	if username, ok := attrs["database_user.0.username"]; ok && username != "" {
		return username
	}
	if username, ok := attrs["external_user.0.username"]; ok && username != "" {
		return username
	}
	return ""
}

func testAccCheckUserExists(resource string, checks ...Check) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_user" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_user", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}
		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		database := rs.Primary.Attributes["database"]
		username := getUsernameFromAttributes(rs.Primary.Attributes)
		user, err := connector.GetUser(database, username)
		if user == nil {
			return fmt.Errorf("user does not exist")
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}

		var actual interface{}
		for _, check := range checks {
			switch check.name {
			case "password":
				actual = user.Password
			case "login_name":
				actual = user.LoginName
			case "roles":
				actual = user.Roles
			case "authentication_type":
				actual = user.AuthType
			default:
				return fmt.Errorf("unknown property %s", check.name)
			}
			if (check.op == "" || check.op == "==") && !equal(check.expected, actual) {
				return fmt.Errorf("expected %s == %s, got %s", check.name, check.expected, actual)
			}
			if check.op == "!=" && equal(check.expected, actual) {
				return fmt.Errorf("expected %s != %s, got %s", check.name, check.expected, actual)
			}
		}
		return nil
	}
}

func equal(a, b interface{}) bool {
	switch a.(type) {
	case []string:
		aa := a.([]string)
		bb := b.([]string)
		if len(aa) != len(bb) {
			return false
		}
		for i, v := range aa {
			if v != bb[i] {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func testAccCheckDatabaseUserWorks(resource string, username, password string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_user" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_user", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}
		connector, err := getTestUserConnector(rs.Primary.Attributes, username, password)
		if err != nil {
			return err
		}
		current, system, err := connector.GetCurrentUser(rs.Primary.Attributes[databaseProp])
		if err != nil {
			return fmt.Errorf("error: %s", err)
		}
		expectedUsername := getUsernameFromAttributes(rs.Primary.Attributes)
		if current != expectedUsername {
			return fmt.Errorf("expected to be user %s, got %s (%s)", expectedUsername, current, system)
		}
		return nil
	}
}

func testAccCheckExternalUserWorks(resource string, tenantId, clientId, clientSecret string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Type != "sqlserver_user" {
			return fmt.Errorf("expected resource of type %s, got %s", "sqlserver_user", rs.Type)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}
		connector, err := getTestExternalConnector(rs.Primary.Attributes, tenantId, clientId, clientSecret)
		if err != nil {
			return err
		}
		current, system, err := connector.GetCurrentUser(rs.Primary.Attributes[databaseProp])
		if err != nil {
			return fmt.Errorf("error: %s", err)
		}
		expectedUsername := getUsernameFromAttributes(rs.Primary.Attributes)
		if current != expectedUsername {
			return fmt.Errorf("expected to be user %s, got %s (%s)", expectedUsername, current, system)
		}
		return nil
	}
}

func getMultipleUsersExistAccCheck(count int) []resource.TestCheckFunc {
	checkFuncs := []resource.TestCheckFunc{}
	for i := 0; i < count; i++ {
		checkFuncs = append(checkFuncs, []resource.TestCheckFunc{
			testAccCheckUserExists(fmt.Sprintf("sqlserver_user.instance.%v", i)),
			testAccCheckDatabaseUserWorks(fmt.Sprintf("sqlserver_user.instance.%v", i), fmt.Sprintf("user_instance-%v", i), "valueIsH8kd$¡"),
			resource.TestCheckResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "database", "master"),
			resource.TestCheckResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "instance_user.0.username", fmt.Sprintf("instance-%v", i)),
			resource.TestCheckResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "instance_user.0.login_name", fmt.Sprintf("user_instance-%v", i)),
			resource.TestCheckResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "authentication_type", "INSTANCE"),
			resource.TestCheckResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "roles.#", "1"),
			resource.TestCheckResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "roles.0", "db_owner"),
			resource.TestCheckResourceAttrSet(fmt.Sprintf("sqlserver_user.instance.%v", i), "principal_id"),
			resource.TestCheckNoResourceAttr(fmt.Sprintf("sqlserver_user.instance.%v", i), "database_user.#"),
		}...,
		)
	}
	return checkFuncs
}
