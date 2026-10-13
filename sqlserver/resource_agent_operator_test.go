package sqlserver

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAgentOperator_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckAgentOperatorDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckAgentOperator(t, "basic", map[string]interface{}{"name": "tf_test_operator", "email_address": "dba@example.com"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentOperatorExists("sqlserver_agent_operator.basic"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "name", "tf_test_operator"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "enabled", "true"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "email_address", "dba@example.com"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "category_name", "[Uncategorized]"),
					resource.TestCheckResourceAttrSet("sqlserver_agent_operator.basic", "operator_id"),
				),
			},
			{
				Config: testAccCheckAgentOperator(t, "basic", map[string]interface{}{"name": "tf_test_operator_renamed", "email_address": "oncall@example.com", "enabled": false}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentOperatorExists("sqlserver_agent_operator.basic"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "name", "tf_test_operator_renamed"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "enabled", "false"),
					resource.TestCheckResourceAttr("sqlserver_agent_operator.basic", "email_address", "oncall@example.com"),
				),
			},
		},
	})
}

func testAccCheckAgentOperator(t *testing.T, name string, data map[string]interface{}) string {
	text := `provider "sqlserver" {
              login {}
            }

            resource "sqlserver_agent_operator" "{{ .rname }}" {
              name = "{{ .name }}"
              {{ with .email_address }}email_address = "{{ . }}"{{ end }}
              {{ if eq .enabled false }}enabled = false{{ end }}
              {{ with .category_name }}category_name = "{{ . }}"{{ end }}
            }`
	data["rname"] = name
	if _, ok := data["enabled"]; !ok {
		data["enabled"] = true
	}

	res, err := templateToString(name, text, data)
	if err != nil {
		t.Fatalf("%s", err)
	}
	return res
}

func testAccCheckAgentOperatorDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_agent_operator" {
			continue
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		id, _ := strconv.Atoi(rs.Primary.Attributes["operator_id"])
		op, err := connector.GetAgentOperator(id)
		if op != nil {
			return fmt.Errorf("agent operator [%s] still exists", op.Name)
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckAgentOperatorExists(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no record ID is set")
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		id, _ := strconv.Atoi(rs.Primary.Attributes["operator_id"])
		op, err := connector.GetAgentOperator(id)
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
		if op == nil {
			return fmt.Errorf("agent operator [%d] does not exist", id)
		}
		if op.Name != rs.Primary.Attributes["name"] {
			return fmt.Errorf("agent operator [%d] is called %q, expected %q", id, op.Name, rs.Primary.Attributes["name"])
		}
		return nil
	}
}
