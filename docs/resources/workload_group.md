# Resource: sqlserver_workload_group

This resource allows you to manage SQL Server Resource Governor workload groups.

## Example Usage

```hcl
resource "sqlserver_resource_pool" "example" {
  name = "my_pool"
}

resource "sqlserver_workload_group" "example" {
  name      = "my_group"
  pool_name = sqlserver_resource_pool.example.name
  importance = "Medium"
  request_max_memory_grant_percent = 25
  request_max_cpu_time_sec = 0
  request_memory_grant_timeout_sec = 0
  max_dop = 0
}
```

## Argument Reference

- `name` (Required) - The name of the workload group.
- `pool_name` (Required) - The name of the resource pool to use.
- `importance` (Optional) - Importance (Low, Medium, High). Default: Medium.
- `request_max_memory_grant_percent` (Optional) - Max memory grant percent. Default: 25.
- `request_max_cpu_time_sec` (Optional) - Max CPU time in seconds. Default: 0.
- `request_memory_grant_timeout_sec` (Optional) - Memory grant timeout in seconds. Default: 0.
- `max_dop` (Optional) - Max degree of parallelism. Default: 0.

## Import

This resource can be imported using:

```
terraform import sqlserver_workload_group.example my_group
```

## Documentation

For more information, see the [Resource Governor documentation](https://learn.microsoft.com/en-us/sql/relational-databases/resource-governor/resource-governor).
