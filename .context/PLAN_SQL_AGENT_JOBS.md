# Plan: SQL Server Agent resources for `terraform-provider-sqlserver`

Goal: add create/read/update/delete support for SQL Server Agent jobs, job steps and schedules, and attaching schedules to jobs
(see https://learn.microsoft.com/en-us/ssms/agent/create-a-job).

## 1. Decisions

| Topic | Decision |
|---|---|
| Resource model | 4 separate resources: `sqlserver_agent_job`, `sqlserver_agent_job_step`, `sqlserver_agent_schedule`, `sqlserver_agent_job_schedule` (attaches a schedule to a job). A fifth, `sqlserver_agent_operator`, is added for notifications. |
| Step identity | Identified by job + step **name**. `step_id` is **Computed only**; users never write a position. |
| Step order | **Backward chain.** The first step sets `job_id`; every other step sets `previous_step_id` (the previous step's resource ID). Exactly one of the two is required. Order follows from the references, so no `depends_on` is needed. |
| Go-to jumps | Optional. **A jump is declared on whichever of the two steps is later in the chain**, so every reference points backwards and no cycles are possible. Backward jumps: `on_success_step_name` / `on_fail_step_name` on the source. Forward jumps: `handles_success_of` / `handles_failure_of` on the target. |
| Jump numbering | msdb stores jump targets as step numbers. The provider snapshots every jump as name → name before an insert, delete or move and rewrites any number that is wrong afterwards. |
| Chain validation | SQL Server jobs can't fork. A provider-level registry, filled by each step's `CustomizeDiff`, fails the **plan** when two steps claim the same previous step (or two steps claim to be first), and when two targets claim the same forward jump. |
| Concurrency | An in-process per-job lock (`map[jobGUID]*sync.Mutex`) around every step read-modify-write. It protects within one `terraform apply` only; cross-process locking (`sp_getapplock`) is out of scope. |
| Start step | Not managed. The head of the chain is step 1, which is SQL Server's default start step. The job exposes a read-only `start_step_id`. |
| Subsystems | **TSQL only**. `subsystem` is not exposed and is always `TSQL`. |
| Schedule syntax | Friendly attributes, translated to and from the msdb integers. |
| Notifications | Email to an operator, event log, and auto-delete on the job. Pager and net send are left out because they're deprecated. |
| Categories | `category_name` referencing an existing category only, defaulting to the built-in uncategorized category so that removing it from config reverts the server. |
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
| `category_name` | Optional | default `[Uncategorized (Local)]`; removing it from config moves the job back to that category |
| `owner_login_name` | Optional+Computed | defaults to the connecting login |
| `notify_level_eventlog` | Optional | `never \| on_success \| on_failure \| always`, default `on_failure` (SQL default 2) |
| `notify_level_email` | Optional | same enum, default `never` |
| `notify_email_operator_name` | Optional | required when `notify_level_email != never` (CustomizeDiff) |
| `delete_level` | Optional | same enum, default `never` |
| `job_id` | Computed | GUID |
| `start_step_id` | Computed | read-only; not managed by the provider |

- **Create:** `sp_add_job … @job_id OUTPUT`, then always `sp_add_jobserver @server_name = N'(local)'` (a job with no target server never runs).
- **Update:** `sp_update_job`.
- **Delete:** `sp_delete_job @job_id, @delete_unused_schedule = 0`. This is critical; otherwise SQL Server also deletes schedules that `sqlserver_agent_schedule` manages.

### `sqlserver_agent_job_step`
ID: `sqlserver://host:port/agent_job/<job_id>/step/<url-escaped name>`

```hcl
resource "sqlserver_agent_job_step" "extract" {
  job_id  = sqlserver_agent_job.etl.job_id                 # first step
  name    = "extract"
  command = "EXEC dbo.extract"
}

resource "sqlserver_agent_job_step" "load" {
  previous_step_id  = sqlserver_agent_job_step.extract.id  # every later step
  name              = "load"
  command           = "EXEC dbo.load"
  on_fail_action    = "go_to_step"                         # backward jump: declared on the source
  on_fail_step_name = sqlserver_agent_job_step.extract.name
}

resource "sqlserver_agent_job_step" "notify" {
  previous_step_id   = sqlserver_agent_job_step.load.id
  name               = "notify"
  command            = "EXEC dbo.notify_failure"
  handles_failure_of = [sqlserver_agent_job_step.extract.name] # forward jump: declared on the target
}
```

| Attribute | Kind | Notes |
|---|---|---|
| `job_id` | Optional+Computed | Set on the first step only. For later steps it is taken from the job GUID in `previous_step_id`. Exactly one of `job_id` / `previous_step_id`. A change to a *different* job is ForceNew (CustomizeDiff); switching between `job_id` and `previous_step_id` within the same job is an in-place move. |
| `previous_step_id` | Optional | Resource ID of the step before this one. Changing it moves the step in place. |
| `name` | Required, ForceNew | |
| `step_id` | Computed | current position, read from msdb |
| `command` | Required | T-SQL |
| `database_name` | Optional | default `master`; removing it from config moves the step back to `master`. Not a ForceNew: changed in place via `sp_update_jobstep`. |
| `database_user_name` | Optional | the step runs as `EXECUTE AS USER` for this user in `database_name`. Only allowed when both the job owner and the login Terraform connects with are sysadmin (see **Run-as** below). |
| `on_success_action` | Optional | `quit_with_success \| quit_with_failure \| go_to_next_step \| go_to_step`, default `go_to_next_step` (as in SSMS) |
| `on_success_step_name` | Optional | required iff action = `go_to_step`; **must be an earlier step** |
| `on_fail_action` | Optional | default `quit_with_failure` |
| `on_fail_step_name` | Optional | required iff action = `go_to_step`; **must be an earlier step** |
| `handles_success_of` | Optional set of step names | earlier steps whose success action jumps to this step |
| `handles_failure_of` | Optional set of step names | earlier steps whose failure action jumps to this step |
| `retry_attempts` / `retry_interval` | Optional | default 0 (interval in minutes) |
| `output_file_name` | Optional | documented as not supported on MI |
| `append_output_file`, `log_to_table`, `append_log_to_table`, `include_step_output_in_history` | Optional bool | mapped to the `flags` bitmask (2 / 8 / 16 / 32 / 4) |
| `step_uid` | Computed | |

**Ordering**
- **Create:** take the job lock. Position = position of `previous_step_id` + 1, or 1 for a head step, looked up by name at apply time. Then `sp_add_jobstep` at that position.
- **Read:** look the step up by name in `msdb.dbo.sysjobsteps`. Set `step_id`, and set `previous_step_id` to the ID of the step currently in front of it (or clear it and set `job_id` when it's step 1), so out-of-band reordering shows as drift. If the job or step is gone, clear the ID.
- **Move (`previous_step_id` changed):** take the lock, `sp_delete_jobstep`, then `sp_add_jobstep` at the new position with the full settings from config (the `step_uid` changes, which is fine). Because positions are always resolved from names at apply time, inserting or deleting a middle step converges in one apply whichever order Terraform runs the operations in.
- **Delete:** take the lock, then `sp_delete_jobstep`.
- **Never pass `@step_id = 0` (or a negative number) to `sp_delete_jobstep`:** `0` deletes **every** step of the job (spike result S1). The by-name lookup returns an error when the step isn't found instead of a zero value, and the delete helper refuses any `step_id < 1`.
- **Steps are always addressed by name.** The `step_id` in state is never used to target a write; the current number is looked up under the lock.
- **No forks:** two steps naming the same previous step (or two head steps) is always an error, caught at plan time by the plan registry below.

**Jumps**
- **Backward (source-owned):** `on_*_step_name` is resolved to the target's current number at apply time. Error if the target is not earlier in the chain, with a hint to use `handles_*_of` on the target instead.
- **Forward (target-owned):** on Create/Update of the target, for each listed source: `sp_update_jobstep` with `on_*_action = 4` (go to step) and `on_*_step_id` = the target's number.
  - Sources removed from the list, and all sources when the target is deleted, are reset to the default action (`quit_with_failure` for failure, `go_to_next_step` for success), but only if they still point at this target.
  - **Read** lists every step whose `on_*_step_id` points at this step and sets `handles_*_of` from it.
- **Source side of a forward jump:** when a step reads an `on_*_action` that jumps to a *later* step, that jump belongs to the target, so Read leaves the source's `on_*_action` / `on_*_step_name` at their prior state values. Create/Update of a step only sends **changed** parameters to `sp_update_jobstep`, so editing the source's `command` never clears a forward jump. `sp_update_jobstep` keeps the current value for any parameter passed as `NULL` (spike result S3), so to **clear** an optional string (`database_user_name`, `output_file_name`) the provider passes `N''`, which the procedure turns into `NULL`. For `database_name`, `N''` falls back to `master`.
- **Conflicts:**
  - two targets claim the same source for the same outcome: **plan-time** error from the plan registry;
  - a source listed in `handles_*_of` already has a non-default action of that kind set in its own config: apply-time error, raised by the later step, which always applies after the earlier one;
  - `on_*_step_name` points at a later step: apply-time error (positions are only known once the chain exists).
- **Renumbering:** before every insert, delete or move (under the lock) the provider snapshots all jumps in the job as source name → target name; afterwards it rewrites every `on_*_step_id` that no longer matches. Spike result S1 shows this is **required**, not just defensive: `sp_add_jobstep` renumbers jumps correctly, but `sp_delete_jobstep` renumbers first and then resets jumps that point at the deleted number, so a jump to the step *right after* the deleted one is wrongly reset to "quit with success/failure". Jumps that pointed at the deleted step itself are also reset; the rewrite sets those to this provider's defaults (`go_to_next_step` / `quit_with_failure`) as described above, rather than msdb's `quit_with_success` for success.
- **Documented limits:** a step can't jump to itself.

**Run-as (`database_user_name`)**
- Empty: SQL Server's own rules apply. With a sysadmin job owner the step runs as the SQL Agent service account; with a non-sysadmin owner it runs as the owner. It never uses the login Terraform connects with, unless that login is the owner. (Confirmed in spike result S3b.)
- Set: on Create/Update, under the job lock and before `sp_add_jobstep` / `sp_update_jobstep`, the provider reads the job's current owner from `msdb.dbo.sysjobs` and checks `IS_SRVROLEMEMBER('sysadmin', <owner>) = 1`. If not, apply fails with e.g. `step "load" sets database_user_name, but job "nightly_etl" is owned by "etl_owner", which is not sysadmin; SQL Server only honours database_user_name for sysadmin-owned jobs. Remove database_user_name or make the owner sysadmin.` This replaces SQL Server's silent ignore: a non-sysadmin owner's step runs as the owner even when `database_user_name` is stored (spike result S3b).
- Set, second check: the login Terraform connects with must also be sysadmin (`IS_SRVROLEMEMBER('sysadmin') = 1`). Otherwise `sp_add_jobstep` fails with "Only a sysadmin can specify '@database_user_name' parameter", but `sp_update_jobstep` silently drops the value without an error, which would show up as permanent drift. The provider raises the same kind of clear error for both.
- The check runs at apply time, not plan time, because the owner may be unknown at plan (it defaults to the connecting login) and its role membership can only be read from the server.
- The provider doesn't check that the user exists in `database_name`: SQL Server already does that in `sp_add_jobstep` / `sp_update_jobstep` for SQL users ("The specified @database_user_name ('x') does not exist"). Windows names (containing `\`) aren't checked by SQL Server until the job runs.
- Changing the job's `owner_login_name` to a non-sysadmin while a step still sets `database_user_name` isn't caught: the job resource doesn't know about its steps, and the step only re-checks when it is itself created or updated. This is documented rather than checked.

**Plan registry (chain validation)**
- `sqlserver/agent_plan_registry.go`: a package-level, mutex-guarded registry. All resources in one plan are planned by the same provider process, so it is shared by every step in that plan.
- Each step's `CustomizeDiff` (which SDK v2 runs for every resource, including those without changes) records the claims from its **config**:
  - "in job *J*, the step after *P* is me" (*P* empty for the head step);
  - "I handle the success / failure of step *S*" for each entry in `handles_*_of`.
- If a different step already holds the same claim, the plan fails, e.g. `steps "transform" and "X" both follow "extract" in job "nightly_etl"; SQL Server jobs can't fork`.
- Claims are keyed by job GUID + step name, so Terraform re-planning the same step during apply is not a second claim.
- Because the registry holds config claims and not the current msdb order, the temporary overlap while inserting a step in the middle never causes a false error:

  | Change | Config claims | Result |
  |---|---|---|
  | Insert `X` correctly | `extract → X`, `X → transform` | OK |
  | Insert `X`, forget `transform` | `extract → X` and `extract → transform` | error at plan |
  | Swap two steps | `extract → load`, `load → transform` | OK |
  | Two head steps | `(head) → a` and `(head) → b` | error at plan |

- When `previous_step_id` or the job GUID is still unknown at plan time (a brand-new chain), the check is skipped. Terraform plans each resource again just before applying it, with known values, so the same check then fails during apply instead.
- **Plan and apply run in different provider processes** (spike result S7): `terraform plan` plans every resource, changed or not, in one process, but `terraform apply` re-plans only the resources that change, in a new process. So during apply the registry only holds claims from changed steps. That's enough for the case above: a step whose `previous_step_id` is unknown at plan follows a new step, so it can only fork with other new (and so re-planned) steps. Because each step is re-planned just before it's applied, the failing step's sibling may already have been created when the error appears; the next plan then shows the fork at plan time.
- **Limits (documented):** the registry is per provider process, so steps of one job managed from two Terraform configurations or two provider aliases aren't checked against each other; `-target` plans can miss a fork, which then shows up as permanent drift rather than a false error.

**Per-job lock**
- `sqlserver/agent_lock.go`: a package-level `sync.Map` of `*sync.Mutex` keyed by job GUID (`sqlserverProvider` is passed around by value, so the lock can't be a field on it).
- Held for the whole read-modify-write of every step Create/Update/Delete, including the jump snapshot and rewrite. Operations on different jobs still run in parallel.
- Needed even with the chain: Terraform runs independent operations in parallel, for example deleting step 2 and updating step 5's `command`. Without the lock, the update could look up step 5, the delete shifts it to 4, and the update writes to the wrong step.
- Only protects within one provider process. Concurrent applies from other pipelines or SSMS edits are out of scope and documented.

### `sqlserver_agent_schedule`
ID: `sqlserver://host:port/agent_schedule/<schedule_id>`. msdb doesn't require schedule names to be unique, so the ID uses the integer.

| Attribute | Notes |
|---|---|
| `name` | Required; updated in place via `@new_name` |
| `enabled` | default `true` |
| `frequency_type` | `once \| daily \| weekly \| monthly \| monthly_relative \| agent_start \| idle` → 1/4/8/16/32/64/128 |
| `recurrence_factor` | "every N days/weeks/months". For `daily` it maps to `freq_interval`, otherwise to `freq_recurrence_factor` |
| `weekly_days` | set of `sunday…saturday` → bitmask 1, 2, 4 … 64 |
| `monthly_day` | 1–31. Values 29–31 give a plan-time **warning** (not an error) that the day doesn't exist in every month, pointing at `monthly_relative_week = "last"` + `monthly_relative_day = "day"` for the last day of the month. Wording (spike result S5): `SQL Agent skips months without day 31 (Feb, Apr, Jun, Sep, Nov); use ... for the last day of the month`, adjusted per value (day 29 only runs in February in leap years). |
| `monthly_relative_week` | `first \| second \| third \| fourth \| last` → 1/2/4/8/16 |
| `monthly_relative_day` | `sunday…saturday \| day \| weekday \| weekend_day` → 1–10 |
| `subday_type` / `subday_interval` | `once \| seconds \| minutes \| hours` → 1/2/4/8. `subday_type` defaults to `once` (run once a day at `active_start_time`). `subday_interval` is required for `seconds`/`minutes`/`hours` and forbidden for `once`; for `seconds` it must be at least 10 (SQL Agent's minimum), checked at plan time. |
| `active_start_date` / `active_end_date` | `YYYY-MM-DD` ↔ `yyyymmdd` int; Optional+Computed (defaults are today and `9999-12-31`). Parsed at plan time with Go's `time.Parse` (layout `2006-01-02`), not just a pattern match, so `2026-02-30` or `2026-13-01` fail at plan instead of with a vague SQL Server error at apply (S6: SQL Server only says `valid values are: 19900101..99991231`). Dates before `1990-01-01` are rejected too, matching SQL Server's range. For "every N" schedules (`recurrence_factor` > 1) the start date anchors the cycle, so the docs tell users to set it explicitly. |
| `active_start_time` / `active_end_time` | `HH:MM:SS` ↔ `hhmmss` int; defaults `00:00:00` / `23:59:59`. Parsed at plan time with `time.Parse` (layout `15:04:05`), so `24:00:00` or `12:61:00` fail at plan. An end time **earlier** than the start time is allowed: SQL Server treats it as a window that crosses midnight (spike result S6). An end time **equal** to the start time with a `seconds`/`minutes`/`hours` `subday_type` is a plan-time error, as SQL Server rejects it. `active_end_time = "00:00:00"` is a plan-time error with a hint to use `23:59:59`: SQL Server treats `0` as "not set" and stores `23:59:59`, which would be permanent drift. |
| `owner_login_name` | Optional+Computed |
| `schedule_id`, `schedule_uid` | Computed |

- A **CustomizeDiff** checks that only the fields relevant to the chosen `frequency_type` are set (for example, `weekly_days` is required for `weekly` and forbidden otherwise), and the `subday_*` rules above (`subday_type` doesn't apply to `once`, `agent_start` and `idle`). It also fails the plan when `active_end_date` is before `active_start_date` (when both are known at plan time).
- **Plan-time checks vs. warnings:** dates (including the 1990-01-01 lower bound), times, the end-date order, equal start/end times with a sub-day interval, an end time of `00:00:00`, and the `subday_*` rules are errors; `monthly_day` 29–31 is a warning (`ValidateDiagFunc` returning `diag.Warning`, since SDK v2's CustomizeDiff can't emit warnings). Daylight saving time isn't checked: a time like 02:15 is valid on almost every day of the year, and the provider doesn't know the server's time zone at plan time. It's documented instead.
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

- **Attributes:** `name` (updated in place via `@new_name`), `enabled` (default true), `email_address`, `category_name` (Optional, default `[Uncategorized]`, so removing it from config moves the operator back), and computed `operator_id`.
- **CRUD:** `sp_add_operator` / `sp_update_operator` / `sp_delete_operator`.
- **Ordering:** Database Mail isn't needed to create an operator. A job that references `sqlserver_agent_operator.x.name` creates the right dependency ordering.
- **Mail delivery is out of scope:** the operator only stores the address. Mail is sent by Database Mail through the profile SQL Agent is configured with, which this provider doesn't manage (see section 8). The provider doesn't check that Database Mail is set up; this is documented instead.

## 3. Code layout (following the existing layering)

**New files**
- `sqlserver/model/agent_job.go`, `agent_job_step.go`, `agent_schedule.go`, `agent_operator.go`: structs in raw msdb form (ints, bitmasks).
- `sql/agent_job.go`, `sql/agent_job_step.go`, `sql/agent_schedule.go`, `sql/agent_job_schedule.go`, `sql/agent_operator.go`: `*Connector` methods that call `msdb.dbo.sp_*` with `sql.Named` parameters, matching the style in `sql/database.go`.
  - `sql/agent_job_step.go` includes `ListAgentJobSteps(ctx, jobID)`, which returns every step of a job with its number, name and jump fields. Position lookups, the jump snapshot and the forward-jump Read all use it.
  - `UpdateAgentJobStep` passes only the parameters that changed (others as `NULL`, which `sp_update_jobstep` leaves untouched).
  - GUIDs are always selected as `CONVERT(nvarchar(36), job_id)`. This sidesteps go-mssqldb's mixed-endian `UniqueIdentifier` byte order.
  - The output id comes from a `DECLARE @id …; EXEC … OUTPUT; SELECT @id` batch run through `QueryRowContext`.
- `sql/agent_common.go`: `EnsureAgentAvailable(ctx)`, which checks `SERVERPROPERTY('EngineEdition')` and returns a clear error for 5 (Azure SQL Database). Every agent Create calls it.
- `sqlserver/agent_schedule_encoding.go`: pure functions for the friendly ↔ msdb translation (days bitmask, relative week and day, times, dates), shared by Create, Read and Update.
- `sqlserver/agent_step_jumps.go`: pure functions for the jump snapshot (numbers → names) and rewrite (names → corrected numbers), and for deciding whether a jump is forward or backward.
- `sqlserver/agent_plan_registry.go`: the plan registry described above.
- `sqlserver/agent_lock.go`: the per-job lock described above.
- `sqlserver/resource_agent_job.go`, `resource_agent_job_step.go`, `resource_agent_schedule.go`, `resource_agent_job_schedule.go`, `resource_agent_operator.go`: each with its own `XxxConnector` interface, schema, CRUD, `getXxxID` and `getXxxConnector`, following the pattern in `resource_database.go`.

**Changed files**
- `sqlserver/const.go`: property-name constants and the enum maps.
- `sqlserver/provider.go`: register the 5 resources in `ResourcesMap`.
- `sqlserver/provider_test.go`: extend `TestConnector` with `GetAgentJob`, `GetAgentJobStep`, `GetAgentSchedule`, `GetAgentJobSchedule` and `GetAgentOperator`.
- `docker-compose/docker-compose.yml` and `test-fixtures/local/main.tf`: add `MSSQL_AGENT_ENABLED=true`.
- `README.md`, plus a version bump in `Makefile` (0.4.0 → 0.5.0).

## 4. Tests
- **Unit tests** (table-driven, like `sql/resource_governor_test.go`):
  - the schedule encoding round-trips (every frequency type, bitmask edges, `last`/`weekend_day`, the time and date formats) and the step `flags` mapping;
  - the date/time validators: `2026-02-30`, `2026-13-01`, `2025-02-29`, `1989-12-31`, `24:00:00`, `12:61:00` and malformed strings are rejected; `2028-02-29` and `23:59:59` are accepted; an end time of `00:00:00` and equal start/end times with a sub-day interval are rejected; an end time earlier than the start time is accepted; the `monthly_day` warning fires for 29–31 only;
  - the jump snapshot/rewrite after an insert, a delete and a move, and the forward/backward classification; include the S1 case of deleting step *n* while a jump points at step *n+1*;
  - the delete helper refuses `step_id < 1`;
  - the plan registry: each row of the table above, re-registration of the same step, and unknown values being skipped.
- **Acceptance tests** (`TestAccAgent*_Local_*`, using `templateToString`, Exists and Destroy checks):
  1. Job: basic, then update (rename, enabled, description, category), then remove `category_name` and assert the job is back in `[Uncategorized (Local)]` with an empty plan, then destroy.
  2. Steps (chain):
     - three chained steps are created in the right order in one apply;
     - insert a step in the middle; delete a middle step; move a step to the head. Each converges in one apply with an empty plan afterwards;
     - delete step 2 and change step 5's `command` in the same apply, and assert the right step changed (covers the lock);
     - a fork (inserting a step but leaving the next step pointing at the old predecessor) and two head steps fail at plan time (`ExpectError`), including on an existing chain where only the new step has changes;
     - a fork in a brand-new chain fails during apply.
  3. Jumps:
     - a backward jump via `on_fail_step_name`;
     - a forward jump via `handles_failure_of`, created in one apply with an empty plan afterwards;
     - the jump survives an insert before its target (renumbering);
     - the jump survives deleting the step directly before its target (the `sp_delete_jobstep` bug from spike result S1);
     - changing the source's `command` keeps the forward jump; removing the target resets the source to the default action;
     - the conflict errors: double claim (at plan), forward `on_*_step_name` and a source with its own non-default action (at apply).
  4. Step run-as and database:
     - `database_name` set to a user database, then removed from config: the step is back in `master` with an empty plan afterwards;
     - `database_user_name` with a sysadmin owner applies and reads back unchanged; removing it from config clears it (sent as `N''`) with an empty plan afterwards;
     - `database_user_name` on a job owned by a non-sysadmin login fails at apply with the run-as error (`ExpectError`);
     - `database_user_name` set while connected as a non-sysadmin login fails at apply with the caller error, both on create and on update (`ExpectError`).
  5. Schedule: one case per `frequency_type`, checking the friendly values read back unchanged and that the plan is empty afterwards. The cases are taken from the examples in section 9 so the documented examples are known to work. Also: `subday_type` omitted reads back as `once` with an empty plan, and `subday_interval < 10` with `seconds` fails at plan (`ExpectError`); an impossible date such as `2026-02-30` and an `active_end_date` before `active_start_date` both fail at plan (`ExpectError`).
  6. Attachment: attach and detach. Assert the schedule **still exists** after the job is destroyed (this covers the `@delete_unused_schedule = 0` fix).
  7. Operator plus a job with `notify_level_email = on_failure`.
- `TestProvider` (`InternalValidate`) covers all the new schemas automatically.

## 5. Docs and examples
- `docs/resources/agent_job.md`, `agent_job_step.md`, `agent_schedule.md`, `agent_job_schedule.md`, `agent_operator.md`, in the same style as `database.md` but without an Import section.
- `examples/agent_job/main.tf`: an end-to-end example with an operator, a job, 3 chained steps, a forward failure jump to a notify step, a shared schedule attached to two jobs, and email on failure.
- The docs cover:
  - the required permissions (`SQLAgentOperatorRole` / sysadmin for owner changes and `database_user_name`) and MI limitations;
  - who a step runs as: with `database_user_name` empty and a sysadmin owner (the default when Terraform connects as sysadmin) the step runs as the SQL Agent service account, i.e. with full sysadmin rights; `database_user_name` or a non-sysadmin owner limits that, and `EXECUTE AS USER` can't reach other databases;
  - when to use `master` vs. a user database for `database_name`, and referencing `sqlserver_database.x.name` so the step depends on the database;
  - the chain (`job_id` on the first step, `previous_step_id` on the rest), that it can't fork, and the plan-time check with its limits;
  - the jump rule (declared on the later step), with an example of each direction;
  - that the step lock only covers a single apply;
  - schedules (in `agent_schedule.md`):
    - an "Example Usage" section built from the examples in section 9, grouped as daily, weekly, monthly, monthly relative, and one-off/event-based, plus `examples/agent_schedule/main.tf` with all of them so they can be applied as-is;
    - times are in the server's local time zone; on Managed Instance that's the instance time zone chosen at creation (UTC by default, can't be changed later);
    - for "every N days/weeks/months" schedules, `active_start_date` decides which days count, and defaults to the day of the apply, so set it explicitly (examples 2, 5 and 9);
    - `subday_type` defaults to `once`, and the 10-second minimum for `seconds`;
    - `monthly_day = 31` vs. `monthly_relative_week = "last"` + `monthly_relative_day = "day"` for the last day of the month, that Agent **skips** months without that day (spike result S5; day 29 only runs in February in leap years), and the plan-time warning;
    - daylight saving time: times are local server time, so on servers that observe DST a job in the skipped spring hour may run at the change or be skipped that day, and around the autumn change a job running every N minutes may pause for about an hour. Advice: avoid roughly 01:00–03:00 local time, or use a UTC server time zone (the MI default), and make jobs safe to re-run;
    - an `active_end_time` earlier than `active_start_time` means a window crossing midnight, e.g. `22:00:00`–`02:00:00` (first run at the start time on the start date); equal times aren't allowed with a sub-day interval; use `23:59:59`, not `00:00:00`, for "until midnight";
    - `idle` needs the idle-CPU condition configured in Agent's properties, and is "not verified on Managed Instance" until someone tests it on a real MI (spike result S4);
  - email notifications (in `agent_operator.md`, linked from `notify_level_email` in `agent_job.md`): an operator only stores the address and mail is sent by Database Mail, which must be set up separately:
    - `Database Mail XPs` enabled, plus an SMTP account and a profile containing it;
    - on-prem/VM: SQL Agent's mail profile selected under Agent properties → Alert System, followed by an Agent restart;
    - Azure SQL Managed Instance: Agent always uses the profile named exactly `AzureManagedInstance_dbmail_profile`, there's no local SMTP relay so an external SMTP service is needed (port 587 with TLS, as port 25 is generally blocked), and the MI subnet's NSG/route table must allow outbound traffic to the SMTP host;
    - a minimal T-SQL setup snippet for MI (`sp_configure`, `sysmail_add_account_sp`, `sysmail_add_profile_sp`, `sysmail_add_profileaccount_sp`);
    - troubleshooting via `msdb.dbo.sysmail_allitems`, `msdb.dbo.sysmail_event_log` and the Agent error log.

## 6. Implementation order
1. **Spike:** run the Docker image with the agent enabled and confirm by hand:
   - whether `sp_add_jobstep` / `sp_delete_jobstep` shift other steps' `on_*_step_id` values and the job's `start_step_id`;
   - whether `sp_add_jobstep` accepts an `on_*_step_id` that doesn't exist yet;
   - that `sp_update_jobstep` leaves parameters passed as `NULL` untouched;
   - whether `frequency_type = idle` schedules work on Managed Instance, and how `monthly_day = 31` behaves in shorter months (skipped or run on the last day);
   - whether `sp_add_schedule` accepts `active_end_time` < `active_start_time`, and if so whether it means a window crossing midnight. If it's rejected, add a plan-time check for it; if it works, document it.

   And in a throwaway provider build:
   - that SDK v2 calls `CustomizeDiff` for resources with no changes;
   - that one provider process plans all resources of a `terraform plan`, so the registry sees every step.

   Two of these can't be fully answered with the Docker image:
   - **`idle` on Managed Instance** needs a real MI. If none is available during the spike, `idle` is documented as "not verified on Managed Instance" rather than as supported.
   - **`monthly_day = 31`** can't be observed by waiting, because Agent only exposes the *next* run date (`next_scheduled_run_date` in `msdb.dbo.sysjobactivity`; `next_run_date` in `sysjobschedules` is only refreshed periodically and read `0` right after attaching). Attach a day-31 schedule to a job with `active_start_date` set to the start of a future short month (e.g. `2027-02-01` or `2027-04-01`) and read which date Agent picks as the next run: the 28th/30th means "last day", the 31st of the following month means "skipped". Changing the container's clock isn't practical, since containers share the host's clock.

   The renumbering rewrite is built either way, but the msdb results decide how much of it is needed and what the tests assert. If either registry assumption fails, the plan registry is redesigned before the step resource is built.

   **Status: done on 2026-10-12, except `idle` on Managed Instance. Results are in section 10.** Both registry assumptions held, so no redesign is needed.
2. Docker fixtures and `EnsureAgentAvailable`.
3. Operator, the simplest resource, to set up the end-to-end pattern.
4. Job.
5. Schedule, with the encoding functions and their unit tests.
6. Job–schedule attachment.
7. Job step: the chain, the lock and the plan registry first, then backward jumps, then forward jumps and the renumbering rewrite.
8. Docs, example, README and version bump.

## 7. Defaults chosen without asking
- No data sources.
- Multi-server (MSX/TSX) targets are out of scope; jobs always target `(local)`.
- Operator pager and net-send fields are left out because they're deprecated.
- `on_success_action` defaults to `go_to_next_step`, matching SSMS rather than `sp_add_jobstep`'s `quit_with_success`.
- Removing a forward jump resets the source to the default action rather than to whatever it was before.
- `category_name` on the job and operator is Optional with the built-in uncategorized category as its default, not Optional+Computed, so that removing it from config reverts the server and changes made outside Terraform show up as drift. The same applies to the step's `database_name` (default `master`).
- `database_user_name` on a non-sysadmin-owned job is an apply-time error from the provider rather than left to SQL Server.
- `subday_type` defaults to `once`; `active_start_date` keeps today as its default (rather than becoming Required) and the docs explain when to set it.
- `monthly_day` 29–31 is a warning, not an error; daylight saving time is documented, not checked.

## 8. Follow-ups (out of scope for this plan)
- **Database Mail resources:** e.g. `sqlserver_database_mail_account` (SMTP server, port, SSL, credentials, with the password as a write-only argument like `password_wo` on `sqlserver_login`) and `sqlserver_database_mail_profile` (accounts with sequence numbers). On MI the profile would be named `AzureManagedInstance_dbmail_profile` so Agent uses it. Until then, Database Mail is configured outside Terraform and only documented here.

## 9. Appendix: schedule examples
Source for the `agent_schedule.md` "Example Usage" section, `examples/agent_schedule/main.tf`, and the schedule acceptance tests. `subday_type` is left out where the default `once` applies.

**Daily**

```hcl
# 1. Every day at 02:30
resource "sqlserver_agent_schedule" "nightly_0230" {
  name              = "Daily 02:30"
  frequency_type    = "daily"
  active_start_time = "02:30:00"
}

# 2. Every 3 days at 04:00, counting from a fixed date
resource "sqlserver_agent_schedule" "every_3_days" {
  name              = "Every 3 days at 04:00"
  frequency_type    = "daily"
  recurrence_factor = 3
  active_start_time = "04:00:00"
  active_start_date = "2026-11-01" # anchors the cycle
}

# 11. Every 5 minutes, only during a date range (Black Friday to Cyber Monday)
resource "sqlserver_agent_schedule" "black_friday_watch" {
  name              = "Black Friday 2026 - every 5 min"
  frequency_type    = "daily"
  subday_type       = "minutes"
  subday_interval   = 5
  active_start_date = "2026-11-26"
  active_end_date   = "2026-11-30"
}
```

**Weekly**

```hcl
# 3. Sunday 01:00 (weekly full backup)
resource "sqlserver_agent_schedule" "weekly_full_backup" {
  name              = "Weekly full backup - Sunday 01:00"
  frequency_type    = "weekly"
  recurrence_factor = 1
  weekly_days       = ["sunday"]
  active_start_time = "01:00:00"
}

# 4. Every 15 minutes, 08:00-18:00, Monday to Friday
resource "sqlserver_agent_schedule" "office_hours_15m" {
  name              = "Weekdays every 15 min, 08:00-18:00"
  frequency_type    = "weekly"
  recurrence_factor = 1
  weekly_days       = ["monday", "tuesday", "wednesday", "thursday", "friday"]
  subday_type       = "minutes"
  subday_interval   = 15
  active_start_time = "08:00:00"
  active_end_time   = "18:00:00"
}

# 5. Every other Friday at 17:30 (payroll)
resource "sqlserver_agent_schedule" "biweekly_payroll" {
  name              = "Payroll export - every other Friday 17:30"
  frequency_type    = "weekly"
  recurrence_factor = 2
  weekly_days       = ["friday"]
  active_start_time = "17:30:00"
  active_start_date = "2026-10-16" # a payroll Friday; anchors which Fridays count
}
```

**Monthly (fixed day)**

```hcl
# 9. Quarterly: 1st of Jan/Apr/Jul/Oct at 07:00
resource "sqlserver_agent_schedule" "quarterly" {
  name              = "Quarterly report - 1st of quarter 07:00"
  frequency_type    = "monthly"
  recurrence_factor = 3
  monthly_day       = 1
  active_start_time = "07:00:00"
  active_start_date = "2027-01-01" # anchors the quarters to January
}
```

**Monthly relative**

```hcl
# 6. First Sunday of the month at 03:00
resource "sqlserver_agent_schedule" "first_sunday" {
  name                  = "First Sunday of the month 03:00"
  frequency_type        = "monthly_relative"
  recurrence_factor     = 1
  monthly_relative_week = "first"
  monthly_relative_day  = "sunday"
  active_start_time     = "03:00:00"
}

# 7. Second Wednesday at 06:00 (post-patch checks). Not always the day after
#    Patch Tuesday: if the 1st is a Wednesday, it falls before the second Tuesday.
resource "sqlserver_agent_schedule" "post_patch_tuesday" {
  name                  = "Second Wednesday - post-patch checks 06:00"
  frequency_type        = "monthly_relative"
  recurrence_factor     = 1
  monthly_relative_week = "second"
  monthly_relative_day  = "wednesday"
  active_start_time     = "06:00:00"
}

# 8. Last weekday (Mon-Fri) of the month at 20:00 (month-end close).
#    Use monthly_relative_day = "day" for the last calendar day.
resource "sqlserver_agent_schedule" "month_end_close" {
  name                  = "Last weekday of the month 20:00"
  frequency_type        = "monthly_relative"
  recurrence_factor     = 1
  monthly_relative_week = "last"
  monthly_relative_day  = "weekday"
  active_start_time     = "20:00:00"
}

# 10. First weekend day (Saturday or Sunday) of the month, every 6 hours
resource "sqlserver_agent_schedule" "first_weekend_day_6h" {
  name                  = "First weekend day, every 6h"
  frequency_type        = "monthly_relative"
  recurrence_factor     = 1
  monthly_relative_week = "first"
  monthly_relative_day  = "weekend_day"
  subday_type           = "hours"
  subday_interval       = 6
}
```

**One-off and event-based**

```hcl
# 12. Once, at a fixed date and time (migration cutover)
resource "sqlserver_agent_schedule" "cutover" {
  name              = "One-off cutover"
  frequency_type    = "once"
  active_start_date = "2026-12-05"
  active_start_time = "22:00:00"
}

# 13. Whenever SQL Server Agent starts
resource "sqlserver_agent_schedule" "on_agent_start" {
  name           = "On SQL Agent start"
  frequency_type = "agent_start"
}

# 14. Whenever the CPU is idle (needs the idle-CPU condition in Agent's properties)
resource "sqlserver_agent_schedule" "when_idle" {
  name           = "When CPU idle"
  frequency_type = "idle"
}
```

**Sharing one schedule between jobs**

```hcl
resource "sqlserver_agent_job_schedule" "etl_nightly" {
  job_id      = sqlserver_agent_job.nightly_etl.job_id
  schedule_id = sqlserver_agent_schedule.nightly_0230.schedule_id
}

resource "sqlserver_agent_job_schedule" "stats_nightly" {
  job_id      = sqlserver_agent_job.update_stats.job_id
  schedule_id = sqlserver_agent_schedule.nightly_0230.schedule_id
}
```

## 10. Spike results (2026-10-12)
Run against SQL Server 2022 Developer Edition (RTM-CU27, 16.0.4295.3) on Linux in Docker, with SQL Agent enabled, plus a throwaway SDK v2.40.1 provider with Terraform 1.16.5. The Agent stored procedures were also read from `msdb.sys.sql_modules` to confirm the behaviour. They're the same T-SQL on Windows and on Managed Instance, but MI wasn't tested directly.

| # | Question | Result | Effect on the plan |
|---|---|---|---|
| S1 | Do step inserts/deletes renumber jumps and `start_step_id`? | **Insert: correct.** `sp_add_jobstep` shifts every `on_*_step_id` ≥ the new position. **Delete: buggy.** `sp_delete_jobstep` decrements jumps > the deleted number *before* resetting jumps = the deleted number, so a jump to the step right after the deleted one is reset to quit (success → `quit_with_success`, failure → `quit_with_failure`). Seen in the procedure source and reproduced: A jumps to C (3), delete B (2) → A's jump is lost. `start_step_id` is never changed by either procedure; it stayed `1` throughout. `@step_id = 0` deletes **all** steps. New steps default to `on_success_action = quit_with_success`. | The renumbering rewrite is required after deletes and moves. Never call `sp_delete_jobstep` with `step_id < 1`. Always pass explicit actions on `sp_add_jobstep`. |
| S2 | Does `sp_add_jobstep` accept an `on_*_step_id` that doesn't exist yet? | **Yes.** `sp_verify_jobstep` allows forward references on purpose and only prints informational message 14236. A jump to the step itself (or a step id < 1) is rejected (14235). | No change. Forward jumps stay target-owned, because the target's number isn't stable; the "can't jump to itself" limit is confirmed. |
| S3 | Does `sp_update_jobstep` leave `NULL` parameters untouched? | **Yes**, every parameter falls back to the current value when `NULL`. `N''` clears `command`, `server`, `database_name` (→ `master`), `database_user_name` and `output_file_name`. | "Send only changed parameters" works. Clearing an optional string sends `N''`. |
| S3b | `database_user_name` behaviour (found while reading S3) | Ran real jobs. Owner `sa`, no user → `NT AUTHORITY\NETWORK SERVICE` (the Agent service account on Linux). Owner `sa`, user `reader` → `reader`. Owner non-sysadmin, with or without `reader` → the owner (`lowpriv`); the stored user is **silently ignored**. Caller not sysadmin: `sp_add_jobstep` errors (14583), `sp_update_jobstep` **silently drops** the value. SQL users are checked for existence at create/update (14262); Windows names aren't. | Run-as section: keep the owner check, add a caller-is-sysadmin check, and drop the "doesn't verify the user exists" note. |
| S4 | `idle` on Managed Instance | **Not tested.** Needs a real MI. | Documented as "not verified on Managed Instance". |
| S5 | `monthly_day` 29–31 in shorter months | **Skipped.** From a 2027-02-01 start: day 31 → 2027-03-31, day 30 → 2027-03-30, day 29 → 2027-03-29. From 2027-04-01: day 31 → 2027-05-31. From 2028-02-01 (leap year): day 29 → 2028-02-29. `last` + `day` from 2027-02-01 → 2027-02-28. | Warning wording and docs: "skips months without that day". |
| S6 | `active_end_time` < `active_start_time`; related schedule edge cases | **Accepted.** The procedure source says it's a window spanning midnight. A 22:00–02:00 hourly schedule's first run is 22:00 on the start date; runs after midnight weren't observed directly (that would mean waiting overnight). Also: equal start/end times with a sub-day interval are rejected (14266). An end time of `0` is stored as `235959`. `subday_interval < 10` seconds is rejected (14278). Impossible dates like `20270230` are rejected with only `valid values are: 19900101..99991231`. | No end-after-start check; document the midnight window. New plan-time errors: equal times with a sub-day interval, end time `00:00:00`, dates before 1990-01-01. |
| S7a | Does SDK v2 run `CustomizeDiff` for resources with no changes? | **Yes.** On `terraform plan` with nothing changed, `CustomizeDiff` ran for all 3 resources. | Registry design holds. |
| S7b | Does one provider process plan all resources? | **Yes, per phase.** All resources were planned in one process (several short-lived instances start for validation, so the registry must be package-level, which it is). `terraform apply` uses a **new process** and re-plans **only changed** resources. For a brand-new chain all steps are re-planned there, and `previous_step_id` was unknown at plan but known at apply. | Registry design holds. Section 2 now explains the apply-phase limits. |

## 11. Implementation notes (2026-10-12)
Implemented on `feature/sql_agent_jobs`. All agent unit and acceptance tests pass against SQL Server 2022 in Docker, together with the existing test suite, and `examples/agent_job` applies with an empty plan afterwards and runs correctly in SQL Server Agent. Where the code differs from the plan above, or the implementation found something new:

- **Plan registry lives on the configured provider**, not in a package-level variable (`sqlserverProvider.agentRegistry`, a pointer, so copies of the provider share it). A new registry per configure means each Terraform command starts empty. In the acceptance tests, where every test step reuses one provider process, a package-level registry would have kept the previous step's claims and raised false fork errors. The per-job lock stays package-level as planned.
- **Deadlocks:** creating several jobs in parallel deadlocks inside msdb's procedures (seen in the attachment test). Every Agent write is retried when SQL Server picks it as the deadlock victim (error 1205, up to 5 attempts), and every multi-statement write runs in one transaction so a retry never repeats half a change.
- **Swallowed errors:** a batch that called an msdb procedure and then `SELECT`ed the new ID returned NULL instead of the procedure's error (e.g. a duplicate job name). Create batches now check each procedure's return code, stop at the first failure, and return IDs through `OUTPUT` parameters with `ExecContext`. `TestAccAgentJob_Local_DuplicateName` covers it.
- **Permissions:** the provider reads the msdb tables (`sysjobs`, `sysoperators`, ...) directly. A login with only `SQLAgentUserRole` can't, so a non-sysadmin provider login also needs `db_datareader` in msdb. Documented in `agent_job.md`.
- **Job description:** `sp_add_job` stores "No description available." when the description is empty, so Create clears it in the same transaction.
- **Defaults:** `active_end_date` is Optional with a default of `9999-12-31` (not Optional+Computed), and `recurrence_factor` defaults to `1`, in line with the "fall back to the default" choices for `category_name` and `database_name`. `active_start_date` stays Optional+Computed.
- **Step `command`:** leading and trailing whitespace is ignored when comparing, because `sp_update_jobstep` trims the command but `sp_add_jobstep` doesn't.
- **Step moves across jobs:** the step is ForceNew when its job changes and both job IDs are known, or when `job_id` comes from a job being created. When the job is only known at apply (a whole chain moving to a new job), Update removes the step from the old job and creates it in the new one.
- **Schedule examples** live in `examples/agent_schedule/schedules.tf` instead of only in section 9; `docs/resources/agent_schedule.md` is built from that file, and `TestAccAgentSchedule_Local_Examples` applies it.
- **README:** unchanged, as it doesn't list resources; the new resources are listed in `docs/index.md`.

