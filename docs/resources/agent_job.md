# sqlserver_agent_job

The `sqlserver_agent_job` resource creates and manages a SQL Server Agent job. A job does nothing on its own: add
steps with [`sqlserver_agent_job_step`](agent_job_step.md) and attach schedules with
[`sqlserver_agent_job_schedule`](agent_job_schedule.md).

SQL Server Agent runs on SQL Server (on-premises or in a VM) and Azure SQL Managed Instance. It isn't available on
Azure SQL Database; creating a job there fails with a clear error.

## Example Usage

### Basic Job

```hcl
resource "sqlserver_agent_job" "nightly_etl" {
  name        = "nightly_etl"
  description = "Loads the warehouse every night"
}
```

### Email an Operator on Failure

```hcl
resource "sqlserver_agent_operator" "dba" {
  name          = "dba"
  email_address = "dba@example.com"
}

resource "sqlserver_agent_job" "nightly_etl" {
  name                       = "nightly_etl"
  notify_level_email         = "on_failure"
  notify_email_operator_name = sqlserver_agent_operator.dba.name
}
```

Mail is sent by Database Mail, which has to be set up separately; see
[`sqlserver_agent_operator`](agent_operator.md#email-delivery).

## Argument Reference

* `name` - (Required) The name of the job. Changing it renames the job in place.
* `description` - (Optional) A description of the job. Default is empty.
* `enabled` - (Optional) Whether the job runs on its schedules. Default is `true`.
* `category_name` - (Optional) The name of an existing job category. Default is `[Uncategorized (Local)]`; removing
  the argument moves the job back to that category. Categories only group jobs in SSMS; the replication (`REPL-*`) and
  `Jobs from MSX` categories are used by SQL Server itself and shouldn't be used for your own jobs.
* `owner_login_name` - (Optional) The login that owns the job. Defaults to the login the provider connects with.
  Changing the owner needs sysadmin. The owner decides who the steps run as: see
  [`sqlserver_agent_job_step`](agent_job_step.md#who-a-step-runs-as).
* `notify_level_eventlog` - (Optional) When to write the job outcome to the Windows application log: `never`,
  `on_success`, `on_failure` or `always`. Default is `on_failure`. Azure SQL Managed Instance has no Windows
  application log and always stores `never`, so set `notify_level_eventlog = "never"` there to avoid a permanent
  diff.
* `notify_level_email` - (Optional) When to email `notify_email_operator_name`: `never`, `on_success`, `on_failure`
  or `always`. Default is `never`.
* `notify_email_operator_name` - (Optional) The name of the operator to email. Required when `notify_level_email` isn't
  `never`.
* `delete_level` - (Optional) When to delete the job after it runs: `never`, `on_success`, `on_failure` or `always`.
  Default is `never`.

## Attribute Reference

* `job_id` - The job's GUID. Steps and schedule attachments refer to the job by this ID.
* `start_step_id` - The step the job starts at. The provider always keeps the first step of the chain at position 1,
  which is where SQL Server starts a job.

## Notes

* The job is always targeted at the local server; a job without a target server never runs.
* Deleting the job deletes its steps but never its schedules, which are managed by
  [`sqlserver_agent_schedule`](agent_schedule.md).

## Permissions

The provider reads the msdb tables directly, so its login needs either sysadmin, or `SQLAgentUserRole` plus
`db_datareader` in msdb. With `SQLAgentUserRole` a login can only manage the jobs it owns.

## Import

Import isn't supported.
