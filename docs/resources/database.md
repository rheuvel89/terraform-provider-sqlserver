# sqlserver_database

The `sqlserver_database` resource creates and manages a database on a SQL Server instance.

## Example Usage

### Basic Database

```hcl
resource "sqlserver_database" "example" {
  name = "mydatabase"
}
```

### Database with Options

```hcl
resource "sqlserver_database" "example" {
  name                = "mydatabase"
  recovery_model      = "SIMPLE"
  compatibility_level = 150
}
```

## Argument Reference

* `name` - (Required, ForceNew) The name of the database.
* `collation` - (Optional, Computed) The collation of the database. If not specified, the server default collation is used. Changing this value requires the database to be in single-user mode.
* `recovery_model` - (Optional, Computed) The recovery model of the database. Valid values are `FULL`, `SIMPLE`, `BULK_LOGGED`.
* `compatibility_level` - (Optional, Computed) The SQL Server compatibility level (e.g. `100`, `110`, `130`, `150`, `160`).

## Attribute Reference

* `owner` - The owner (server login) of the database.
* `collation` - The database collation.
* `recovery_model` - The database recovery model.
* `compatibility_level` - The database compatibility level.

## Import

Databases can be imported using a resource ID with the following format:

```shell
terraform import sqlserver_database.example sqlserver://host:port/database/DatabaseName
```

or as a terraform import block:

```tf
import {
  to = sqlserver_database.example
  id = "sqlserver://host:port/database/DatabaseName"
}
```
