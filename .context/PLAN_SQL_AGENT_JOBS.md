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
| `database_name` | Optional+Computed | default `master` |
| `database_user_name` | Optional | runs as that user (needs sysadmin) |
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
- **Steps are always addressed by name.** The `step_id` in state is never used to target a write; the current number is looked up under the lock.
- **No forks:** two steps naming the same previous step (or two head steps) is always an error, caught at plan time by the plan registry below.

**Jumps**
- **Backward (source-owned):** `on_*_step_name` is resolved to the target's current number at apply time. Error if the target is not earlier in the chain, with a hint to use `handles_*_of` on the target instead.
- **Forward (target-owned):** on Create/Update of the target, for each listed source: `sp_update_jobstep` with `on_*_action = 4` (go to step) and `on_*_step_id` = the target's number.
  - Sources removed from the list, and all sources when the target is deleted, are reset to the default action (`quit_with_failure` for failure, `go_to_next_step` for success), but only if they still point at this target.
  - **Read** lists every step whose `on_*_step_id` points at this step and sets `handles_*_of` from it.
- **Source side of a forward jump:** when a step reads an `on_*_action` that jumps to a *later* step, that jump belongs to the target, so Read leaves the source's `on_*_action` / `on_*_step_name` at their prior state values. Create/Update of a step only sends **changed** parameters to `sp_update_jobstep`, so editing the source's `command` never clears a forward jump.
- **Conflicts:**
  - two targets claim the same source for the same outcome: **plan-time** error from the plan registry;
  - a source listed in `handles_*_of` already has a non-default action of that kind set in its own config: apply-time error, raised by the later step, which always applies after the earlier one;
  - `on_*_step_name` points at a later step: apply-time error (positions are only known once the chain exists).
- **Renumbering:** before every insert, delete or move (under the lock) the provider snapshots all jumps in the job as source name → target name; afterwards it rewrites every `on_*_step_id` that no longer matches. This holds whatever msdb itself does on renumbering (see the spike).
- **Documented limits:** a step can't jump to itself.

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
  - the jump snapshot/rewrite after an insert, a delete and a move, and the forward/backward classification;
  - the plan registry: each row of the table above, re-registration of the same step, and unknown values being skipped.
- **Acceptance tests** (`TestAccAgent*_Local_*`, using `templateToString`, Exists and Destroy checks):
  1. Job: basic, then update (rename, enabled, description, category), then destroy.
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
     - changing the source's `command` keeps the forward jump; removing the target resets the source to the default action;
     - the conflict errors: double claim (at plan), forward `on_*_step_name` and a source with its own non-default action (at apply).
  4. Schedule: one case per `frequency_type`, checking the friendly values read back unchanged and that the plan is empty afterwards.
  5. Attachment: attach and detach. Assert the schedule **still exists** after the job is destroyed (this covers the `@delete_unused_schedule = 0` fix).
  6. Operator plus a job with `notify_level_email = on_failure`.
- `TestProvider` (`InternalValidate`) covers all the new schemas automatically.

## 5. Docs and examples
- `docs/resources/agent_job.md`, `agent_job_step.md`, `agent_schedule.md`, `agent_job_schedule.md`, `agent_operator.md`, in the same style as `database.md` but without an Import section.
- `examples/agent_job/main.tf`: an end-to-end example with an operator, a job, 3 chained steps, a forward failure jump to a notify step, a shared schedule attached to two jobs, and email on failure.
- The docs cover:
  - the required permissions (`SQLAgentOperatorRole` / sysadmin for owner changes and `database_user_name`) and MI limitations;
  - the chain (`job_id` on the first step, `previous_step_id` on the rest), that it can't fork, and the plan-time check with its limits;
  - the jump rule (declared on the later step), with an example of each direction;
  - that the step lock only covers a single apply.

## 6. Implementation order
1. **Spike:** run the Docker image with the agent enabled and confirm by hand:
   - whether `sp_add_jobstep` / `sp_delete_jobstep` shift other steps' `on_*_step_id` values and the job's `start_step_id`;
   - whether `sp_add_jobstep` accepts an `on_*_step_id` that doesn't exist yet;
   - that `sp_update_jobstep` leaves parameters passed as `NULL` untouched.

   And in a throwaway provider build:
   - that SDK v2 calls `CustomizeDiff` for resources with no changes;
   - that one provider process plans all resources of a `terraform plan`, so the registry sees every step.

   The renumbering rewrite is built either way, but the msdb results decide how much of it is needed and what the tests assert. If either registry assumption fails, the plan registry is redesigned before the step resource is built.
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
