# Microsoft SQL Server Provider

The SQL Server provider exposes resources used to manage the configuration of resources in a Microsoft SQL Server and an Azure SQL Database. It might also work for other Microsoft SQL Server products like Azure Managed SQL Server, but it has not been tested against these resources.

## Example Usage

```hcl
terraform {
  required_providers {
    sqlserver = {
      source = "rheuvel89/sqlserver"
      version = "0.1.0"
    }
  }
}

provider "sqlserver" {
  debug = false
  host = "localhost"
  login {
    username = "sa"
    password = "MySuperSecr3t!"
  }
}

resource "sqlserver_login" "example" {
  sql_login {
    login_name = "testlogin"
    password   = "NotSoS3cret?"
  }
}

resource "sqlserver_user" "example" {
  database   = "master"
  username   = "testuser"
  login_name = sqlserver_login.example.sql_login[0].login_name
}
```

## Argument Reference


The following arguments are supported:

### Provider Arguments

* `debug` - (Optional) Enable provider debug logging. Either `false` or `true`. Defaults to `false`. If `true`, the provider will write a debug log to `terraform-provider-sqlserver.log`.
* `host` - (Optional) The hostname or IP address of the SQL Server. Can be set via the `TF_SQLSERVER_HOST` environment variable.
* `port` - (Optional) The port number to connect to on the SQL Server. Defaults to `1433`. Can be set via the `TF_SQLSERVER_PORT` environment variable.
* `login` - (Optional) Block for SQL authentication. Conflicts with `azure_login`, `azuread_default_chain_auth`, and `azuread_managed_identity_auth`.
  * `username` - (Optional) The SQL Server username. Can be set via `TF_SQLSERVER_USERNAME`.
  * `password` - (Optional, Sensitive) The SQL Server password. Can be set via `TF_SQLSERVER_PASSWORD`.
* `azure_login` - (Optional) Block for Azure AD client credentials authentication. Conflicts with `login`, `azuread_default_chain_auth`, and `azuread_managed_identity_auth`.
  * `tenant_id` - (Optional) The Azure AD tenant ID. Can be set via `TF_SQLSERVER_TENANT_ID`.
  * `client_id` - (Optional) The Azure AD client ID. Can be set via `TF_SQLSERVER_CLIENT_ID`.
  * `client_secret` - (Optional, Sensitive) The Azure AD client secret. Can be set via `TF_SQLSERVER_CLIENT_SECRET`.
* `azuread_default_chain_auth` - (Optional) Use Azure AD Default Credential Chain. Conflicts with other authentication blocks.
* `azuread_managed_identity_auth` - (Optional) Use Azure AD Managed Identity authentication. Conflicts with other authentication blocks.
  * `user_id` - (Optional) The user-assigned managed identity client ID.
