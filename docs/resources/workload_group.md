# sqlserver_workload_group

The `sqlserver_workload_group` resource creates and manages a workload group in SQL Server Resource Governor.

Workload groups serve as containers for session requests that are similar according to the classification criteria. A workload group allows for aggregate monitoring of resource consumption and the application of policies to all the requests in the group.

## Example Usage

```hcl
resource "sqlserver_resource_pool" "reporting" {
  name               = "ReportingPool"
  min_cpu_percent    = 10
  max_cpu_percent    = 50
}

resource "sqlserver_workload_group" "adhoc_reports" {
  name                             = "AdhocReports"
  resource_pool_name               = sqlserver_resource_pool.reporting.name
  importance                       = "LOW"
  request_max_memory_grant_percent = 25
  request_max_cpu_time_sec         = 60
  max_dop                          = 4
  group_max_requests               = 10
}

resource "sqlserver_workload_group" "scheduled_reports" {
  name                             = "ScheduledReports"
  resource_pool_name               = sqlserver_resource_pool.reporting.name
  importance                       = "MEDIUM"
  request_max_memory_grant_percent = 50
  max_dop                          = 8
}
```

## Argument Reference

* `name` - (Required) The name of the workload group. Must be unique on the server.
* `resource_pool_name` - (Required) The name of the resource pool to associate with the workload group. Can be changed to move the workload group to a different pool.
* `importance` - (Optional) Specifies the relative importance of a request in the workload group. Valid values are `LOW`, `MEDIUM`, and `HIGH`. Default is `MEDIUM`.
* `request_max_memory_grant_percent` - (Optional) Specifies the maximum amount of memory that a single request can take from the pool. Range is 1 to 100. Default is 25.
* `request_max_cpu_time_sec` - (Optional) Specifies the maximum amount of CPU time, in seconds, that a request can use. 0 = unlimited. Default is 0.
* `request_memory_grant_timeout_sec` - (Optional) Specifies the maximum time, in seconds, that a query can wait for a memory grant to become available. 0 = use internal calculation based on query cost. Default is 0.
* `max_dop` - (Optional) Specifies the maximum degree of parallelism (MAXDOP) for parallel query execution. 0 = use global setting. Default is 0.
* `group_max_requests` - (Optional) Specifies the maximum number of simultaneous requests that are allowed to execute in the workload group. 0 = unlimited. Default is 0.

## Attribute Reference

* `group_id` - The ID of the workload group.

## Import

Workload groups can be imported using the group name:

```shell
terraform import sqlserver_workload_group.example GroupName
```
