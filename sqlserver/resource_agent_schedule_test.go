package sqlserver

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"terraform-provider-sqlserver/sqlserver/model"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const testAccProviderBlock = `provider "sqlserver" {
                                login {}
                              }
`

// TestAccAgentSchedule_Local_Examples applies every schedule in examples/agent_schedule/schedules.tf, which the
// documentation is built from, and checks that the plan is empty afterwards and that msdb holds the expected values.
func TestAccAgentSchedule_Local_Examples(t *testing.T) {
	examples, err := os.ReadFile("../examples/agent_schedule/schedules.tf")
	if err != nil {
		t.Fatal(err)
	}

	raw := func(name string, want model.AgentSchedule) resource.TestCheckFunc {
		return testAccCheckAgentScheduleRaw("sqlserver_agent_schedule."+name, want)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckAgentScheduleDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccProviderBlock + string(examples),
				Check: resource.ComposeTestCheckFunc(
					raw("nightly_0230", model.AgentSchedule{FreqType: 4, FreqInterval: 1, FreqSubdayType: 1, ActiveStartTime: 23000, ActiveEndTime: 235959}),
					raw("every_3_days", model.AgentSchedule{FreqType: 4, FreqInterval: 3, FreqSubdayType: 1, ActiveStartDate: 20261101}),
					raw("black_friday_watch", model.AgentSchedule{FreqType: 4, FreqInterval: 1, FreqSubdayType: 4, FreqSubdayInterval: 5, ActiveStartDate: 20261126, ActiveEndDate: 20261130}),
					raw("weekly_full_backup", model.AgentSchedule{FreqType: 8, FreqInterval: 1, FreqSubdayType: 1, FreqRecurrenceFactor: 1}),
					raw("office_hours_15m", model.AgentSchedule{FreqType: 8, FreqInterval: 62, FreqSubdayType: 4, FreqSubdayInterval: 15, FreqRecurrenceFactor: 1, ActiveStartTime: 80000, ActiveEndTime: 180000}),
					raw("biweekly_payroll", model.AgentSchedule{FreqType: 8, FreqInterval: 32, FreqSubdayType: 1, FreqRecurrenceFactor: 2}),
					raw("quarterly", model.AgentSchedule{FreqType: 16, FreqInterval: 1, FreqSubdayType: 1, FreqRecurrenceFactor: 3, ActiveStartDate: 20270101}),
					raw("first_sunday", model.AgentSchedule{FreqType: 32, FreqInterval: 1, FreqRelativeInterval: 1, FreqSubdayType: 1, FreqRecurrenceFactor: 1}),
					raw("month_end_close", model.AgentSchedule{FreqType: 32, FreqInterval: 9, FreqRelativeInterval: 16, FreqSubdayType: 1, FreqRecurrenceFactor: 1}),
					raw("first_weekend_day_6h", model.AgentSchedule{FreqType: 32, FreqInterval: 10, FreqRelativeInterval: 1, FreqSubdayType: 8, FreqSubdayInterval: 6, FreqRecurrenceFactor: 1}),
					raw("cutover", model.AgentSchedule{FreqType: 1, ActiveStartDate: 20261205, ActiveStartTime: 220000}),
					raw("on_agent_start", model.AgentSchedule{FreqType: 64}),
					raw("when_idle", model.AgentSchedule{FreqType: 128}),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.office_hours_15m", "weekly_days.#", "5"),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.nightly_0230", "subday_type", "once"),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.nightly_0230", "active_end_date", "9999-12-31"),
					resource.TestCheckResourceAttrSet("sqlserver_agent_schedule.nightly_0230", "active_start_date"),
					resource.TestCheckResourceAttrSet("sqlserver_agent_schedule.nightly_0230", "schedule_uid"),
					resource.TestCheckResourceAttrSet("sqlserver_agent_schedule.nightly_0230", "owner_login_name"),
				),
			},
		},
	})
}

func TestAccAgentSchedule_Local_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      func(state *terraform.State) error { return testAccCheckAgentScheduleDestroy(state) },
		Steps: []resource.TestStep{
			{
				Config: testAccAgentSchedule(`
                  name              = "tf_test_schedule"
                  frequency_type    = "weekly"
                  weekly_days       = ["monday", "friday"]
                  subday_type       = "hours"
                  subday_interval   = 2
                  active_start_date = "2027-01-04"`),
				Check: testAccCheckAgentScheduleRaw("sqlserver_agent_schedule.test", model.AgentSchedule{FreqType: 8, FreqInterval: 34, FreqSubdayType: 8, FreqSubdayInterval: 2, FreqRecurrenceFactor: 1, ActiveStartDate: 20270104}),
			},
			{
				// Switch to a different frequency type: the weekly fields must be cleared, and the schedule keeps its ID.
				Config: testAccAgentSchedule(`
                  name                  = "tf_test_schedule_renamed"
                  enabled               = false
                  frequency_type        = "monthly_relative"
                  monthly_relative_week = "last"
                  monthly_relative_day  = "day"
                  active_start_date     = "2027-01-04"
                  active_start_time     = "20:00:00"`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentScheduleRaw("sqlserver_agent_schedule.test", model.AgentSchedule{FreqType: 32, FreqInterval: 8, FreqRelativeInterval: 16, FreqSubdayType: 1, FreqRecurrenceFactor: 1, ActiveStartTime: 200000}),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.test", "name", "tf_test_schedule_renamed"),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.test", "enabled", "false"),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.test", "weekly_days.#", "0"),
					resource.TestCheckResourceAttr("sqlserver_agent_schedule.test", "subday_type", "once"),
				),
			},
		},
	})
}

func TestAccAgentSchedule_Local_PlanErrors(t *testing.T) {
	cases := map[string]string{
		"at least 10":                      `frequency_type = "daily"` + "\n" + `subday_type = "seconds"` + "\n" + `subday_interval = 5`,
		`"2026-02-30" is not a valid date`: `frequency_type = "once"` + "\n" + `active_start_date = "2026-02-30"`,
		"is before active_start_date":      `frequency_type = "daily"` + "\n" + `active_start_date = "2027-02-01"` + "\n" + `active_end_date = "2027-01-01"`,
		`use "23:59:59"`:                   `frequency_type = "daily"` + "\n" + `active_end_time = "00:00:00"`,
		"weekly_days is required":          `frequency_type = "weekly"`,
	}
	var steps []resource.TestStep
	for want, body := range cases {
		steps = append(steps, resource.TestStep{
			Config:      testAccAgentSchedule(`name = "tf_test_schedule_invalid"` + "\n" + body),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(regexp.QuoteMeta(want)),
		})
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		Steps:             steps,
	})
}

func testAccAgentSchedule(body string) string {
	return testAccProviderBlock + fmt.Sprintf(`
            resource "sqlserver_agent_schedule" "test" {
              %s
            }`, body)
}

// testAccCheckAgentScheduleRaw compares the msdb values of a schedule with want. Zero dates and times in want
// aren't compared, since they depend on the day the test runs or are covered by other cases.
func testAccCheckAgentScheduleRaw(resourceName string, want model.AgentSchedule) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}
		id, _ := strconv.Atoi(rs.Primary.Attributes["schedule_id"])
		got, err := connector.GetAgentSchedule(id)
		if err != nil {
			return err
		}
		if got == nil {
			return fmt.Errorf("agent schedule [%d] does not exist", id)
		}
		if want.ActiveStartDate == 0 {
			want.ActiveStartDate = got.ActiveStartDate
		}
		if want.ActiveEndDate == 0 {
			want.ActiveEndDate = got.ActiveEndDate
		}
		if want.ActiveStartTime == 0 {
			want.ActiveStartTime = got.ActiveStartTime
		}
		if want.ActiveEndTime == 0 {
			want.ActiveEndTime = got.ActiveEndTime
		}
		want.ScheduleID, want.ScheduleUID, want.Name, want.Enabled, want.OwnerLoginName = got.ScheduleID, got.ScheduleUID, got.Name, got.Enabled, got.OwnerLoginName
		if *got != want {
			return fmt.Errorf("%s: msdb holds\n %+v\nexpected\n %+v", resourceName, *got, want)
		}
		return nil
	}
}

func testAccCheckAgentScheduleDestroy(state *terraform.State) error {
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "sqlserver_agent_schedule" {
			continue
		}

		connector, err := getTestConnector(rs.Primary.Attributes)
		if err != nil {
			return err
		}

		id, _ := strconv.Atoi(rs.Primary.Attributes["schedule_id"])
		s, err := connector.GetAgentSchedule(id)
		if s != nil {
			return fmt.Errorf("agent schedule [%s] still exists", s.Name)
		}
		if err != nil {
			return fmt.Errorf("expected no error, got %s", err)
		}
	}
	return nil
}
