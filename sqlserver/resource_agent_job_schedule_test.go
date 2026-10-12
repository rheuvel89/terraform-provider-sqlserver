package sqlserver

import (
	"database/sql"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAgentJobSchedule_Local_Basic(t *testing.T) {
	schedule := `
      resource "sqlserver_agent_schedule" "nightly" {
        name              = "tf_test_shared_nightly"
        frequency_type    = "daily"
        active_start_time = "02:30:00"
      }`
	jobs := `
      resource "sqlserver_agent_job" "etl" {
        name = "tf_test_attach_etl"
      }
      resource "sqlserver_agent_job" "stats" {
        name = "tf_test_attach_stats"
      }
      resource "sqlserver_agent_job_schedule" "etl" {
        job_id      = sqlserver_agent_job.etl.job_id
        schedule_id = sqlserver_agent_schedule.nightly.schedule_id
      }
      resource "sqlserver_agent_job_schedule" "stats" {
        job_id      = sqlserver_agent_job.stats.job_id
        schedule_id = sqlserver_agent_schedule.nightly.schedule_id
      }`
	unmanaged := `
      resource "sqlserver_agent_job" "unmanaged" {
        name = "tf_test_attach_unmanaged"
      }`

	var scheduleID, unmanagedJobID string

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy: func(state *terraform.State) error {
			if err := testAccCheckAgentJobDestroy(state); err != nil {
				return err
			}
			return testAccCheckAgentScheduleDestroy(state)
		},
		Steps: []resource.TestStep{
			{
				// One schedule shared by two jobs.
				Config: testAccProviderBlock + schedule + jobs + unmanaged,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAgentJobScheduleExists("sqlserver_agent_job_schedule.etl"),
					testAccCheckAgentJobScheduleExists("sqlserver_agent_job_schedule.stats"),
					func(state *terraform.State) error {
						scheduleID = state.RootModule().Resources["sqlserver_agent_schedule.nightly"].Primary.Attributes["schedule_id"]
						unmanagedJobID = state.RootModule().Resources["sqlserver_agent_job.unmanaged"].Primary.Attributes["job_id"]
						return nil
					},
				),
			},
			{
				// Detaching and deleting the jobs must leave the schedule in place. The schedule is also attached to the
				// third job outside Terraform, so deleting that job covers sp_delete_job's @delete_unused_schedule = 0.
				PreConfig: func() {
					connector, err := getTestConnector(nil)
					if err != nil {
						t.Fatal(err)
					}
					id, _ := strconv.Atoi(scheduleID)
					if err = connector.ExecContext(`EXEC [msdb].[dbo].[sp_attach_schedule] @job_id = @jobId, @schedule_id = @scheduleId`,
						sql.Named("jobId", unmanagedJobID), sql.Named("scheduleId", id)); err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccProviderBlock + schedule,
				Check: func(state *terraform.State) error {
					connector, err := getTestConnector(nil)
					if err != nil {
						return err
					}
					id, _ := strconv.Atoi(scheduleID)
					s, err := connector.GetAgentSchedule(id)
					if err != nil {
						return err
					}
					if s == nil {
						return fmt.Errorf("agent schedule [%d] was deleted together with its jobs", id)
					}
					return nil
				},
			},
		},
	})
}

func testAccCheckAgentJobScheduleExists(resourceName string) resource.TestCheckFunc {
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
		attached, err := connector.GetAgentJobSchedule(rs.Primary.Attributes["job_id"], id)
		if err != nil {
			return err
		}
		if !attached {
			return fmt.Errorf("%s: schedule [%d] is not attached to job [%s]", resourceName, id, rs.Primary.Attributes["job_id"])
		}
		return nil
	}
}
