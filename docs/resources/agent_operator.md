# sqlserver_agent_operator

The `sqlserver_agent_operator` resource creates and manages a SQL Server Agent operator: a name and an email address
that jobs can notify. See `notify_level_email` on [`sqlserver_agent_job`](agent_job.md).

## Example Usage

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

## Argument Reference

* `name` - (Required) The name of the operator. Changing it renames the operator in place.
* `enabled` - (Optional) Whether the operator receives notifications. Default is `true`.
* `email_address` - (Optional) The email address notifications are sent to.
* `category_name` - (Optional) The name of an existing operator category. Default is `[Uncategorized]`; removing the
  argument moves the operator back to that category.

Pager and net send notifications aren't supported, as SQL Server has deprecated them.

## Attribute Reference

* `operator_id` - The ID of the operator in msdb.

## Email Delivery

An operator only stores the address. SQL Server Agent sends the mail through **Database Mail**, which this provider
doesn't manage and doesn't check. Without it, jobs and operators are created without errors but no mail arrives.
Database Mail needs:

* `sp_configure 'Database Mail XPs', 1`;
* an SMTP account (`msdb.dbo.sysmail_add_account_sp`) and a profile that contains it
  (`sysmail_add_profile_sp` and `sysmail_add_profileaccount_sp`);
* on SQL Server (on-premises or in a VM), SQL Server Agent set to use that profile: in SSMS, SQL Server Agent >
  Properties > Alert System > Enable mail profile, followed by a restart of SQL Server Agent.

On **Azure SQL Managed Instance**:

* SQL Server Agent always uses the profile named exactly `AzureManagedInstance_dbmail_profile`; there's no setting to
  choose another one.
* There's no local SMTP relay, so use an external SMTP service, such as Exchange Online, Azure Communication Services
  Email or SendGrid. Use port 587 with TLS; Azure generally blocks outbound port 25.
* The network security group and route table of the instance's subnet must allow outbound traffic to the SMTP host and
  port.

A minimal setup on Managed Instance:

```sql
EXEC sp_configure 'Database Mail XPs', 1; RECONFIGURE;

EXEC msdb.dbo.sysmail_add_account_sp
  @account_name = N'smtp', @email_address = N'sql-alerts@example.com',
  @mailserver_name = N'smtp.office365.com', @port = 587, @enable_ssl = 1,
  @username = N'sql-alerts@example.com', @password = N'...';

EXEC msdb.dbo.sysmail_add_profile_sp
  @profile_name = N'AzureManagedInstance_dbmail_profile';

EXEC msdb.dbo.sysmail_add_profileaccount_sp
  @profile_name = N'AzureManagedInstance_dbmail_profile',
  @account_name = N'smtp', @sequence_number = 1;
```

When mail doesn't arrive, check `msdb.dbo.sysmail_allitems` (sent, failed or unsent), `msdb.dbo.sysmail_event_log`
and the SQL Server Agent error log.

## Permissions

Only a sysadmin can create, change or delete operators.

## Import

Import isn't supported.
