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

### SQL Login with an Ephemeral Password

Requires Terraform CLI 1.11 or later, since `password_wo` is a write-only argument.

Use a write-only argument together with an ephemeral value (e.g. [`ephemeral.random_password`](https://registry.terraform.io/providers/hashicorp/random/latest/docs/ephemeral-resources/password)) so the password is never written to the plan or state file. `password_wo_version` must be bumped to rotate the password.

```hcl
ephemeral "random_password" "example" {
  length = 64
}

resource "sqlserver_login" "example" {
  sql_login {
    login_name          = "testlogin"
    password_wo         = ephemeral.random_password.example.result
    password_wo_version = 1
  }
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
  * `password` - (Optional, Sensitive) The password for the SQL login. Exactly one of `password` or `password_wo` must be set.
  * `password_wo` - (Optional, Sensitive, Write-Only) The password for the SQL login. Accepts ephemeral values and is never persisted to plan or state. Exactly one of `password` or `password_wo` must be set.
  * `password_wo_version` - (Optional) An integer used to trigger rotation of `password_wo`. Increment this value whenever `password_wo` changes.
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