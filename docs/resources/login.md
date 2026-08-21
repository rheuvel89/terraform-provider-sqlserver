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
  roles = ["dbcreator", "diskadmin"]
}
```

### Disabled SQL Login

```hcl
resource "sqlserver_login" "disabled_example" {
  sql_login {
    login_name = "disabledlogin"
    password   = "NotSoS3cret?"
  }
  is_disabled = true
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
* `roles` - (Optional) A set of fixed server roles (e.g. `sysadmin`, `dbcreator`, `securityadmin`) to assign to the login. The built-in `public` role is always assigned and cannot be managed through this attribute.
* `is_disabled` - (Optional) When set to `true` the login is disabled (cannot connect). Defaults to `false` (login is enabled).

## Attribute Reference

* `principal_id` - The principal ID of this server login.
* `sid` - The security identifier (SID) of this login in string format.
* `is_disabled` - Whether the login is disabled.

## Import

Logins can be imported using a resource ID with the following format:

```shell
terraform import sqlserver_login.example sqlserver://host:port/login/LoginName
```

or as a terraform import block:
```tf
import {
  to = sqlserver_login.example
  id = "sqlserver://host:port/login/LoginName"
}
```

Include credentials when provider credentials are not sufficient like:

```
terraform import sqlserver_login.example sqlserver://host:port/login/LoginName?username=username&password=p@55word
```