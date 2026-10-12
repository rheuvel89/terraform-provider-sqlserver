package sqlserver

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAgentJob_Local_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckAgentJobDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckAgentJob(t, `name = "tf_test_job"`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentJobExists("sqlserver_agent_job.test"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "name", "tf_test_job"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "description", ""),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "enabled", "true"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "category_name", "[Uncategorized (Local)]"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "notify_level_eventlog", "on_failure"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "notify_level_email", "never"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "delete_level", "never"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "start_step_id", "1"),
					resource.TestCheckResourceAttrSet("sqlserver_agent_job.test", "owner_login_name"),
					resource.TestCheckResourceAttrSet("sqlserver_agent_job.test", "job_id"),
				),
			},
			{
				Config: testAccCheckAgentJob(t, `
                  name                  = "tf_test_job_renamed"
                  description           = "nightly ETL"
                  enabled               = false
                  category_name         = "Database Maintenance"
                  notify_level_eventlog = "always"`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentJobExists("sqlserver_agent_job.test"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "name", "tf_test_job_renamed"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "description", "nightly ETL"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "enabled", "false"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "category_name", "Database Maintenance"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "notify_level_eventlog", "always"),
				),
			},
			{
				// Removing description and category_name from config reverts them to their defaults.
				Config: testAccCheckAgentJob(t, `name = "tf_test_job_renamed"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "description", ""),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "category_name", "[Uncategorized (Local)]"),
					testAccCheckAgentJobAttr("sqlserver_agent_job.test", func(rs map[string]string) error {
						connector, err := getTestConnector(rs)
						if err != nil {
							return err
						}
						job, err := connector.GetAgentJob(rs["job_id"])
						if err != nil {
							return err
						}
						if job.CategoryName != "[Uncategorized (Local)]" || job.Description != "" {
							return fmt.Errorf("expected default category and empty description, got %q / %q", job.CategoryName, job.Description)
						}
						return nil
					}),
				),
			},
		},
	})
}

func TestAccAgentJob_Local_EmailOperator(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy: func(state *terraform.State) error {
			if err := testAccCheckAgentJobDestroy(state); err != nil {
				return err
			}
			return testAccCheckAgentOperatorDestroy(state)
		},
		Steps: []resource.TestStep{
			{
				Config: `provider "sqlserver" {
                           login {}
                         }
                         resource "sqlserver_agent_operator" "dba" {
                           name          = "tf_test_job_operator"
                           email_address = "dba@example.com"
                         }
                         resource "sqlserver_agent_job" "test" {
                           name                       = "tf_test_job_email"
                           notify_level_email         = "on_failure"
                           notify_email_operator_name = sqlserver_agent_operator.dba.name
                         }`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentJobExists("sqlserver_agent_job.test"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "notify_level_email", "on_failure"),
					resource.TestCheckResourceAttr("sqlserver_agent_job.test", "notify_email_operator_name", "tf_test_job_operator"),
				),
			},
		},
	})
}

func TestAccAgentJob_Local_EmailWithoutOperator(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckAgentJob(t, `
                  name               = "tf_test_job_noop"
                  notify_level_email = "always"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("notify_email_operator_name is required"),
			},
		},
	})
}

func testAccCheckAgentJob(t *testing.T, body string) string {
	return fmt.Sprintf(`provider "sqlserver" {
                          login {}
                        }

                        resource "sqlserver_agent_job" "test" {
                          %s
                        }`, body)
}

func testAccCheckAgentJobAttr(resource string, check func(map[string]string) error) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("not found: %s", resource)
		}
		return check(rs.Primary.Attributes)
	}
}

func testAccCheckAgentJobDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_agent_job" {
			continue
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		job, err := connector.GetAgentJob(rs.Primary.Attributes["job_id"])
		if job != nil {
			return fmt.Errorf("agent job [%s] still exists", job.Name)
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}

func testAccCheckAgentJobExists(resource string) resource.TestCheckFunc {
	return testAccCheckAgentJobAttr(resource, func(rs map[string]string) error {
		connector, err := getTestConnector(rs)
		if err != nil {
			return err
		}

		job, err := connector.GetAgentJob(rs["job_id"])
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
		if job == nil {
			return fmt.Errorf("agent job [%s] does not exist", rs["job_id"])
		}
		if job.Name != rs["name"] {
			return fmt.Errorf("agent job [%s] is called %q, expected %q", rs["job_id"], job.Name, rs["name"])
		}
		return nil
	})
}

func TestAccAgentJob_Local_DuplicateName(t *testing.T) {
	// A failing msdb procedure must surface SQL Server's error rather than an empty job ID.
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckAgentJobDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccProviderBlock + `
                  resource "sqlserver_agent_job" "first" {
                    name = "tf_test_job_duplicate"
                  }
                  resource "sqlserver_agent_job" "second" {
                    name       = "tf_test_job_duplicate"
                    depends_on = [sqlserver_agent_job.first]
                  }`,
				ExpectError: regexp.MustCompile(`The specified @name \('tf_test_job_duplicate'\) already exists`),
			},
		},
	})
}
