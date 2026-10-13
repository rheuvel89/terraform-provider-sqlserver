# sqlserver_agent_job_step

The `sqlserver_agent_job_step` resource creates and manages a T-SQL step of a
[`sqlserver_agent_job`](agent_job.md).

## How Steps Are Ordered

A job's steps form a chain, and each step says which step comes before it:

* the **first** step sets `job_id`;
* **every other** step sets `previous_step_id` to the `id` of the step before it.

Terraform works out the order from these references, so no `depends_on` is needed and the provider never asks you for
a step number. To insert a step, point it at the step before it and point the next step at the new one. To move a step,
change its `previous_step_id`. Each change converges in one apply.

SQL Server jobs can't fork: two steps can't follow the same step, and only one step can set `job_id`. The provider
fails the **plan** when that happens, for example:

```
steps "transform" and "X" both follow "extract" in job ...; SQL Server jobs can't fork
```

When the steps are new and their IDs aren't known yet, the same check fails during apply instead.

## Example Usage

```hcl
resource "sqlserver_agent_job" "etl" {
  name = "nightly_etl"
}

resource "sqlserver_agent_job_step" "extract" {
  job_id  = sqlserver_agent_job.etl.job_id # first step
  name    = "extract"
  command = "EXEC dbo.extract"
}

resource "sqlserver_agent_job_step" "transform" {
  previous_step_id = sqlserver_agent_job_step.extract.id # every later step
  name             = "transform"
  database_name    = "warehouse"
  command          = <<-SQL
    EXEC dbo.transform;
    EXEC dbo.update_statistics;
  SQL
  retry_attempts   = 3
  retry_interval   = 5
}

resource "sqlserver_agent_job_step" "load" {
  previous_step_id  = sqlserver_agent_job_step.transform.id
  name              = "load"
  command           = "EXEC dbo.load"
  on_fail_action    = "go_to_step" # backward jump: declared on the step that jumps
  on_fail_step_name = sqlserver_agent_job_step.extract.name
}

resource "sqlserver_agent_job_step" "notify" {
  previous_step_id   = sqlserver_agent_job_step.load.id
  name               = "notify"
  command            = "EXEC dbo.notify_failure"
  handles_failure_of = [sqlserver_agent_job_step.extract.name] # forward jump: declared on the step jumped to
}
```

## Jumps

By default a step goes to the next step when it succeeds and stops the job, reporting failure, when it fails. To go to
a specific step instead, declare the jump on **whichever of the two steps is later in the chain**, so references
always point backwards:

* **Backward** (to an earlier step): set `on_success_action` / `on_fail_action` to `go_to_step` and
  `on_success_step_name` / `on_fail_step_name` to the earlier step's name, on the step that jumps.
* **Forward** (to a later step): list the earlier steps in `handles_success_of` / `handles_failure_of` on the step
  they jump to. Leave the earlier steps' own actions at their defaults; the provider sets them.

SQL Server stores jumps as step numbers. The provider keeps them pointing at the right step when steps are inserted,
deleted or moved (`sp_delete_jobstep` on its own loses jumps to the step after the deleted one). When the target of a
jump is deleted, the jumping step falls back to its default action. A step can't jump to itself.

## Argument Reference

* `job_id` - (Optional) The `job_id` of the job. Set it on the first step only. Exactly one of `job_id` and
  `previous_step_id` is required. A step can't move to another job; see [Limits](#limits).
* `previous_step_id` - (Optional) The `id` of the step that runs before this one. Changing it moves the step.
* `name` - (Required, ForceNew) The name of the step, unique within the job.
* `command` - (Required) The T-SQL the step runs. Leading and trailing whitespace is ignored when comparing.
* `database_name` - (Optional) The database the step runs in. Default is `master`; removing the argument moves the
  step back to `master`. Reference a `sqlserver_database` resource's `name` so that the step depends on the database.
* `database_user_name` - (Optional) The database user the step runs as. See [Who a Step Runs As](#who-a-step-runs-as).
* `on_success_action` - (Optional) What to do when the step succeeds: `quit_with_success`, `quit_with_failure`,
  `go_to_next_step` or `go_to_step`. Default is `go_to_next_step`.
* `on_success_step_name` - (Optional) The earlier step to go to on success. Required when `on_success_action` is
  `go_to_step`.
* `on_fail_action` - (Optional) What to do when the step fails, with the same values. Default is `quit_with_failure`.
* `on_fail_step_name` - (Optional) The earlier step to go to on failure. Required when `on_fail_action` is
  `go_to_step`.
* `handles_success_of` - (Optional) Names of earlier steps that go to this step when they succeed.
* `handles_failure_of` - (Optional) Names of earlier steps that go to this step when they fail.
* `retry_attempts` - (Optional) How many times to retry the step when it fails. Default is `0`.
* `retry_interval` - (Optional) Minutes to wait between retries. Default is `0`.
* `output_file_name` - (Optional) A file on the server to write the step output to. Not supported on Azure SQL
  Managed Instance.
* `append_output_file` - (Optional) Append to `output_file_name` instead of overwriting it. Default is `false`.
* `log_to_table` - (Optional) Write the step output to `msdb.dbo.sysjobstepslogs`, overwriting the previous entry.
  Default is `false`. Unlike the job history, this keeps the full output.
* `append_log_to_table` - (Optional) Write the step output to `msdb.dbo.sysjobstepslogs`, appending to the previous
  entry. Default is `false`.
* `include_step_output_in_history` - (Optional) Include the step output in the job history. Default is `false`. The
  history keeps only about 4,000 characters per step; see [Where to Find a Step's Output](#where-to-find-a-steps-output).

## Attribute Reference

* `id` - The step's ID, used as `previous_step_id` by the next step.
* `step_id` - The step's position in the job as of the last refresh. When other steps are inserted or deleted,
  it's updated on the next plan or refresh; the provider never uses it to find the step.
* `step_uid` - The GUID of the step. It changes when the step moves.

## Who a Step Runs As

It's the job owner (`owner_login_name` on the job) that decides this, not the login the provider connects with:

| Job owner | `database_user_name` empty | `database_user_name` set |
|---|---|---|
| sysadmin | the SQL Server Agent service account, which has sysadmin rights | that database user, in `database_name` |
| not sysadmin | the owner | not allowed |

The owner defaults to the login the provider connects with, so with a sysadmin provider login a step without
`database_user_name` runs with full sysadmin rights. To limit that, set `database_user_name`, or make the job owner a
login that isn't sysadmin. A step that runs as a database user can't reach other databases.

SQL Server silently ignores `database_user_name` for jobs whose owner isn't sysadmin, and only a sysadmin can set it.
The provider checks both during apply and fails with a clear error instead. Changing the job's owner later isn't
checked again until the step itself changes.

## Where to Find a Step's Output

| Where | How to read it | Size | Notes |
|---|---|---|---|
| Job history | SSMS (View History), `EXEC msdb.dbo.sp_help_jobhistory @job_name = N'…', @mode = N'FULL'`, or `msdb.dbo.sysjobhistory` | about 4,000 characters per step (`nvarchar(4000)`) | Holds the step's messages and errors. When the output is longer, the history keeps roughly the **first** 4,000 characters, marks the cut with `...` and **drops the rest, including the error that failed the step**. |
| Table log | `EXEC msdb.dbo.sp_help_jobsteplog @job_name = N'…'`, or `msdb.dbo.sysjobstepslogs` | no limit (`nvarchar(max)`) | Needs `log_to_table` (overwritten on each run) or `append_log_to_table` (kept across runs; clear it with `msdb.dbo.sp_delete_jobsteplog`). Deleted together with the step, including when the provider moves the step (a move deletes the step and adds it again). |
| Output file | the file in `output_file_name` | no limit | On the server's file system; not supported on Azure SQL Managed Instance. |

For steps whose output can be long, such as a step that loops over many databases, set `log_to_table = true` and look
there first: the job history may have cut off exactly the part that says why the step failed.

## Limits

* Only T-SQL steps are supported.
* Steps can't move to another job. Pointing a first step's `job_id`, or a later step's `previous_step_id`, at another
  job fails with an error, usually during `terraform plan`. To move steps, use two applies: remove the step resources
  from the old job first, then add them to the new job. A plan can't tell a chain that moves to another job from a
  step that is inserted before it, because both make `previous_step_id` unknown until apply.
* Step changes of one job are serialised within one `terraform apply`. Changing the same job's steps from two applies
  at once, or in SSMS during an apply, isn't protected against.
* The fork check covers the steps planned together. It doesn't see steps of the same job managed from another
  configuration or provider alias, and a `-target` plan can miss a fork.

## Import

Import isn't supported.
