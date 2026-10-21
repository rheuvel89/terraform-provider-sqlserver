# Plan: SQL Server Agent email notifications for `terraform-provider-sqlserver`

Goal: make the email that SQL Server Agent sends to an operator when a job succeeds, fails or completes
(`notify_level_email` on `sqlserver_agent_job`) work end to end from Terraform. Today the provider manages the operator and
the job's notification level, but everything that actually delivers mail is configured by hand (see "Email Delivery" in
`docs/resources/agent_operator.md` and follow-up 8 in `PLAN_SQL_AGENT_JOBS.md`).

Targets, as for the agent jobs: **SQL Server on-premises (installed, or in a container)** and **Azure SQL Managed Instance**.
Azure SQL Database has no SQL Server Agent and fails fast with the existing error.

Status: **plan only, no provider code changed.** Spikes D1–D12 were run on 2026-10-21 (section 9). Decisions marked
**proposed** still need your answer (section 10).

## 0. What has to be in place for a notification email to arrive

Verified end to end on SQL Server 2022 in Docker (spikes D3, D4, D11): an operator, a job with `notify_level_email`, and a
failing step. Every link below is needed, and when any one is missing the job still runs and fails normally; the only trace
is `NOTE: Failed to notify '<operator>' via email.` in the job history and an error in the Agent log.

| # | Piece | On-premises (Windows) | On-premises (Linux / container) | Managed Instance | Managed by the provider today |
|---|---|---|---|---|---|
| 1 | `Database Mail XPs` server option = 1 | `sp_configure` (advanced option) | same (D1) | same (doc; not verified, M1) | No |
| 2 | A Database Mail **account** (SMTP server, port, TLS, username/password) | `sysmail_add_account_sp` | same | same | No |
| 3 | A Database Mail **profile** containing the account(s) | `sysmail_add_profile_sp`, `sysmail_add_profileaccount_sp` | same | same, and it must be named **`AzureManagedInstance_dbmail_profile`** | No |
| 4 | SQL Server Agent configured to **use that profile** | `sp_set_sqlagent_properties @use_databasemail = 1, @databasemail_profile = …` (not verified here, W1) | **Not possible with T-SQL** (D5): `mssql-conf set sqlagent.databasemailprofile` or env var `MSSQL_AGENT_EMAIL_PROFILE` (D6, D11) | Fixed: Agent always uses `AzureManagedInstance_dbmail_profile`; Agent settings are read-only | No |
| 5 | An **operator** with an email address | `sp_add_operator` | same | same | Yes, `sqlserver_agent_operator` |
| 6 | The **job** set to notify the operator | `sp_update_job @notify_level_email` | same | same | Yes, `sqlserver_agent_job` |
| 7 | Network: the instance can reach the SMTP host and port, and trusts its TLS certificate | OS trust store | CA in `/var/opt/mssql/security/ca-certificates`, read at startup (D8) | NSG / route table allows outbound 587; public CA | Out of scope (documented) |

So the work is items 1–4. Items 5–6 exist; item 7 stays documentation.

## 1. Decisions

| Topic | Decision |
|---|---|
| Resources | Three new resources: `sqlserver_database_mail_account`, `sqlserver_database_mail_profile`, `sqlserver_agent_mail_settings` (items 1 and 4). **Proposed**, see Q1–Q3. |
| Platforms | Windows, Linux (installed or container) and Managed Instance, detected at apply time: `SERVERPROPERTY('EngineEdition') = 8` → Managed Instance; otherwise `sys.dm_os_host_info.host_platform` (`Windows` / `Linux`). Azure SQL Database: the existing "no SQL Server Agent" error. |
| Agent profile on Linux | The provider can't set it (D5). `sqlserver_agent_mail_settings` **checks** that the effective profile (read with `xp_instance_regread`, which reflects mssql.conf and the env var, D6/D11) is the configured one, and fails the apply with the exact `mssql-conf` / `MSSQL_AGENT_EMAIL_PROFILE` instructions when it isn't. When it is, the provider sends `sp_sqlagent_notify @op_type = N'M'` so Agent reloads without a restart (D7). |
| Agent profile on Managed Instance | `profile_name` must be `AzureManagedInstance_dbmail_profile`; anything else is an apply-time error. Nothing is written. |
| Agent profile on Windows | `sp_set_sqlagent_properties @use_databasemail = 1, @databasemail_profile = …`, which writes the registry and notifies Agent itself. Whether Agent picks it up without a restart needs a Windows run (W1); the docs say to restart Agent. |
| Secrets | `password` (Sensitive, in state) and write-only `password_wo` + `password_wo_version`, exactly like `sqlserver_login`. The password can't be read back (stored in a `sys.credentials` row, D9), so password drift isn't detectable; username drift is. |
| Updates | In place wherever msdb allows: renames by ID, account updates with `@no_credential_change = 1` unless the username or password changes (D10). |
| Import | Not supported, like the other agent resources. **Proposed** (Q5). |
| Alerts, fail-safe operator | Out of scope: Agent alerts don't work on Linux ("AlertEngine has been disabled", D12) or Managed Instance ("Alerts aren't yet supported"), and the fail-safe operator can't be set from T-SQL on Linux (D12). Windows-only follow-up (section 8). |
| SMTP authentication | Basic (username/password) and anonymous. Windows authentication (`@use_default_credentials`) is left out: it only works on Windows with a domain account. |

## 2. Resource schemas

### `sqlserver_database_mail_account`
ID: `sqlserver://host:port/database_mail_account/<account_id>`

```hcl
resource "sqlserver_database_mail_account" "smtp" {
  name            = "acs"
  email_address   = "sql-alerts@contoso.com"
  display_name    = "SQL Agent (prod-mi)"
  mailserver_name = "smtp.azurecomm.net"
  port            = 587
  enable_ssl      = true
  username        = "prod-mi-alerts"          # the ACS "SMTP username" resource
  password_wo     = var.acs_client_secret     # the Entra application's client secret
  password_wo_version = 1
}
```

| Attribute | Kind | Notes |
|---|---|---|
| `name` | Required | Renamed in place (`sysmail_update_account_sp @account_id, @account_name`). Case-insensitive unique (D9). |
| `email_address` | Required | The From address. |
| `display_name` | Optional | default `""` |
| `replyto_address` | Optional | default `""` |
| `description` | Optional | default `""` |
| `mailserver_name` | Required | SMTP host. |
| `port` | Optional | default `25` (SQL Server's default); the docs recommend 587. |
| `enable_ssl` | Optional | default `false`. **STARTTLS only**: implicit TLS (port 465) doesn't work (D8). |
| `username` | Optional | Empty = anonymous SMTP. Drift is detected (`sysmail_server.username`). |
| `password` | Optional, Sensitive | Conflicts with `password_wo`. |
| `password_wo` | Optional, Sensitive, WriteOnly | Plus `password_wo_version` (Optional int) to trigger a rotation. |
| `account_id` | Computed | `sysmail_account.account_id` |

- **Create:** `sysmail_add_account_sp … @account_id OUTPUT`.
- **Read:** `sysmail_account` joined with `sysmail_server` (`servertype = 'SMTP'`) by `account_id`. Not found → clear the ID.
- **Update:**
  - nothing secret changed → `sysmail_update_account_sp @account_id, …, @no_credential_change = 1`. Without that flag the procedure **drops the stored username and password** when `@username` is NULL (D10);
  - `username`, `password` or `password_wo_version` changed → the same call with `@username` and `@password` (the write-only value is read from the raw config during apply);
  - the procedure treats NULL as "keep the current value", so clearing `display_name`, `replyto_address` or `description` sends `''` (to be checked for `replyto_address`, which is validated as an address).
- **CustomizeDiff:** with `password_wo`, changing `username` requires bumping `password_wo_version` (the procedure needs both together); a username without any password is an error.
- **Delete:** `sysmail_delete_account_sp @account_id`. It also removes the account from every profile and drops its credential (D9).
- Not exposed: `timeout` (`sysmail_update_account_sp` can't change it), `mailserver_type` (always SMTP), `use_default_credentials`.

### `sqlserver_database_mail_profile`
ID: `sqlserver://host:port/database_mail_profile/<profile_id>`

```hcl
resource "sqlserver_database_mail_profile" "agent" {
  name          = "AzureManagedInstance_dbmail_profile"
  description   = "Used by SQL Server Agent"
  account_names = [sqlserver_database_mail_account.smtp.name] # failover order
}
```

| Attribute | Kind | Notes |
|---|---|---|
| `name` | Required | Renamed in place (`sysmail_update_profile_sp @profile_id, @profile_name`). |
| `description` | Optional | default `""` |
| `account_names` | Optional, ordered list | Position = `sequence_number` (1, 2, …): Database Mail tries the accounts in this order. |
| `profile_id` | Computed | |

- **Create:** `sysmail_add_profile_sp … @profile_id OUTPUT`, then `sysmail_add_profileaccount_sp` per account, in one transaction.
- **Read:** `sysmail_profile` + `sysmail_profileaccount` joined to `sysmail_account`, ordered by `sequence_number`.
- **Update:** diff the memberships: `sysmail_delete_profileaccount_sp` for removed accounts, `sysmail_update_profileaccount_sp` for moved ones, `sysmail_add_profileaccount_sp` for new ones, in one transaction.
- **Delete:** `sysmail_delete_profile_sp`. SQL Server allows deleting the profile Agent uses, without any warning, after which notifications silently stop (D9). Documented.
- Public / private profile security (`sysmail_add_principalprofile_sp`) is a follow-up: Agent doesn't need it, only `sp_send_dbmail` callers without `@profile_name` do.

### `sqlserver_agent_mail_settings`
One per server. ID: `sqlserver://host:port/agent_mail_settings`

```hcl
resource "sqlserver_agent_mail_settings" "this" {
  profile_name = sqlserver_database_mail_profile.agent.name
}
```

| Attribute | Kind | Notes |
|---|---|---|
| `profile_name` | Required | The Database Mail profile Agent uses. |
| `platform` | Computed | `windows`, `linux` or `managed_instance`; helps explain the behaviour below in plans and errors. |

- **Create / Update:**
  1. Enable `Database Mail XPs` if it's off (`show advanced options` on, set, `RECONFIGURE`, `show advanced options` back to what it was; D1). Without it Agent can't send (D4) and `sp_send_dbmail` fails.
  2. Per platform:
     - **Windows:** `sp_set_sqlagent_properties @use_databasemail = 1, @databasemail_profile = @profile_name`.
     - **Linux:** read the effective profile; if it differs, fail with: "SQL Server Agent on Linux reads its mail profile from mssql.conf: run `mssql-conf set sqlagent.databasemailprofile <name>`, or set `MSSQL_AGENT_EMAIL_PROFILE=<name>` on the container, then apply again". If it matches, `sp_sqlagent_notify @op_type = N'M'`.
     - **Managed Instance:** error unless `profile_name = "AzureManagedInstance_dbmail_profile"` (case-insensitive).
  3. Warn (diagnostic, not error) when the profile doesn't exist or has no accounts: SQL Server accepts that, and mail then fails silently.
- **Read:** `Database Mail XPs` (`sys.configurations.value_in_use`) and the effective profile name (`xp_instance_regread`; on Managed Instance the fixed name). A server with XPs switched off, or a different Agent profile, shows as a diff.
- **Delete:** removes it from state only and leaves the server as it is: other code may use Database Mail, and on Linux and Managed Instance there's nothing to undo. **Proposed** (Q3).

### Existing resources: small changes
- `agent_operator.md` "Email Delivery": replace the hand-written T-SQL with the three resources; keep the troubleshooting queries.
- `agent_operator.md`: deleting an operator (outside Terraform, or a replace) silently sets `notify_level_email` to `never` on every job that used it (D9); the job shows a diff on the next plan.
- `agent_job.md`: `notify_level_eventlog` has no effect on Managed Instance ("EventLog isn't supported"); the value is only stored.

## 3. Code layout (following the existing layering)

**New files**
- `sqlserver/model/database_mail.go`: `DatabaseMailAccount`, `DatabaseMailProfile`, `AgentMailSettings`.
- `sql/database_mail.go`: account and profile CRUD on `*Connector`, using `agentExec` for the deadlock retry like the other msdb writers.
- `sql/agent_mail_settings.go`: platform detection, `Database Mail XPs`, read/set/notify of the Agent profile.
- `sqlserver/resource_database_mail_account.go`, `resource_database_mail_profile.go`, `resource_agent_mail_settings.go` (+ `_test.go` each).
- `docs/resources/database_mail_account.md`, `database_mail_profile.md`, `agent_mail_settings.md`.
- `examples/agent_email/main.tf` (section 5).

**Changed files**
- `sqlserver/const.go`, `sqlserver/provider.go` (register), `sqlserver/provider_test.go` (`TestConnector` getters).
- `sql/agent_common.go`: an `engineEditionManagedInstance = 8` constant next to the Azure SQL Database one.
- `test-fixtures/local` (and `docker-compose/docker-compose.yml`): an SMTP sink (section 4).
- `docs/resources/agent_operator.md`, `agent_job.md`, `docs/index.md`, `README.md`; version bump in `Makefile`.

## 4. Tests
- **Unit tests:** `password` / `password_wo` validation, the username-change-needs-version rule, `account_names` → sequence mapping, profile-membership diffing, the Managed Instance name check, and the Linux error text.
- **Test fixture:** add an SMTP sink with an HTTP API to `test-fixtures/local` (Mailpit, `axllent/mailpit`, accepting any SMTP AUTH), in a Docker network with SQL Server, and start SQL Server with **`MSSQL_AGENT_EMAIL_PROFILE=AzureManagedInstance_dbmail_profile`**. A profile created later is used without any reload (D11), which is exactly the Managed Instance behaviour, so the same tests and example cover both.
- **Acceptance tests** (`TestAcc…_Local_*`):
  1. Account: create, every attribute read back, empty plan; in-place update of each attribute; rename keeps `account_id`.
  2. Account credentials: password → `password_wo` → bumped version; changing a non-secret attribute keeps the stored username/credential (guards D10); removing `username` makes it anonymous.
  3. Profile: create with two accounts, reorder, remove one, rename; account deleted outside Terraform shows as a diff.
  4. Agent mail settings: `Database Mail XPs` off → enabled; a mismatching `profile_name` on Linux fails with the mssql-conf message.
  5. **End to end:** account (Mailpit) + profile + settings + operator + job with a failing step and `notify_level_email = "on_failure"`; start the job, poll Mailpit's API for `[Failed] SQL Server Job System: '<job>'` addressed to the operator.
- **Needs other environments** (run once before release, like C4 for the credential): W1 on Windows, M1–M3 on a Managed Instance (section 9).
- **STARTTLS with a private CA** stays manual: it needs the CA in `/var/opt/mssql/security/ca-certificates` before SQL Server starts (D8), the same fixture work as the Azurite follow-up in `PLAN_SQL_CREDENTIAL.md`; worth doing together.

## 5. Docs and examples
- `database_mail_account.md`:
  - **STARTTLS only**: `enable_ssl = true` with port 587 (or 25); implicit TLS on 465 doesn't work (D8);
  - authentication is `AUTH LOGIN` with username/password (D8); OAuth isn't supported by Database Mail, so **Exchange Online SMTP AUTH is a poor choice**: Microsoft is retiring basic authentication for SMTP client submission (dates have moved; check the current status). Recommended relays: **Azure Communication Services Email** (`smtp.azurecomm.net:587`, username = the ACS "SMTP username" resource, password = the Entra application's client secret, which expires: rotate with `password_wo_version`), SendGrid, or an on-premises relay;
  - the password can't be read back, so changes made outside Terraform aren't detected;
  - Linux: a private CA must be in `/var/opt/mssql/security/ca-certificates` before SQL Server starts; an untrusted certificate fails with the opaque "The handle specified is invalid" in `sysmail_event_log` (D8);
  - Managed Instance: the subnet NSG/route table must allow outbound traffic to the SMTP host and port.
- `database_mail_profile.md`: account order is failover order; Managed Instance name; deleting the Agent profile silently stops notifications.
- `agent_mail_settings.md`: the per-platform table from section 0 (rows 1 and 4), the Linux mssql-conf / env var instructions (container: `MSSQL_AGENT_EMAIL_PROFILE`), troubleshooting (`sysjobhistory` "Failed to notify", `sysmail_allitems`, `sysmail_event_log`, the Agent log), a test query with `sp_send_dbmail`, and permissions (sysadmin: `sysmail_*` procedures, `sp_configure`, `sp_set_sqlagent_properties`).
- `examples/agent_email/main.tf`: account → profile named `AzureManagedInstance_dbmail_profile` → settings → operator → job with `notify_level_email = "on_failure"`. Using the Managed Instance name everywhere makes one example work on all targets (Linux with the env var or mssql-conf).

## 6. Implementation order
1. Spikes D1–D12 done (section 9). W1 and M1–M3 need environments we don't have here.
2. Model and SQL layer (account, profile, settings).
3. `sqlserver_database_mail_account` with secrets and the D10 update rule.
4. `sqlserver_database_mail_profile`.
5. `sqlserver_agent_mail_settings` with platform detection.
6. Test fixture (Mailpit, env var), unit and acceptance tests including the end-to-end mail test.
7. Docs, example, operator/job doc changes, `docs/index.md`, README, version bump.

## 7. Defaults chosen without asking
- `port` defaults to 25 like `sysmail_add_account_sp`, not 587; the docs and example use 587.
- `account_names` is an ordered list (position = sequence number) instead of blocks with explicit sequence numbers: equal sequence numbers aren't useful, and a list can't have gaps.
- `sqlserver_agent_mail_settings` turns `Database Mail XPs` on, because Agent mail can't work without it; it never turns it off.
- `show advanced options` is restored to its previous value after changing `Database Mail XPs`.
- Version bump to the next patch version (0.5.1 → 0.5.2), following the `sqlserver_credential` precedent.

## 8. Follow-ups (out of scope)
- **Agent alerts** (`sqlserver_agent_alert` + operator notifications) and the **fail-safe operator**: Windows only (D12, Managed Instance docs).
- Profile security: public/private profiles and the default profile (`sysmail_add_principalprofile_sp`), for `sp_send_dbmail` callers.
- Database Mail system parameters (`sysmail_configure_sp`: `AccountRetryAttempts`, `AccountRetryDelay`, `MaxFileSize`, `ProhibitedExtensions`, `LoggingLevel`).
- Windows authentication for SMTP (`use_default_credentials`).
- Pager and net send: deprecated and unsupported on Managed Instance; not planned.
- STARTTLS acceptance test with a private CA, together with the Azurite fixture work.

## 9. Spike results (2026-10-21, SQL Server 2022 RTM-CU27-GDR 16.0.4305.5, Developer, Linux in Docker)
SMTP: a local sink (`aiosmtpd`, ports for plain/anonymous, STARTTLS + AUTH with a self-signed certificate, implicit TLS,
plus a logging TCP proxy). Mailpit couldn't be pulled in this environment (Docker Hub rate limit, ghcr blocked); it's still
the proposal for the fixture.

| # | Question | Result |
|---|---|---|
| D1 | `Database Mail XPs` | Off by default, and an **advanced** option: `sp_configure` fails with 15123 unless `show advanced options` is 1. Persists across restarts. |
| D2 | Send with account + profile, no Agent involved | Works on Linux: `sp_send_dbmail` → `sysmail_allitems.sent_status = sent`, mail received. |
| D3 | Job notification without an Agent profile | Job fails normally; history says `NOTE: Failed to notify 'dba' via email.`; Agent log: "No mail profile defined", error 14607 "profile name is not valid". A default public profile isn't a fallback (verified after a restart). |
| D4 | Job notification with `Database Mail XPs` off | Same silent "Failed to notify". Accounts and profiles can still be created while it's off. |
| D5 | `sp_set_sqlagent_properties` on Linux | Returns success but **changes nothing**: it writes with `xp_instance_regwrite`, which returns 1 on Linux and doesn't persist (also not for other settings such as `JobHistoryMaxRows`). Same for the fail-safe operator (`sp_MSsetalertinfo`). **The Agent mail profile can't be set with T-SQL on Linux.** |
| D6 | `mssql-conf set sqlagent.databasemailprofile` | Writes `[sqlagent] databasemailprofile` to mssql.conf; `xp_instance_regread … 'DatabaseMailProfile'` returns the new value immediately, so the provider can read the effective setting. |
| D7 | Does Agent need a restart after D6? | No: after `EXEC msdb.dbo.sp_sqlagent_notify @op_type = N'M'` (what `sp_set_sqlagent_properties` sends on Windows) the next job failure was emailed. Without the notify, Agent kept the old value. |
| D8 | TLS and authentication | STARTTLS with `enable_ssl = 1` works once the CA is in `/var/opt/mssql/security/ca-certificates` **and SQL Server has restarted**; with an untrusted certificate the error is "The handle specified is invalid". Authentication is `AUTH login <base64 user>` then the password (AUTH LOGIN, lowercase mechanism, which RFC 4954 allows; the test sink first rejected it). Implicit TLS (port 465): the mail ends as `failed` ("connection forcibly closed"). |
| D9 | msdb behaviour that affects the resources | Names are unique case-insensitively (`PTLS` clashes with `ptls`). The password is stored as a `sys.credentials` row (`sysmail_server.credential_id`, identity = username) and can't be read. Deleting an account removes its profile memberships and its credential. Deleting the profile Agent uses succeeds silently. `sp_delete_operator` sets `notify_level_email = 0` on jobs that used the operator. |
| D10 | `sysmail_update_account_sp` semantics | NULL means "keep" for most parameters, but a call without `@username` **removes the username and credential** (seen: an update of only `@port` turned an authenticated account anonymous). `@no_credential_change = 1` keeps them. `timeout` can't be changed by the procedure. |
| D11 | `MSSQL_AGENT_EMAIL_PROFILE` env var (containers) | Works: the profile name is visible through `xp_instance_regread` without any mssql.conf, and a profile **created after startup** with that name was used for the next job notification without any reload. |
| D12 | Alerts on Linux | Agent log at startup: "Warning: The AlertEngine engine has been disabled"; a severity 16 `RAISERROR … WITH LOG` didn't fire a severity-16 alert (`occurrence_count` stayed 0). |

**Open, need other environments:**

| # | Question | Needs |
|---|---|---|
| W1 | Windows: does `sp_set_sqlagent_properties @databasemail_profile` take effect without restarting Agent (it sends the same notify as D7)? Microsoft's docs say to restart Agent. | A Windows SQL Server. |
| M1 | Managed Instance: is `Database Mail XPs` off by default, and can the provider enable it with `sp_configure` (needs `show advanced options`)? Microsoft's job-automation docs show the `sp_configure` call. | A Managed Instance. |
| M2 | Managed Instance: end-to-end notification with Azure Communication Services Email over 587/STARTTLS, and the NSG rule needed. | A Managed Instance + ACS Email resource. |
| M3 | Managed Instance: can a non-sysadmin read `sys.configurations` and is `sys.dm_os_host_info` available (only used for the platform check)? | A Managed Instance. |

Sources for the Managed Instance and Linux behaviour: Microsoft's "T-SQL differences between SQL Server and Azure SQL
Managed Instance" (SQL Server Agent and Database Mail sections), "Automate management tasks in Azure SQL Managed Instance
using SQL Agent jobs" (job notifications) and "Database Mail and email alerts with SQL Server Agent on Linux"
(read from the MicrosoftDocs/sql-docs repository, 2026-10-21).

## 10. Questions for you
1. **Resource split** (Q1): three resources as above, or fold `Database Mail XPs` and the Agent profile into the profile resource
   (e.g. `used_by_agent = true`)? Recommendation: keep `sqlserver_agent_mail_settings` separate; it's a per-server singleton
   with platform-specific behaviour, while profiles are ordinary objects you can have several of.
2. **`Database Mail XPs`** (Q2): managed inside `sqlserver_agent_mail_settings` (recommended, it's always needed for Agent mail),
   or a generic `sqlserver_server_configuration` resource (`sp_configure` name/value) that could later cover other options too?
3. **Delete behaviour of `sqlserver_agent_mail_settings`** (Q3): leave the server as it is (recommended), or on Windows switch
   Agent mail off (`@use_databasemail = 0`)?
4. **Linux** (Q4): is failing the apply with mssql-conf / env var instructions acceptable, given T-SQL can't change it? The
   alternative is a warning only, which lets an apply succeed while mail won't work.
5. **Import** (Q5): skip, like the other agent resources, or support it here (accounts by name; the password then has to be
   supplied before the first apply)?
6. **Test environments**: can you run W1 (Windows) and M1–M3 (a Managed Instance, plus an ACS Email resource) before release?
