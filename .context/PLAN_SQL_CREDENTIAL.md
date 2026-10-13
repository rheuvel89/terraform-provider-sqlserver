# Plan: `sqlserver_credential` resource (preview) for `terraform-provider-sqlserver`

Goal: manage the SQL Server credential that **backup to and restore from Azure Blob Storage with a managed identity** needs,
so a `sqlserver_agent_job` can back up databases on Azure SQL Managed Instance to a storage account. Without this resource
the credential has to be created inside a job step, where Terraform can't see, plan or delete it.

This is deliberately a **preview with provisional support**: only managed identity credentials, the minimum for that goal.
Other credential types (SAS tokens, storage keys, S3) need secrets and are follow-ups (section 8). The resource is designed
so they can be added later without breaking existing configurations.

See https://learn.microsoft.com/en-us/sql/t-sql/statements/create-credential-transact-sql.

## 1. Decisions

| Topic | Decision |
|---|---|
| Scope | Server-level credentials (`CREATE CREDENTIAL`, `sys.credentials`) with `IDENTITY = 'Managed Identity'` only. No secrets. Decided 2026-10-13: "as slim as possible", clearly marked as not a full implementation. |
| Signalling "preview" | `identity` only accepts `"Managed Identity"`; anything else fails at plan time with an error saying other credential types aren't supported yet. The docs page is titled "sqlserver_credential (preview)" and starts with a note on what's supported; the schema descriptions say the same. |
| Why `identity` is still an attribute | So configurations stay valid when more identities are supported. It's Required (no default), so a config always says which kind of credential it is. |
| Platforms | Azure SQL Managed Instance (the goal) and SQL Server 2022 enabled by Azure Arc, which supports the same managed identity backups. Plain SQL Server accepts the credential (it's only stored), but `BACKUP TO URL` with a managed identity won't work there; documented. Azure SQL Database has no server-level credentials: clear error, like the agent resources. |
| Identity in the resource | `name` is the ID. Names are case-insensitive and can't be renamed (spike C1). |
| Updates | None: `name` and `identity` are both ForceNew, and nothing else is settable, so there's no Update function. |
| Import | Not supported for now (decided 2026-10-13). |
| Database-scoped credentials | Out of scope (decided 2026-10-13). |

## 2. Resource schema

### `sqlserver_credential`
ID: `sqlserver://host:port/credential/<url-escaped name>` (the name is usually a URL, so it's path-escaped like step names).

```hcl
resource "sqlserver_credential" "backup" {
  name     = "https://backupsa.blob.core.windows.net/sql-backups" # the container URL, no trailing slash
  identity = "Managed Identity"
}
```

| Attribute | Kind | Notes |
|---|---|---|
| `name` | Required, ForceNew | Up to 128 characters (`sysname`). For backup to URL: the container URL. Plan-time **warning** when it starts with `https://` and ends with `/`, since `BACKUP TO URL` only finds a credential whose name matches the container URL exactly. |
| `identity` | Required, ForceNew | Only `"Managed Identity"` for now (case-insensitive match, stored as written). |
| `credential_id` | Computed | `sys.credentials.credential_id`. |

- **Create:** `CREATE CREDENTIAL` built in T-SQL with `QUOTENAME(@name)`; the identity is the fixed literal `'Managed Identity'`, so no user text is quoted into the statement apart from the name.
- **Read:** `SELECT credential_id, credential_identity FROM sys.credentials WHERE name = @name`. Not found → clear the ID. A different `credential_identity` (changed outside Terraform) shows as a diff and replaces the credential.
- **Delete:** `IF EXISTS ... DROP CREDENTIAL`. SQL Server refuses while a SQL Server Agent proxy or a login uses the credential; that error is passed through with the credential name.
- **No secrets in this version.** For later: `ALTER CREDENTIAL` without `SECRET` deletes the stored secret (spike C3), and secrets longer than 128 characters can't be quoted with `QUOTENAME(@x, '''')`, which returns NULL. Both matter as soon as secrets are added.

## 3. Code layout (following the existing layering)

**New files**
- `sqlserver/model/credential.go`: `Credential{ID int; Name, Identity string}`.
- `sql/credential.go`: `GetCredential`, `CreateCredential(name)`, `DeleteCredential` on `*Connector`, in the style of `sql/database.go`.
- `sqlserver/resource_credential.go`: `CredentialConnector` interface, schema, CRUD, `getCredentialID`, `getCredentialConnector`.
- `sqlserver/resource_credential_test.go`.
- `docs/resources/credential.md`.
- `examples/backup_to_url/main.tf` (section 5).

**Changed files**
- `sqlserver/const.go`: property names.
- `sqlserver/provider.go`: register `sqlserver_credential`.
- `sqlserver/provider_test.go`: `GetCredential` on `TestConnector`.
- `sql/agent_common.go`: reuse the Azure SQL Database check (`EnsureAgentAvailable`) or add a general sibling with a message that fits credentials.
- `docs/index.md`; version bump in `Makefile` (0.5.0 → 0.6.0).

## 4. Tests
- **Unit tests:** the `name` warning (trailing slash on a URL; none without) and the `identity` validation.
- **Acceptance tests** (`TestAccCredential_Local_*`, Docker SQL Server; creating the credential doesn't contact storage, so no Azure is needed):
  1. A managed identity credential named after a container URL: create, check `sys.credentials`, empty plan afterwards.
  2. Changing the name replaces the credential (the old one is gone).
  3. Deleted outside Terraform → recreated.
  4. Plan-time error for another identity, e.g. `"SHARED ACCESS SIGNATURE"`.
  5. An agent job step that references `sqlserver_credential.x.name` in its command, to show the ordering works (the step only stores the command).
- Not testable locally: an actual `BACKUP TO URL` with a managed identity (needs a Managed Instance and a storage account). Documented; the example in section 5 should be applied once against a real Managed Instance before release.

## 5. Docs and examples
- `docs/resources/credential.md`, titled "sqlserver_credential (preview)":
  - a note at the top: preview, managed identity credentials only, for backup to and restore from Azure Blob Storage; other credential types aren't supported yet;
  - the name must be the container URL, exactly, without a trailing slash;
  - no secret: Azure checks access against the instance's identity. With user-assigned identities, Managed Instance uses its **primary** identity (`primary_user_managed_identity_id` in azurerm); that identity needs **Storage Blob Data Contributor** on the container or storage account;
  - networking: if the storage account limits public access, the instance's subnet needs a route to it (private endpoint or service endpoint);
  - ordering: reference `sqlserver_credential.x.name` in the job step's `command`, so Terraform creates the credential before the step;
  - permissions: `ALTER ANY CREDENTIAL`.
- `examples/backup_to_url/main.tf`: the credential, an agent job with one step that loops over the user databases and runs `BACKUP DATABASE ... TO URL = '<container>/<db>/<db>_<timestamp>.bak' WITH COPY_ONLY, COMPRESSION, CHECKSUM`, a nightly schedule, and an operator emailed on failure. Comments in the example cover the Managed Instance backup rules:
  - only `COPY_ONLY` full backups are allowed (Managed Instance owns the backup chain);
  - databases with **service-managed TDE** can't be backed up by users (customer-managed TDE or no TDE is needed);
  - large databases need several URLs (striping) and `MAXTRANSFERSIZE = 4194304`, since one block blob holds about 195 GB;
  - restoring: `RESTORE DATABASE ... FROM URL` with the same credential.

  These rules come from Microsoft's documentation and couldn't be verified here without a Managed Instance; the example says so.
- `docs/index.md`: list the resource as "(preview)" and the example.

## 6. Implementation order
1. Spikes C1–C3 done (section 9); C4 needs a Managed Instance.
2. Model and SQL layer.
3. Resource, validation and the name warning.
4. Tests.
5. Docs, example, `docs/index.md`, version bump.

## 7. Defaults chosen without asking
- `identity` is Required rather than defaulting to `"Managed Identity"`: a default would be a commitment once other identities are supported.
- The identity match is case-insensitive (`managed identity` is accepted), but the value is written as configured; SQL Server stores the identity as given.
- The trailing-slash check is a warning, not an error: a credential with any name is valid SQL Server, it just won't be found by `BACKUP TO URL`.

## 8. Follow-ups (out of scope)
- **Full credential support:** SAS tokens (`identity = "SHARED ACCESS SIGNATURE"`), storage account keys and S3-compatible storage (`"S3 Access Key"`), with `secret` and the write-only `secret_wo` + `secret_wo_version` like `password` / `password_wo` on `sqlserver_login`. Mind spike C3 (every `ALTER` must resend the secret) and the `QUOTENAME` 128-character limit; secrets also can't be read back, so drift isn't detectable.
- Import of existing credentials.
- `sqlserver_database_scoped_credential` (with a database master key): Azure SQL Database, external data sources, `OPENROWSET`.
- Login-to-credential mapping and SQL Server Agent proxies (only for non-T-SQL job steps, which the agent resources don't support).
- **Azurite in the test fixtures** (decided 2026-10-13: once the preview is implemented and in use in production). Add the Azure Storage emulator to `test-fixtures/local` (and `test-fixtures/all`) so acceptance tests can run real `BACKUP … TO URL` / `RESTORE … FROM URL`. Most useful together with SAS support above, because it then tests provider code end to end: real SAS tokens, quoting of long secrets, and that an identity change resends the secret (a backup that still works after the change proves it, replacing the dedicated-admin-connection check). Setup from spike C5:
  - Azurite (`mcr.microsoft.com/azure-storage/azurite`, `azurite-blob --blobPort 443 --cert … --key … --loose --skipApiVersionCheck`) in a Docker network shared with SQL Server, with the network alias `devstoreaccount1.blob.core.windows.net`;
  - a test root CA and a server certificate for that name (`basicConstraints CA:FALSE`, `extendedKeyUsage serverAuth`, SAN), generated with Terraform's `tls` provider;
  - the root CA copied to `/var/opt/mssql/security/ca-certificates` **before SQL Server starts** (the OS trust store isn't used);
  - the container created with the Azure SDK or Azurite's REST API and Azurite's well-known development key; a container SAS for the credential.

  It can't test managed identity authentication: that needs a real Managed Instance or an Arc-enabled SQL Server.

## 9. Spike results (2026-10-13, SQL Server 2022 in Docker)

| # | Question | Result |
|---|---|---|
| C1 | Names: case, rename, length | Case-insensitive (`SAS_CRED` clashes with `sas_cred`). No rename syntax. `name` is `sysname` (128 characters); `credential_identity` holds up to 4000 characters. |
| C2 | Can the provider see whether a secret is set? | No. `sys.credentials` only has `credential_id`, `name`, `credential_identity`, `create_date`, `modify_date`, `target_type`, `target_id`. The secret is only visible in `sys.sysobjvalues` (valclass 28, valnum 2) over the dedicated admin connection. Not needed in this version. |
| C3 | Does `ALTER CREDENTIAL` without `SECRET` keep the secret? | **No, it deletes it.** After `CREATE ... SECRET` the secret row exists (84 bytes); after `ALTER ... WITH IDENTITY` only, it's gone; `ALTER ... SECRET` adds it back. Not needed in this version (no secrets, no updates). |
| C4 | `Managed Identity` and `BACKUP TO URL` on Managed Instance | Creating a `Managed Identity` credential works on SQL Server 2022 in Docker (it's only stored). Whether the backup itself works needs a real Managed Instance and storage account. **Open.** |
| C5 | Can backup to URL be tested locally with Azurite (the Azure Storage emulator)? | **Yes, with a SAS credential; not with a managed identity.** Setup that worked: Azurite on HTTPS port 443 in a Docker network, reachable from the SQL Server container as `devstoreaccount1.blob.core.windows.net` (network alias), with a certificate for that name signed by a test root CA. SQL Server on Linux only trusts the CA from **`/var/opt/mssql/security/ca-certificates`** (read at startup: "Number of Files Processed … 1"); the OS trust store (`update-ca-certificates`) isn't used, and without the CA there `BACKUP TO URL` fails with OS error 12175 (TLS failure). With a SAS credential (`generate_container_sas` from the Azure SDK, Azurite's well-known development key): a `COPY_ONLY, COMPRESSION, CHECKSUM` backup, a striped backup over two URLs with `MAXTRANSFERSIZE = 4194304`, `RESTORE VERIFYONLY` and a restore to a new database all worked. A `Managed Identity` credential fails with error 37563 "The primary managed identity is not selected for this server", which confirms SQL Server always uses the server's primary managed identity. |

