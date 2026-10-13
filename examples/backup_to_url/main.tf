#
# Nightly backups of every user database on an Azure SQL Managed Instance to a storage account, authenticated with the
# instance's managed identity: no SAS token or storage key.
#
# sqlserver_credential is a preview and only supports managed identity credentials.
#
# Before applying, check the Managed Instance backup rules at the step below. The backup script was checked against
# SQL Server 2022; run the job once on your instance and check its history before relying on it.
#
terraform {
  required_version = "~> 1.5"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    sqlserver = {
      source  = "rheuvel89/sqlserver"
      version = "0.6.0"
    }
  }
}

provider "azurerm" {
  features {}
}

# Connects as the signed-in Microsoft Entra identity (az login, a pipeline's identity, ...), which must be an
# Entra admin of the instance.
provider "sqlserver" {
  host = var.sql_mi_fqdn
  azuread_default_chain_auth {}
}

variable "sql_mi_fqdn" {
  type        = string
  description = "The host name of the Managed Instance, e.g. sql-mi.0123456789ab.database.windows.net."
}

variable "sql_mi_principal_id" {
  type        = string
  description = "The principal ID of the instance's primary managed identity: the system-assigned identity, or the user-assigned identity set as primary on the instance."
}

variable "storage_account_id" {
  type = string
}

variable "storage_account_name" {
  type = string
}

variable "dba_email" {
  type = string
}

resource "azurerm_storage_container" "backups" {
  name                  = "sql-backups"
  storage_account_id    = var.storage_account_id
  container_access_type = "private"
}

# The instance's managed identity writes the backups. Scoped to the container.
resource "azurerm_role_assignment" "sql_mi_backups" {
  scope                = azurerm_storage_container.backups.id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = var.sql_mi_principal_id
}

# The credential name is the container URL, without a trailing slash. BACKUP TO URL finds it by that name.
resource "sqlserver_credential" "backups" {
  name     = "https://${var.storage_account_name}.blob.core.windows.net/${azurerm_storage_container.backups.name}"
  identity = "Managed Identity"

  depends_on = [azurerm_role_assignment.sql_mi_backups]
}

resource "sqlserver_agent_operator" "dba" {
  name          = "dba"
  email_address = var.dba_email # needs Database Mail; see docs/resources/agent_operator.md
}

resource "sqlserver_agent_job" "backup" {
  name                       = "backup_to_url"
  description                = "Copy-only backups of all user databases to ${sqlserver_credential.backups.name}"
  notify_level_email         = "on_failure"
  notify_email_operator_name = sqlserver_agent_operator.dba.name
}

# Managed Instance backup rules:
# - Only COPY_ONLY full backups are allowed: the instance owns the backup chain of its automatic backups.
# - Databases encrypted with service-managed TDE can't be backed up by users. They need customer-managed TDE (a Key
#   Vault key) or no TDE; otherwise they show up in the step's failure message.
# - One block blob holds about 195 GB, so the step stripes each backup over one file per 100 GB of data (up to 64),
#   with MAXTRANSFERSIZE = 4 MB and BLOCKSIZE = 64 KB.
# - Restore with RESTORE DATABASE [new_name] FROM URL = '<every stripe>' using the same credential. The new name
#   mustn't exist yet; Managed Instance doesn't support REPLACE or MOVE and places the files itself.
#
# The step runs as the SQL Server Agent service account (the job is owned by a sysadmin and has no
# database_user_name), which can back up every database.
resource "sqlserver_agent_job_step" "backup" {
  job_id                         = sqlserver_agent_job.backup.job_id
  name                           = "backup_user_databases"
  include_step_output_in_history = true
  log_to_table                   = true # the full output, in msdb.dbo.sysjobstepslogs; the history keeps ~4000 characters
  command                        = <<-SQL
    DECLARE @container nvarchar(400) = N'${sqlserver_credential.backups.name}';
    DECLARE @stamp nvarchar(20) = FORMAT(SYSUTCDATETIME(), 'yyyyMMdd_HHmmss');
    DECLARE @db sysname, @data_gb decimal(18, 2), @stripes int, @i int, @urls nvarchar(max), @sql nvarchar(max);
    DECLARE @failed nvarchar(max) = N'';

    DECLARE dbs CURSOR LOCAL FAST_FORWARD FOR
      SELECT d.name, SUM(CAST(f.size AS bigint)) * 8 / 1024.0 / 1024.0
      FROM sys.databases AS d
      JOIN sys.master_files AS f ON f.database_id = d.database_id AND f.type = 0 -- data files
      WHERE d.database_id > 4 AND d.state_desc = N'ONLINE'
      GROUP BY d.name;

    OPEN dbs;
    FETCH NEXT FROM dbs INTO @db, @data_gb;
    WHILE @@FETCH_STATUS = 0
    BEGIN
      -- One stripe per 100 GB of data, at least 1 and at most 64.
      SET @stripes = CEILING(@data_gb / 100);
      SET @stripes = CASE WHEN @stripes < 1 THEN 1 WHEN @stripes > 64 THEN 64 ELSE @stripes END;

      SET @urls = N'';
      SET @i = 1;
      WHILE @i <= @stripes
      BEGIN
        SET @urls += CASE WHEN @i > 1 THEN N', ' ELSE N'' END + N'URL = N'''
          + REPLACE(@container + N'/' + @db + N'/' + @db + N'_' + @stamp + N'_'
                    + CAST(@i AS nvarchar(3)) + N'of' + CAST(@stripes AS nvarchar(3)) + N'.bak', N'''', N'''''')
          + N'''';
        SET @i += 1;
      END;

      SET @sql = N'BACKUP DATABASE ' + QUOTENAME(@db) + N' TO ' + @urls
        + N' WITH COPY_ONLY, COMPRESSION, CHECKSUM, MAXTRANSFERSIZE = 4194304, BLOCKSIZE = 65536';
      -- No TRY/CATCH: it would only keep SQL Server's last, generic error ("BACKUP DATABASE is terminating
      -- abnormally"). Uncaught, the real reason a backup failed goes to the step output, and the loop carries on
      -- with the next database.
      EXEC (@sql);
      IF @@ERROR <> 0
        SET @failed += @db + NCHAR(10);
      ELSE
        PRINT N'Backed up ' + @db + N' in ' + CAST(@stripes AS nvarchar(3)) + N' file(s).';

      FETCH NEXT FROM dbs INTO @db, @data_gb;
    END;
    CLOSE dbs;
    DEALLOCATE dbs;

    IF @failed <> N''
    BEGIN
      DECLARE @message nvarchar(2048) = LEFT(N'Backups failed for (see the errors above):' + NCHAR(10) + @failed, 2048);
      THROW 50000, @message, 1;
    END;
  SQL
}

resource "sqlserver_agent_schedule" "nightly" {
  name              = "Nightly 02:00 UTC"
  frequency_type    = "daily"
  active_start_time = "02:00:00" # Managed Instance runs in UTC unless created with another time zone
}

resource "sqlserver_agent_job_schedule" "backup_nightly" {
  job_id      = sqlserver_agent_job.backup.job_id
  schedule_id = sqlserver_agent_schedule.nightly.schedule_id
}
