# sqlserver_credential (preview)

> **Preview.** This resource currently only supports **managed identity** credentials, for backup to and restore from
> Azure Blob Storage on Azure SQL Managed Instance. Other credential types, such as SAS tokens or storage account keys,
> need a secret and aren't supported yet; setting another `identity` fails during `terraform plan`.

The `sqlserver_credential` resource creates a server-level credential (`CREATE CREDENTIAL`). With a managed identity,
`BACKUP DATABASE … TO URL` and `RESTORE DATABASE … FROM URL` authenticate to Azure Storage as the instance's managed
identity: there's no SAS token, key or other secret to store or rotate.

## Example Usage

```hcl
resource "sqlserver_credential" "backups" {
  name     = "https://backupsa.blob.core.windows.net/sql-backups" # the container URL, no trailing slash
  identity = "Managed Identity"
}
```

A complete setup, with the storage container, the role assignment and a nightly backup job for every user database, is
in [`examples/backup_to_url`](../../examples/backup_to_url/main.tf).

## Argument Reference

* `name` - (Required, ForceNew) The name of the credential, up to 128 characters. For backup to URL it must be the
  container URL **exactly**, without a trailing slash: `BACKUP TO URL` looks the credential up by that name. A URL
  ending with `/` gives a warning during `terraform plan`. Names are case-insensitive and can't be changed in place.
* `identity` - (Required, ForceNew) The identity of the credential. Only `"Managed Identity"` is supported for now.

## Attribute Reference

* `credential_id` - The ID of the credential in `sys.credentials`.

## Managed Identity Setup

* **Which identity.** SQL Server uses the instance's **primary** managed identity. That's the system-assigned identity,
  or, when the instance has user-assigned identities, the one set as primary (`primary_user_managed_identity_id` in
  the azurerm provider). The credential can't name another identity.
* **Access.** That identity needs **Storage Blob Data Contributor** on the container or the storage account. Role
  assignments can take a few minutes to apply.
* **Network.** If the storage account limits public network access, the instance's subnet needs a route to it, for
  example a private endpoint or a `Microsoft.Storage` service endpoint with a network rule.
* **Platforms.** Azure SQL Managed Instance, and SQL Server 2022 enabled by Azure Arc with a primary managed identity.
  Plain SQL Server stores the credential, but a backup fails with "The primary managed identity is not selected for
  this server". Azure SQL Database has no server-level credentials; creating one fails with a clear error.

## Using the Credential in a Job

Put the credential's name in the job step's command, so Terraform creates the credential before the step:

```hcl
resource "sqlserver_agent_job_step" "backup" {
  job_id  = sqlserver_agent_job.backup.job_id
  name    = "backup"
  command = "BACKUP DATABASE [sales] TO URL = N'${sqlserver_credential.backups.name}/sales.bak' WITH COPY_ONLY, COMPRESSION, CHECKSUM"
}
```

On Azure SQL Managed Instance only `COPY_ONLY` full backups are allowed, and databases encrypted with service-managed
TDE can't be backed up by users. See the example for these and other rules.

## Permissions

Creating and deleting credentials needs `ALTER ANY CREDENTIAL`. SQL Server refuses to delete a credential that a SQL
Server Agent proxy or a login still uses.

## Import

Import isn't supported.
