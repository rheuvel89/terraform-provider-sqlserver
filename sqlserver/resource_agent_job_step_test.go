package sqlserver

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// testStep is one sqlserver_agent_job_step in a test config. The resource is named after the step.
type testStep struct {
	name  string
	after string // previous step, "" for the first step
	extra string // more attributes
}

func testAccAgentJobStepsConfig(jobName string, steps ...testStep) string {
	var b strings.Builder
	b.WriteString(testAccProviderBlock)
	fmt.Fprintf(&b, `
      resource "sqlserver_agent_job" "test" {
        name = %q
      }
`, jobName)
	for _, s := range steps {
		placement := "job_id = sqlserver_agent_job.test.job_id"
		if s.after != "" {
			placement = fmt.Sprintf("previous_step_id = sqlserver_agent_job_step.%s.id", s.after)
		}
		command := fmt.Sprintf(`command = "SELECT '%s'"`, s.name)
		if strings.Contains(s.extra, "command") {
			command = ""
		}
		fmt.Fprintf(&b, `
      resource "sqlserver_agent_job_step" %q {
        %s
        name = %q
        %s
        %s
      }
`, s.name, placement, s.name, command, s.extra)
	}
	return b.String()
}

func testAccAgentJobSteps(t *testing.T) func(state *terraform.State) []model.AgentJobStep {
	return func(state *terraform.State) []model.AgentJobStep {
		rs := state.RootModule().Resources["sqlserver_agent_job.test"]
		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := connector.ListAgentJobSteps(rs.Primary.Attributes["job_id"])
		if err != nil {
			t.Fatal(err)
		}
		return steps
	}
}

// testAccCheckAgentJobStepOrder checks the order of the job's steps in msdb.
func testAccCheckAgentJobStepOrder(t *testing.T, names ...string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		var got []string
		for _, s := range testAccAgentJobSteps(t)(state) {
			got = append(got, s.Name)
		}
		if strings.Join(got, ",") != strings.Join(names, ",") {
			return fmt.Errorf("steps are %v, expected %v", got, names)
		}
		return nil
	}
}

// testAccCheckAgentJump checks a step's action for an outcome in msdb. target is the name of the step it goes to,
// or "" when it should have the outcome's default action.
func testAccCheckAgentJump(t *testing.T, source string, o agentOutcome, target string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		steps := testAccAgentJobSteps(t)(state)
		s := findAgentStep(steps, source)
		if s == nil {
			return fmt.Errorf("no step %q", source)
		}
		action, targetID := o.action(*s)
		if target == "" {
			if action != o.defaultAction() {
				return fmt.Errorf("step %q: %s action is %d, expected the default %d", source, o, action, o.defaultAction())
			}
			return nil
		}
		want := findAgentStep(steps, target)
		if action != agentActionGoToStep || want == nil || targetID != want.StepID {
			return fmt.Errorf("step %q: %s action is %d to step %d, expected go to step %q", source, o, action, targetID, target)
		}
		return nil
	}
}

func testAccAgentJobStepCase(t *testing.T, steps ...resource.TestStep) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckAgentJobDestroy(state) },
		Steps:             steps,
	})
}

func TestAccAgentJobStep_Local_Chain(t *testing.T) {
	const job = "tf_test_step_chain"
	testAccAgentJobStepCase(t,
		resource.TestStep{
			Config: testAccAgentJobStepsConfig(job, testStep{"extract", "", ""}, testStep{"transform", "extract", ""}, testStep{"load", "transform", ""}),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJobStepOrder(t, "extract", "transform", "load"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.load", "step_id", "3"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.load", "database_name", "master"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.load", "on_success_action", "go_to_next_step"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.load", "on_fail_action", "quit_with_failure"),
				resource.TestCheckResourceAttrPair("sqlserver_agent_job_step.load", "job_id", "sqlserver_agent_job.test", "job_id"),
			),
		},
		resource.TestStep{
			// Insert a step in the middle.
			Config: testAccAgentJobStepsConfig(job, testStep{"extract", "", ""}, testStep{"clean", "extract", ""}, testStep{"transform", "clean", ""}, testStep{"load", "transform", ""}),
			Check:  testAccCheckAgentJobStepOrder(t, "extract", "clean", "transform", "load"),
		},
		resource.TestStep{
			// Delete a middle step.
			Config: testAccAgentJobStepsConfig(job, testStep{"extract", "", ""}, testStep{"clean", "extract", ""}, testStep{"load", "clean", ""}),
			Check:  testAccCheckAgentJobStepOrder(t, "extract", "clean", "load"),
		},
		resource.TestStep{
			// Move the last step to the head.
			Config: testAccAgentJobStepsConfig(job, testStep{"load", "", ""}, testStep{"extract", "load", ""}, testStep{"clean", "extract", ""}),
			Check:  testAccCheckAgentJobStepOrder(t, "load", "extract", "clean"),
		},
	)
}

func TestAccAgentJobStep_Local_DeleteAndUpdateTogether(t *testing.T) {
	// Deleting step 2 renumbers step 5 while it's being updated; the lock makes sure the right step changes.
	const job = "tf_test_step_lock"
	five := []testStep{{"s1", "", ""}, {"s2", "s1", ""}, {"s3", "s2", ""}, {"s4", "s3", ""}, {"s5", "s4", ""}}
	after := []testStep{{"s1", "", ""}, {"s3", "s1", ""}, {"s4", "s3", ""}, {"s5", "s4", `command = "SELECT 'changed'"`}}
	testAccAgentJobStepCase(t,
		resource.TestStep{Config: testAccAgentJobStepsConfig(job, five...)},
		resource.TestStep{
			Config: testAccAgentJobStepsConfig(job, after...),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJobStepOrder(t, "s1", "s3", "s4", "s5"),
				func(state *terraform.State) error {
					for _, s := range testAccAgentJobSteps(t)(state) {
						changed := s.Command == "SELECT 'changed'"
						if changed != (s.Name == "s5") {
							return fmt.Errorf("step %q has command %q", s.Name, s.Command)
						}
					}
					return nil
				},
			),
		},
	)
}

func TestAccAgentJobStep_Local_Forks(t *testing.T) {
	const job = "tf_test_step_forks"
	chain := []testStep{{"a", "", ""}, {"b", "a", ""}}
	testAccAgentJobStepCase(t,
		resource.TestStep{Config: testAccAgentJobStepsConfig(job, chain...)},
		resource.TestStep{
			// Insert x after a but leave b pointing at a: only the new step has changes.
			Config:      testAccAgentJobStepsConfig(job, append(chain, testStep{"x", "a", ""})...),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`steps "b" and "x" both follow "a"|steps "x" and "b" both follow "a"`),
		},
		resource.TestStep{
			Config:      testAccAgentJobStepsConfig(job, append(chain, testStep{"y", "", ""})...),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`are both the first step`),
		},
	)
}

func TestAccAgentJobStep_Local_ForkInNewChain(t *testing.T) {
	// previous_step_id is unknown at plan time for a brand-new chain, so the fork is caught during apply.
	testAccAgentJobStepCase(t,
		resource.TestStep{
			Config:      testAccAgentJobStepsConfig("tf_test_step_newfork", testStep{"a", "", ""}, testStep{"b", "a", ""}, testStep{"c", "a", ""}),
			ExpectError: regexp.MustCompile(`both follow "a"`),
		},
	)
}

func TestAccAgentJobStep_Local_Jumps(t *testing.T) {
	const job = "tf_test_step_jumps"
	backward := testStep{"load", "transform", `
        on_fail_action    = "go_to_step"
        on_fail_step_name = sqlserver_agent_job_step.extract.name`}
	notify := testStep{"notify", "load", `handles_failure_of = [sqlserver_agent_job_step.extract.name, sqlserver_agent_job_step.transform.name]`}

	testAccAgentJobStepCase(t,
		resource.TestStep{
			// A backward jump on the source and a forward jump on the target, created in one apply.
			Config: testAccAgentJobStepsConfig(job, testStep{"extract", "", ""}, testStep{"transform", "extract", ""}, backward, notify),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJump(t, "load", agentOutcomeFailure, "extract"),
				testAccCheckAgentJump(t, "extract", agentOutcomeFailure, "notify"),
				testAccCheckAgentJump(t, "transform", agentOutcomeFailure, "notify"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.notify", "handles_failure_of.#", "2"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.extract", "on_fail_action", "quit_with_failure"),
			),
		},
		resource.TestStep{
			// Insert a step before the forward jump's target and change the source's command: the jumps survive.
			Config: testAccAgentJobStepsConfig(job,
				testStep{"extract", "", `command = "SELECT 'extract v2'"`}, testStep{"transform", "extract", ""}, testStep{"validate", "transform", ""},
				testStep{"load", "validate", backward.extra}, notify),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJobStepOrder(t, "extract", "transform", "validate", "load", "notify"),
				testAccCheckAgentJump(t, "load", agentOutcomeFailure, "extract"),
				testAccCheckAgentJump(t, "extract", agentOutcomeFailure, "notify"),
				testAccCheckAgentJump(t, "transform", agentOutcomeFailure, "notify"),
			),
		},
		resource.TestStep{
			// Delete the step directly before the target of a backward jump (validate is before load, whose jump goes to
			// extract) and directly before notify's sources: sp_delete_jobstep loses jumps to the next step (spike S1).
			Config: testAccAgentJobStepsConfig(job,
				testStep{"extract", "", `command = "SELECT 'extract v2'"`}, testStep{"transform", "extract", ""},
				testStep{"load", "transform", backward.extra}, notify),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJobStepOrder(t, "extract", "transform", "load", "notify"),
				testAccCheckAgentJump(t, "load", agentOutcomeFailure, "extract"),
				testAccCheckAgentJump(t, "extract", agentOutcomeFailure, "notify"),
				testAccCheckAgentJump(t, "transform", agentOutcomeFailure, "notify"),
			),
		},
		resource.TestStep{
			// Remove a source from handles_failure_of, then the target itself: the sources fall back to the default.
			Config: testAccAgentJobStepsConfig(job,
				testStep{"extract", "", `command = "SELECT 'extract v2'"`}, testStep{"transform", "extract", ""},
				testStep{"load", "transform", backward.extra}, testStep{"notify", "load", `handles_failure_of = [sqlserver_agent_job_step.transform.name]`}),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJump(t, "extract", agentOutcomeFailure, ""),
				testAccCheckAgentJump(t, "transform", agentOutcomeFailure, "notify"),
			),
		},
		resource.TestStep{
			Config: testAccAgentJobStepsConfig(job,
				testStep{"extract", "", `command = "SELECT 'extract v2'"`}, testStep{"transform", "extract", ""},
				testStep{"load", "transform", backward.extra}),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckAgentJobStepOrder(t, "extract", "transform", "load"),
				testAccCheckAgentJump(t, "transform", agentOutcomeFailure, ""),
				testAccCheckAgentJump(t, "load", agentOutcomeFailure, "extract"),
			),
		},
	)
}

func TestAccAgentJobStep_Local_JumpConflicts(t *testing.T) {
	const job = "tf_test_step_conflicts"
	base := []testStep{{"a", "", ""}, {"b", "a", ""}}
	testAccAgentJobStepCase(t,
		resource.TestStep{Config: testAccAgentJobStepsConfig(job, base...)},
		resource.TestStep{
			// Two targets claim a's failure.
			Config: testAccAgentJobStepsConfig(job, testStep{"a", "", ""}, testStep{"b", "a", `handles_failure_of = ["a"]`},
				testStep{"c", "b", `handles_failure_of = ["a"]`}),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`both handle the failure of step "a"`),
		},
		resource.TestStep{
			// on_*_step_name can't go forward.
			Config: testAccAgentJobStepsConfig(job, testStep{"a", "", `
              on_success_action    = "go_to_step"
              on_success_step_name = "b"`}, testStep{"b", "a", ""}),
			ExpectError: regexp.MustCompile(`can only go to an earlier step`),
		},
		resource.TestStep{
			// The source already sets its own failure action.
			Config: testAccAgentJobStepsConfig(job, testStep{"a", "", `on_fail_action = "quit_with_success"`},
				testStep{"b", "a", `handles_failure_of = [sqlserver_agent_job_step.a.name]`}),
			ExpectError: regexp.MustCompile(`step "a" already has on_fail_action = "quit_with_success"`),
		},
	)
}

func TestAccAgentJobStep_Local_DatabaseAndRunAs(t *testing.T) {
	const job = "tf_test_step_runas"
	setup := `
      resource "sqlserver_database" "runas" {
        name = "tf_test_step_runas_db"
      }
`
	runAs := func(extra string) string {
		return testAccAgentJobStepsConfig(job, testStep{"s", "", extra}) + setup
	}

	testAccAgentJobStepCase(t,
		resource.TestStep{
			Config: runAs(`database_name = sqlserver_database.runas.name
                           database_user_name = "dbo"`),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.s", "database_name", "tf_test_step_runas_db"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.s", "database_user_name", "dbo"),
			),
		},
		resource.TestStep{
			// Removing both from config clears the user and moves the step back to master.
			Config: runAs(""),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.s", "database_name", "master"),
				resource.TestCheckResourceAttr("sqlserver_agent_job_step.s", "database_user_name", ""),
				func(state *terraform.State) error {
					s := testAccAgentJobSteps(t)(state)[0]
					if s.DatabaseName != "master" || s.DatabaseUserName != "" {
						return fmt.Errorf("msdb has database %q and user %q", s.DatabaseName, s.DatabaseUserName)
					}
					return nil
				},
			),
		},
	)
}

func TestAccAgentJobStep_Local_RunAsNonSysadmin(t *testing.T) {
	const login, password = "tf_test_step_owner", "Owner!Passw0rd1"
	connector, err := getTestConnector(nil)
	if err != nil {
		t.Fatal(err)
	}
	setupSQL := `IF SUSER_ID(@login) IS NULL
                   EXEC ('CREATE LOGIN ' + @quoted + ' WITH PASSWORD = ''` + password + `'', CHECK_POLICY = OFF')
                 IF NOT EXISTS (SELECT 1 FROM [msdb].[sys].[database_principals] WHERE [name] = @login)
                   EXEC ('USE [msdb]; CREATE USER ' + @quoted + ' FOR LOGIN ' + @quoted + '; ALTER ROLE [SQLAgentUserRole] ADD MEMBER ' + @quoted + '; ALTER ROLE [db_datareader] ADD MEMBER ' + @quoted)`
	if runLocalAccTests {
		if err = connector.ExecContext(setupSQL, sql.Named("login", login), sql.Named("quoted", "["+login+"]")); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = connector.ExecContext(`USE [msdb]; IF USER_ID(@login) IS NOT NULL EXEC ('DROP USER ' + @quoted);
                                       IF SUSER_ID(@login) IS NOT NULL EXEC ('DROP LOGIN ' + @quoted)`,
				sql.Named("login", login), sql.Named("quoted", "["+login+"]"))
		}()
	}

	ownedConfig := func(extra string) string {
		return strings.Replace(testAccAgentJobStepsConfig("tf_test_step_owned", testStep{"s", "", extra}),
			`name = "tf_test_step_owned"`, fmt.Sprintf(`name = "tf_test_step_owned"
        owner_login_name = %q`, login), 1)
	}
	nonSysadminProvider := fmt.Sprintf(`provider "sqlserver" {
                                          login {
                                            username = %q
                                            password = %q
                                          }
                                        }`, login, password)

	testAccAgentJobStepCase(t,
		resource.TestStep{
			// The job owner isn't sysadmin, so SQL Server would silently ignore database_user_name.
			Config:      ownedConfig(`database_user_name = "dbo"`),
			ExpectError: regexp.MustCompile(`owned by "tf_test_step_owner", which isn't sysadmin`),
		},
	)
	testAccAgentJobStepCase(t,
		resource.TestStep{
			// The provider connects as a login that isn't sysadmin.
			Config: strings.Replace(testAccAgentJobStepsConfig("tf_test_step_caller", testStep{"s", "", `database_user_name = "dbo"`}),
				testAccProviderBlock, nonSysadminProvider, 1),
			ExpectError: regexp.MustCompile(`the login the provider connects with isn't sysadmin`),
		},
	)
}
