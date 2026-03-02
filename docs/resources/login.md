# sqlserver_login

The `sqlserver_login` resource creates and manages a login on a SQL Server.

## Example Usage

### SQL Login

```hcl
resource "sqlserver_login" "example" {
  sql_login {
    login_name = "testlogin"
    password   = "NotSoS3cret?"
  }
}
```

### External Login (Azure AD)

```hcl
resource "sqlserver_login" "external" {
  external_login {
    login_name          = "user@domain.com"
    external_login_type = "user"
  }
}
```

## Argument Reference

* `sql_login` - (Optional) Block for SQL login. Only one of `sql_login` or `external_login` can be specified.
  * `login_name` - (Required) The name of the SQL login.
  * `password` - (Required, Sensitive) The password for the SQL login.
* `external_login` - (Optional) Block for external login. Only one of `sql_login` or `external_login` can be specified.
  * `login_name` - (Required) The name of the external login.
  * `external_login_type` - (Optional) The type of external login. Valid values are `user` or `group`. Defaults to `user`.
* `sid` - (Optional) The security identifier (SID) for the login. If not specified, SQL Server will generate one.

## Attribute Reference

* `principal_id` - The principal ID of this server login.
* `sid` - The security identifier (SID) of this login in string format.
