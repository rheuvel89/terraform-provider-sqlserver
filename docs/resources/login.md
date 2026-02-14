# sqlserver_login

The `sqlserver_login` resource creates and manages a login on a SQL Server.

## Example Usage

```hcl
resource "sqlserver_login" "example" {
  login_name = "testlogin"
  password   = "NotSoS3cret?"
}
```

## Argument Reference

* `sql_login` - (Optional) Block for SQL login. Only one of `sql_login` or `external_login` can be specified.
  * `login_name` - (Required) The name of the SQL login.
  * `password` - (Required, Sensitive) The password for the SQL login.
* `external_login` - (Optional) Block for external login. Only one of `sql_login` or `external_login` can be specified.
  * `login_name` - (Required) The name of the external login.
  * `external_login_type` - (Optional) The type of external login. Valid values are `user` or `group`. Defaults to `user`.
* `sid` - (Computed) The security identifier (SID) for the login.
* `principal_id` - (Computed) The principal ID for the login.
* `user` - (Optional) Whether the external login is a user. Defaults to `false`.
* `group` - (Optional) Whether the external login is a group. Defaults to `false`.

## Attribute Reference

* `principal_id` - The principal id of this server login.
* `sid` - The security identifier (SID) of this login in String format.
