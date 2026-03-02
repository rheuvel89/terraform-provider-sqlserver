# sqlserver_user

The `sqlserver_user` resource creates and manages a user on a SQL Server database.

## Example Usage

### User from SQL Login

```hcl
resource "sqlserver_login" "example" {
  sql_login {
    login_name = "testlogin"
    password   = "NotSoS3cret?"
  }
}

resource "sqlserver_user" "example" {
  database   = "mydb"
  username   = "testuser"
  login_name = sqlserver_login.example.sql_login[0].login_name
}
```

### Contained Database User

```hcl
resource "sqlserver_user" "contained" {
  database = "mydb"
  username = "containeduser"
  password = "SecureP@ssw0rd!"
}
```

### External User (Azure AD)

```hcl
resource "sqlserver_user" "external" {
  database = "mydb"
  username = "user@domain.com"
}
```

## Argument Reference

* `database` - (Optional) The name of the database in which to create the user. Defaults to `master`.
* `username` - (Required) The name of the user.
* `object_id` - (Optional) The Azure AD object ID for the user.
* `login_name` - (Optional) The login name to associate with the user. Cannot be set with `password`.
* `password` - (Optional, Sensitive) The password for the user (for contained database users). Cannot be set with `login_name`.
* `roles` - (Optional) A set of database roles to assign to the user.

## Attribute Reference

* `principal_id` - The principal ID of this database user.
* `sid` - The security identifier (SID) of this database user in string format.
* `authentication_type` - One of `DATABASE`, `INSTANCE`, or `EXTERNAL`.