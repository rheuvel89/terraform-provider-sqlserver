# Plan: `sqlserver_credential` resource for `terraform-provider-sqlserver`

Goal: manage SQL Server **server-level credentials** (`CREATE CREDENTIAL`) in Terraform. The immediate use case is backup to and
restore from Azure Blob Storage on Azure SQL Managed Instance with the instance's managed identity, run by a
`sqlserver_agent_job`. Without this resource the credential has to be created inside a job step, where Terraform can't see,
plan or delete it.

See https://learn.microsoft.com/en-us/sql/t-sql/statements/create-credential-transact-sql.

## 1. Decisions

| Topic | Decision |
|---|---|
| Scope | **Server-level credentials only** (`CREATE CREDENTIAL`, `sys.credentials`). Database-scoped credentials (`CREATE DATABASE SCOPED CREDENTIAL`, needed on Azure SQL Database and for external data sources) are a different statement, need a database master key, and live in another catalog view; they're a possible follow-up resource (section 8). |
| Platforms | SQL Server (on-premises or VM) and Azure SQL Managed Instance. Azure SQL Database has no server-level credentials and fails with a clear error, like the agent resources. |
| Identity | `name` is the ID. Names are case-insensitive and can't be renamed (spike C1), so `name` is ForceNew. |
| Secret | Optional. Either `secret` (Sensitive, stored in state) or the write-only `secret_wo` + `secret_wo_version`, mirroring `password` / `password_wo` on `sqlserver_login`. Neither is required: a managed identity credential has no secret. |
| Secret drift | Not detectable: SQL Server never exposes the secret (`sys.credentials` has no secret column; only the dedicated admin connection can see that one exists, spike C2). Documented. |
| Updates | `identity` and the secret change in place with `ALTER CREDENTIAL`. **Every `ALTER` resends the secret**, because `ALTER CREDENTIAL` without `SECRET` deletes the stored secret (spike C3). |
| Cryptographic providers (EKM) | Out of scope (`FOR CRYPTOGRAPHIC PROVIDER`). |
| Import | Supported, by name, like `sqlserver_login` and `sqlserver_database`. The secret can't be imported; after an import, set `secret_wo` (or `secret`) and apply. |

## 2. Resource schema

### `sqlserver_credential`
ID: `sqlserver://host:port/credential/<url-escaped name>` (names are often URLs, so the name is path-escaped like step names).

```hcl
# Backup to URL with the managed identity of an Azure SQL Managed Instance (or an Arc-enabled SQL Server 2022)
resource "sqlserver_credential" "backup" {
  name     = "https://backupsa.blob.core.windows.net/sql-backups" # the container URL, no trailing slash
  identity = "Managed Identity"
}

# Backup to URL with a SAS token
resource "sqlserver_credential" "backup_sas" {
  name              = "https://backupsa.blob.core.windows.net/sql-backups"
  identity          = "SHARED ACCESS SIGNATURE"
  secret_wo         = ephemeral.azurerm_storage_account_sas.backup.sas # without the leading '?'
  secret_wo_version = 1
}
```

| Attribute | Kind | Notes |
|---|---|---|
| `name` | Required, ForceNew | Up to 128 characters (`sysname`). |
| `identity` | Required | Up to 4000 characters. Changed in place. |
| `secret` | Optional, Sensitive | Stored in state. Conflicts with `secret_wo`. |
| `secret_wo` | Optional, Sensitive, Write-only | Never stored. Requires `secret_wo_version`. Needs Terraform 1.11+. |
| `secret_wo_version` | Optional | Bump it to rotate `secret_wo`. |
| `credential_id` | Computed | `sys.credentials.credential_id`. |

- **CustomizeDiff** (like `resourceLoginCustomizeDiff`, since `ConflictsWith`/`RequiredWith` don't work on write-only attributes):
  - `secret` and `secret_wo` can't both be set;
  - `secret_wo_version` is required with `secret_wo`, and not allowed without it;
  - plan-time checks that catch the common mistakes (errors unless noted):
    - `identity = "SHARED ACCESS SIGNATURE"` without a secret;
    - `identity = "Managed Identity"` with a secret: **warning**, since a managed identity needs no secret (not verified whether SQL Server rejects or ignores it);
    - a SAS secret starting with `?` (SQL Server rejects it at backup time, not at `CREATE`). Only checkable for `secret`, not `secret_wo`, since write-only values aren't in the plan; for `secret_wo` the check runs at apply time;
    - a `name` that starts with `https://` and ends with `/`: **warning**, since `BACKUP TO URL` only finds a credential whose name matches the container URL exactly.
- **Create:** dynamic `CREATE CREDENTIAL` built in T-SQL with `QUOTENAME(@name)` for the name and `N'' + REPLACE(@value, '''', '''''') + ''` for identity and secret. **Not `QUOTENAME(@x, '''')`:** it returns NULL for input over 128 characters, and SAS tokens are longer, so `EXEC (@sql)` would silently run nothing.
- **Read:** `SELECT credential_id, credential_identity FROM sys.credentials WHERE name = @name`. Not found → clear the ID. `secret` stays as in state.
- **Update:** `ALTER CREDENTIAL ... WITH IDENTITY = ..., SECRET = ...` whenever `identity`, `secret` or `secret_wo_version` changes, always with the current secret: `secret` from state/config, `secret_wo` from the raw config of this apply (available in every apply where it's set), or no `SECRET` clause when neither is set.
- **Delete:** `IF EXISTS ... DROP CREDENTIAL`. SQL Server refuses while the credential is used by a SQL Server Agent proxy or mapped to a login (`ALTER LOGIN ... WITH CREDENTIAL`); the error is passed through with the credential name.
- **Import:** by ID or name; reads `identity`; secrets stay empty.

## 3. Code layout (following the existing layering)

**New files**
- `sqlserver/model/credential.go`: `Credential{ID int; Name, Identity string}`.
- `sql/credential.go`: `GetCredential`, `CreateCredential(name, identity, secret *string)`, `UpdateCredential(...)`, `DeleteCredential` on `*Connector`, in the style of `sql/login.go`. A `nil` secret means "no SECRET clause".
- `sqlserver/resource_credential.go`: `CredentialConnector` interface, schema, CustomizeDiff, CRUD, import, `getCredentialID`, `getCredentialConnector`.
- `sqlserver/resource_credential_test.go`, `resource_credential_import_test.go`.
- `docs/resources/credential.md`.
- `examples/backup_to_url/main.tf`: credential plus an agent job that backs up every user database to the container (see section 5).

**Changed files**
- `sqlserver/const.go`: property names.
- `sqlserver/provider.go`: register `sqlserver_credential`.
- `sqlserver/provider_test.go`: `GetCredential` on `TestConnector`.
- `sql/agent_common.go`: `EnsureAgentAvailable` is generalised (or a sibling `EnsureNotAzureSQLDatabase` is added) so the credential resource gives the same clear Azure SQL Database error.
- `docs/index.md`, `README` (no resource list, so unchanged), version bump in `Makefile` (0.5.0 → 0.6.0).

## 4. Tests
- **Unit tests:**
  - the CustomizeDiff rules (both secrets, version without `secret_wo`, SAS without secret, managed identity with secret, `?` prefix, trailing-slash warning);
  - quoting: a name, identity and secret containing `'`, `]` and more than 128 characters.
- **Acceptance tests** (`TestAccCredential_Local_*`, Docker SQL Server; creating a credential doesn't contact the storage account, so no Azure is needed):
  1. Managed identity credential named after a container URL: create, read back, empty plan.
  2. SAS credential with `secret`: create, change the secret, change the identity; check `credential_identity` in `sys.credentials`.
  3. `secret_wo` + `secret_wo_version`: create, bump the version to rotate, and **check through the dedicated admin connection that the secret is still set after an identity-only change** (spike C3: a missing `SECRET` would delete it). The DAC check is test-only.
  4. A secret longer than 128 characters with quotes in it (covers the `QUOTENAME` trap).
  5. Deleted outside Terraform → recreated.
  6. Import by name.
  7. Plan-time errors (`ExpectError`) for the CustomizeDiff rules.
- Not testable locally: an actual `BACKUP TO URL` (needs Azure Storage, and on MI a managed identity). Documented; the example in section 5 is applied once against a real Managed Instance before release if one is available.

## 5. Docs and examples
- `docs/resources/credential.md`:
  - examples for managed identity, SAS (with `secret_wo` and an ephemeral SAS), and S3-compatible storage (`identity = "S3 Access Key"`, SQL Server 2022);
  - the name must match the container URL exactly, without a trailing slash, for `BACKUP TO URL` to find it;
  - managed identity setup on Managed Instance: the instance's identity needs **Storage Blob Data Contributor** on the container (or account); with user-assigned identities, the instance's *primary* identity is used;
  - SAS: no leading `?`; needs read, write, list, delete (and add/create) permissions on the container;
  - secrets can't be read back, so changes made outside Terraform aren't detected; rotate with `secret_wo_version`;
  - ordering: reference `sqlserver_credential.x.name` in the job step's `command` (or use `depends_on`) so the credential exists before the job runs a backup;
  - permissions: `ALTER ANY CREDENTIAL`.
- `examples/backup_to_url/main.tf`: credential with managed identity, an agent job with one step that loops over the user databases and runs `BACKUP DATABASE ... TO URL = '<container>/<db>/<db>_<timestamp>.bak' WITH COPY_ONLY, COMPRESSION, CHECKSUM`, a nightly schedule, and an operator emailed on failure. The step's notes cover the Managed Instance rules that come up with backups:
  - only `COPY_ONLY` full backups are allowed (MI owns the backup chain);
  - databases encrypted with **service-managed TDE** can't be backed up by users (customer-managed TDE or no TDE is needed);
  - large databases need several URLs (striping) and `MAXTRANSFERSIZE = 4194304`, since one block blob holds about 195 GB.

  These MI rules come from Microsoft's documentation; they couldn't be verified here without a Managed Instance, and the example says so.
- `docs/index.md`: list the resource and the example.

## 6. Implementation order
1. Spikes (done for C1–C3, see section 9; C4 still open).
2. Model, SQL layer and the quoting unit test.
3. Resource with `secret`, then `secret_wo`, CustomizeDiff and import.
4. Acceptance tests.
5. Docs, the backup example, `docs/index.md`, version bump.

## 7. Defaults chosen without asking
- `identity` is Required: SQL Server requires `IDENTITY` in `CREATE CREDENTIAL`.
- Secrets have no default; `secret` and `secret_wo` are optional because managed identity credentials have none.
- No `ForceNew` on `identity` or the secret: `ALTER CREDENTIAL` changes both in place.
- A SAS secret starting with `?` and a SAS credential without a secret are errors: neither can ever work. A managed identity credential with a secret and a URL name with a trailing slash are warnings.

## 8. Follow-ups (out of scope)
- `sqlserver_database_scoped_credential` (needs a `sqlserver_database_master_key` too): for Azure SQL Database, external data sources and `OPENROWSET`.
- Login-to-credential mapping (`ALTER LOGIN ... WITH CREDENTIAL`) and SQL Server Agent proxies (only relevant for non-T-SQL job steps, which the agent resources don't support).

## 9. Spike results (2026-10-13, SQL Server 2022 in Docker)

| # | Question | Result |
|---|---|---|
| C1 | Names: case, rename, length | Case-insensitive (`SAS_CRED` clashes with `sas_cred`). No rename syntax. `name` is `sysname` (128 characters); `credential_identity` holds up to 4000 characters. |
| C2 | Can the provider see whether a secret is set? | No. `sys.credentials` only has `credential_id`, `name`, `credential_identity`, `create_date`, `modify_date`, `target_type`, `target_id`. The secret is only visible in `sys.sysobjvalues` (valclass 28, valnum 2) over the dedicated admin connection, which the provider doesn't use; the tests can. |
| C3 | Does `ALTER CREDENTIAL` without `SECRET` keep the secret? | **No, it deletes it.** After `CREATE ... SECRET` the secret row exists (84 bytes); after `ALTER ... WITH IDENTITY` only, it's gone; `ALTER ... SECRET` adds it back. |
| C4 | `Managed Identity` and `BACKUP TO URL` on Managed Instance | Creating a `Managed Identity` credential works on SQL Server 2022 in Docker (it's only stored). Whether the backup itself works needs a real Managed Instance and storage account. **Open.** |
