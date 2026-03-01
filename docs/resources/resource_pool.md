# Resource: sqlserver_resource_pool

This resource allows you to manage SQL Server Resource Governor resource pools.

## Example Usage

```hcl
resource "sqlserver_resource_pool" "example" {
  name                = "my_pool"
  min_memory_percent  = 0
  max_memory_percent  = 100
  min_cpu_percent     = 0
  max_cpu_percent     = 100
  cap_cpu_percent     = 100
}
```

## Argument Reference

- `name` (Required) - The name of the resource pool.
- `min_memory_percent` (Optional) - Minimum memory percent.
- `max_memory_percent` (Optional) - Maximum memory percent.
- `min_cpu_percent` (Optional) - Minimum CPU percent.
- `max_cpu_percent` (Optional) - Maximum CPU percent.
- `cap_cpu_percent` (Optional) - Cap CPU percent.

## Import

This resource can be imported using:

```
terraform import sqlserver_resource_pool.example my_pool
```

## Documentation

For more information, see the [Resource Governor documentation](https://learn.microsoft.com/en-us/sql/relational-databases/resource-governor/resource-governor).
