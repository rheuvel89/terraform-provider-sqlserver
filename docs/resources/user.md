# sqlserver_user

The `sqlserver_user` resource creates and manages a user on a SQL Server database.

## Example Usage

### User from SQL Login (Instance User)

```hcl
resource "sqlserver_login" "example" {
  sql_login {
    login_name = "testlogin"
    password   = "NotSoS3cret?"
  }
}

resource "sqlserver_user" "example" {
  database = "mydb"
  instance_user {
    username   = "testuser"
    login_name = sqlserver_login.example.sql_login[0].login_name
  }
}
```

### Contained Database User

```hcl
resource "sqlserver_user" "contained" {
  database = "mydb"
  database_user {
    username = "containeduser"
    password = "SecureP@ssw0rd!"
  }
}
```

### External User (Azure AD)

```hcl
resource "sqlserver_user" "external" {
  database = "mydb"
  external_user {
    username = "user@domain.com"
  }
}
```

## Argument Reference

* `database` - (Optional) The name of the database in which to create the user. Defaults to `master`.
* `instance_user` - (Optional) Block for creating a user from a SQL Server login. Only one of `instance_user`, `database_user`, or `external_user` can be specified.
  * `username` - (Required) The name of the user.
  * `login_name` - (Required) The login name to associate with the user.
* `database_user` - (Optional) Block for creating a contained database user. Only one of `instance_user`, `database_user`, or `external_user` can be specified.
  * `username` - (Required) The name of the user.
  * `password` - (Required, Sensitive) The password for the contained database user.
* `external_user` - (Optional) Block for creating an external (Azure AD) user. Only one of `instance_user`, `database_user`, or `external_user` can be specified.
  * `username` - (Required) The name of the user (typically the Azure AD user's email or display name).
  * `object_id` - (Optional) The Azure AD object ID for the user.
* `roles` - (Optional) A set of database roles to assign to the user.

## Attribute Reference

* `principal_id` - The principal ID of this database user.
* `sid` - The security identifier (SID) of this database user in string format.
* `authentication_type` - One of `DATABASE`, `INSTANCE`, or `EXTERNAL`.