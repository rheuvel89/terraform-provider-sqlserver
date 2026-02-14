# sqlserver_user

The `sqlserver_user` resource creates and manages a user on a SQL Server database.

## Example Usage

```hcl
resource "sqlserver_user" "example" {
  username   = "testuser"
  login_name = sqlserver_login.example.login_name
}
```

## Argument Reference

* `database` - (Optional) The name of the database in which to create the user. Defaults to `master`.
* `username` - (Required) The name of the user.
* `object_id` - (Optional) The object ID for the user.
* `login_name` - (Optional) The login name to associate with the user. Cannot be set with `password`.
* `password` - (Optional, Sensitive) The password for the user. Cannot be set with `login_name`.
* `sid` - (Computed) The security identifier (SID) for the user.
* `authentication_type` - (Computed) The authentication type for the user.
* `principal_id` - (Computed) The principal ID for the user.
* `roles` - (Optional) A set of database roles to assign to the user.

## Attribute Reference

* `principal_id` - The principal id of this database user.
* `sid` - The security identifier (SID) of this database user in String format.
* `authentication_type` - One of `DATABASE`, `INSTANCE`, or `EXTERNAL`.
\