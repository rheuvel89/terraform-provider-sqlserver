# sqlserver_agent_job_schedule

The `sqlserver_agent_job_schedule` resource attaches a [`sqlserver_agent_schedule`](agent_schedule.md) to a
[`sqlserver_agent_job`](agent_job.md). One schedule can be attached to several jobs, and a job can have several
schedules.

## Example Usage

```hcl
resource "sqlserver_agent_schedule" "nightly" {
  name              = "Daily 02:30"
  frequency_type    = "daily"
  active_start_time = "02:30:00"
}

resource "sqlserver_agent_job_schedule" "etl_nightly" {
  job_id      = sqlserver_agent_job.nightly_etl.job_id
  schedule_id = sqlserver_agent_schedule.nightly.schedule_id
}

resource "sqlserver_agent_job_schedule" "stats_nightly" {
  job_id      = sqlserver_agent_job.update_stats.job_id
  schedule_id = sqlserver_agent_schedule.nightly.schedule_id
}
```

## Argument Reference

* `job_id` - (Required, ForceNew) The `job_id` of the job.
* `schedule_id` - (Required, ForceNew) The `schedule_id` of the schedule.

## Notes

* Removing the attachment detaches the schedule but never deletes it, even when no job uses it any more.
* Unless the provider's login is sysadmin, the job's owner must also own the schedule.

## Import

Import isn't supported.
