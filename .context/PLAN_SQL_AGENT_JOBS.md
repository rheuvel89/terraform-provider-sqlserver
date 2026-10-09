# Plan: SQL Server Agent resources for `terraform-provider-sqlserver`

Goal: add create/read/update/delete support for SQL Server Agent jobs, job steps and schedules, and attaching schedules to jobs
(see https://learn.microsoft.com/en-us/ssms/agent/create-a-job).

## 1. Decisions

| Topic | Decision |
|---|---|
| Resource model | 4 separate resources: `sqlserver_agent_job`, `sqlserver_agent_job_step`, `sqlserver_agent_schedule`, `sqlserver_agent_job_schedule` (attaches a schedule to a job). A fifth, `sqlserver_agent_operator`, is added for notifications. |
| Step identity | Identified by job + step **name** (ForceNew). `step_id` is an Optional+Computed position. |
| Step order | A per-job lock in the provider; a `step_id` greater than count + 1 is clamped to append with a warning; the docs require each step to `depends_on` the previous one. Drift is fixed on the next apply. |
| Go-to step | `on_success_step_name` / `on_fail_step_name`, resolved to an id at apply time and mapped back to a name on read. |
| Start step | `is_start_step` flag on the step resource; read-only `start_step_id` on the job. |
| Subsystems | **TSQL only**. `subsystem` is not exposed and is always `TSQL`. |
| Schedule syntax | Friendly attributes, translated to and from the msdb integers. |
| Notifications | Email to an operator, event log, and auto-delete on the job. Pager and net send are left out because they're deprecated. |
| Categories | `category_name` referencing an existing category only. |
| Platforms | On-prem/VM and Azure SQL Managed Instance. Azure SQL Database fails fast with a clear error. |
| Import | Not supported. |

## 2. Resource schemas

### `sqlserver_agent_job`
ID: `sqlserver://host:port/agent_job/<job_id GUID>`

| Attribute | Kind | Notes |
|---|---|---|
| `name` | Required | Updated in place via `sp_update_job @new_name` (the ID is the GUID, so renames don't recreate the job) |
| `description` | Optional | default `""` |
| `enabled` | Optional | default `true` |
| `category_name` | Optional+Computed | server default `[Uncategorized (Local)]` |
| `owner_login_name` | Optional+Computed | defaults to the connecting login |
| `notify_level_eventlog` | Optional | `never \| on_success \| on_failure \| always`, default `on_failure` (SQL default 2) |
| `notify_level_email` | Optional | same enum, default `never` |
| `notify_email_operator_name` | Optional | required when `notify_level_email != never` (CustomizeDiff) |
| `delete_level` | Optional | same enum, default `never` |
| `job_id` | Computed | GUID |
| `start_step_id` | Computed | |

- **Create:** `sp_add_job … @job_id OUTPUT`, then always `sp_add_jobserver @server_name = N'(local)'` (a job with no target server never runs).
- **Update:** `sp_update_job`.
- **Delete:** `sp_delete_job @job_id, @delete_unused_schedule = 0`. This is critical; otherwise SQL Server also deletes schedules that `sqlserver_agent_schedule` manages.

### `sqlserver_agent_job_step`
ID: `sqlserver://host:port/agent_job/<job_id>/step/<url-escaped name>`

| Attribute | Kind | Notes |
|---|---|---|
| `job_id` | Required, ForceNew | |
| `name` | Required, ForceNew | |
| `step_id` | Optional+Computed | desired position |
| `command` | Required | T-SQL |
| `database_name` | Optional+Computed | default `master` |
| `database_user_name` | Optional | runs as that user (needs sysadmin) |
| `on_success_action` | Optional | `quit_with_success \| quit_with_failure \| go_to_next_step \| go_to_step`, default `go_to_next_step` (as in SSMS) |
| `on_success_step_name` | Optional | required iff action = `go_to_step` |
| `on_fail_action` | Optional | default `quit_with_failure` |
| `on_fail_step_name` | Optional | required iff action = `go_to_step` |
| `retry_attempts` / `retry_interval` | Optional | default 0 (interval in minutes) |
| `output_file_name` | Optional | documented as not supported on MI |
| `append_output_file`, `log_to_table`, `append_log_to_table`, `include_step_output_in_history` | Optional bool | mapped to the `flags` bitmask (2 / 8 / 16 / 32 / 4) |
| `is_start_step` | Optional bool | default false |
| `step_uid` | Computed | |

- **Create:** take the per-job lock. Resolve the go-to names to ids, and fail with a hint to add `depends_on` if a target is missing. Use `sp_add_jobstep` with `step_id = min(requested, count+1)`. If `is_start_step` is set, also call `sp_update_job @start_step_id`.
- **Read:** look up the step by name in `msdb.dbo.sysjobsteps` and map the go-to ids back to names. `is_start_step` = `sysjobs.start_step_id == step_id`. If the job or step is gone, clear the ID.
- **Update:** take the lock. Use `sp_update_jobstep`. If `step_id` changed, move the step by running `sp_delete_jobstep` and then `sp_add_jobstep` at the new position with the same settings (the `step_uid` changes, which is fine).
- **Delete:** take the lock, then `sp_delete_jobstep`.
- **Documented limits:** a cycle where step A goes to B and B goes to A can't be expressed in Terraform. Only one step per job should set `is_start_step`.

### `sqlserver_agent_schedule`
ID: `sqlserver://host:port/agent_schedule/<schedule_id>`. msdb doesn't require schedule names to be unique, so the ID uses the integer.

| Attribute | Notes |
|---|---|
| `name` | Required; updated in place via `@new_name` |
| `enabled` | default `true` |
| `frequency_type` | `once \| daily \| weekly \| monthly \| monthly_relative \| agent_start \| idle` → 1/4/8/16/32/64/128 |
| `recurrence_factor` | "every N days/weeks/months". For `daily` it maps to `freq_interval`, otherwise to `freq_recurrence_factor` |
| `weekly_days` | set of `sunday…saturday` → bitmask 1, 2, 4 … 64 |
| `monthly_day` | 1–31 |
| `monthly_relative_week` | `first \| second \| third \| fourth \| last` → 1/2/4/8/16 |
| `monthly_relative_day` | `sunday…saturday \| day \| weekday \| weekend_day` → 1–10 |
| `subday_type` / `subday_interval` | `once \| seconds \| minutes \| hours` → 1/2/4/8 |
| `active_start_date` / `active_end_date` | `YYYY-MM-DD` ↔ `yyyymmdd` int; Optional+Computed (defaults are today and `9999-12-31`) |
| `active_start_time` / `active_end_time` | `HH:MM:SS` ↔ `hhmmss` int; defaults `00:00:00` / `23:59:59` |
| `owner_login_name` | Optional+Computed |
| `schedule_id`, `schedule_uid` | Computed |

- A **CustomizeDiff** checks that only the fields relevant to the chosen `frequency_type` are set (for example, `weekly_days` is required for `weekly` and forbidden otherwise).
- **CRUD:** `sp_add_schedule` / `sp_update_schedule` / `sp_delete_schedule @force_delete = 0`. Terraform destroys the attachments first, so `force_delete` isn't needed.

### `sqlserver_agent_job_schedule` (attachment)
ID: `sqlserver://host:port/agent_job/<job_id>/agent_schedule/<schedule_id>`

- `job_id` and `schedule_id` are both Required and ForceNew; there's no Update.
- **Create:** `sp_attach_schedule`.
- **Read:** `msdb.dbo.sysjobschedules`.
- **Delete:** `sp_detach_schedule @delete_unused_schedule = 0`.
- **Documented requirement:** the job owner must also own the schedule unless the caller is sysadmin.

### `sqlserver_agent_operator`
ID: `sqlserver://host:port/agent_operator/<id>`

- **Attributes:** `name` (updated in place via `@new_name`), `enabled` (default true), `email_address`, `category_name` (Optional+Computed), and computed `operator_id`.
- **CRUD:** `sp_add_operator` / `sp_update_operator` / `sp_delete_operator`.
- **Ordering:** Database Mail isn't needed to create an operator. A job that references `sqlserver_agent_operator.x.name` creates the right dependency ordering.

## 3. Code layout (following the existing layering)

**New files**
- `sqlserver/model/agent_job.go`, `agent_job_step.go`, `agent_schedule.go`, `agent_operator.go`: structs in raw msdb form (ints, bitmasks).
- `sql/agent_job.go`, `sql/agent_job_step.go`, `sql/agent_schedule.go`, `sql/agent_job_schedule.go`, `sql/agent_operator.go`: `*Connector` methods that call `msdb.dbo.sp_*` with `sql.Named` parameters, matching the style in `sql/database.go`.
  - GUIDs are always selected as `CONVERT(nvarchar(36), job_id)`. This sidesteps go-mssqldb's mixed-endian `UniqueIdentifier` byte order.
  - The output id comes from a `DECLARE @id …; EXEC … OUTPUT; SELECT @id` batch run through `QueryRowContext`.
- `sql/agent_common.go`: `EnsureAgentAvailable(ctx)`, which checks `SERVERPROPERTY('EngineEdition')` and returns a clear error for 5 (Azure SQL Database). Every agent Create calls it.
- `sqlserver/agent_schedule_encoding.go`: pure functions for the friendly ↔ msdb translation (days bitmask, relative week and day, times, dates), shared by Create, Read and Update.
- `sqlserver/agent_lock.go`: a package-level `sync.Map` of `*sync.Mutex` keyed by `job_id`. `sqlserverProvider` is passed around by value, so the lock can't be a field on it.
- `sqlserver/resource_agent_job.go`, `resource_agent_job_step.go`, `resource_agent_schedule.go`, `resource_agent_job_schedule.go`, `resource_agent_operator.go`: each with its own `XxxConnector` interface, schema, CRUD, `getXxxID` and `getXxxConnector`, following the pattern in `resource_database.go`.

**Changed files**
- `sqlserver/const.go`: property-name constants and the enum maps.
- `sqlserver/provider.go`: register the 5 resources in `ResourcesMap`.
- `sqlserver/provider_test.go`: extend `TestConnector` with `GetAgentJob`, `GetAgentJobStep`, `GetAgentSchedule`, `GetAgentJobSchedule` and `GetAgentOperator`.
- `docker-compose/docker-compose.yml` and `test-fixtures/local/main.tf`: add `MSSQL_AGENT_ENABLED=true`.
- `README.md`, plus a version bump in `Makefile` (0.4.0 → 0.5.0).

## 4. Tests
- **Unit tests** (table-driven, like `sql/resource_governor_test.go`): the schedule encoding round-trips (every frequency type, bitmask edges, `last`/`weekend_day`, the time and date formats) and the step `flags` mapping.
- **Acceptance tests** (`TestAccAgent*_Local_*`, using `templateToString`, Exists and Destroy checks):
  1. Job: basic, then update (rename, enabled, description, category), then destroy.
  2. Steps: three chained steps; reorder by changing `step_id`; delete the middle step and assert the plan converges; go-to by name; `is_start_step`.
  3. Schedule: one case per `frequency_type`, checking the friendly values read back unchanged and that the plan is empty afterwards.
  4. Attachment: attach and detach. Assert the schedule **still exists** after the job is destroyed (this covers the `@delete_unused_schedule = 0` fix).
  5. Operator plus a job with `notify_level_email = on_failure`.
- `TestProvider` (`InternalValidate`) covers all the new schemas automatically.

## 5. Docs and examples
- `docs/resources/agent_job.md`, `agent_job_step.md`, `agent_schedule.md`, `agent_job_schedule.md`, `agent_operator.md`, in the same style as `database.md` but without an Import section.
- `examples/agent_job/main.tf`: an end-to-end example with an operator, a job, 3 chained steps, a shared schedule attached to two jobs, and email on failure.
- The docs cover the required permissions (`SQLAgentOperatorRole` / sysadmin for owner changes and `database_user_name`), MI limitations, the step-ordering and `depends_on` rule, and the single-start-step rule.

## 6. Implementation order
1. **Spike:** run the Docker image with the agent enabled and confirm three behaviours by hand: whether `sp_delete_jobstep` / `sp_add_jobstep` shift other steps' `on_*_step_id` values and the job's `start_step_id`, and the error `sp_add_jobstep` returns when `step_id > count+1`. The step design depends on these, so if they behave differently, step Read/Update gets adjusted before anything else is built.
2. Docker fixtures and `EnsureAgentAvailable`.
3. Operator, the simplest resource, to set up the end-to-end pattern.
4. Job.
5. Schedule, with the encoding functions and their unit tests.
6. Job–schedule attachment.
7. Job step, with the lock, clamping, go-to resolution and start step.
8. Docs, example, README and version bump.

## 7. Defaults chosen without asking
- No data sources.
- Multi-server (MSX/TSX) targets are out of scope; jobs always target `(local)`.
- Operator pager and net-send fields are left out because they're deprecated.
- `on_success_action` defaults to `go_to_next_step`, matching SSMS rather than `sp_add_jobstep`'s `quit_with_success`.
