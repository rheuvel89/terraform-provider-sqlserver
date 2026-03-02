# sqlserver_resource_pool

The `sqlserver_resource_pool` resource creates and manages a resource pool in SQL Server Resource Governor.

Resource pools represent physical resources of the server (CPU, memory, I/O). You can allocate or limit the resources available to workload groups within a pool.

## Example Usage

```hcl
resource "sqlserver_resource_pool" "reporting" {
  name               = "ReportingPool"
  min_cpu_percent    = 10
  max_cpu_percent    = 50
  min_memory_percent = 10
  max_memory_percent = 50
  cap_cpu_percent    = 50
}

resource "sqlserver_resource_pool" "oltp" {
  name               = "OLTPPool"
  min_cpu_percent    = 50
  max_cpu_percent    = 100
  min_memory_percent = 50
  max_memory_percent = 100
}
```

## Argument Reference

* `name` - (Required) The name of the resource pool. Must be unique on the server.
* `min_cpu_percent` - (Optional) Specifies the guaranteed average CPU bandwidth for all requests in the resource pool when there is CPU contention. Range is 0 to 100. Default is 0.
* `max_cpu_percent` - (Optional) Specifies the maximum average CPU bandwidth that all requests in resource pool will receive when there is CPU contention. Range is 1 to 100. Default is 100.
* `min_memory_percent` - (Optional) Specifies the minimum amount of memory reserved for this resource pool that cannot be shared with other resource pools. Range is 0 to 100. Default is 0.
* `max_memory_percent` - (Optional) Specifies the total server memory that can be used by requests in this resource pool. Range is 1 to 100. Default is 100.
* `cap_cpu_percent` - (Optional) Specifies a hard cap on the CPU bandwidth that all requests in the resource pool will receive. Range is 1 to 100. Default is 100.
* `min_iops_per_volume` - (Optional) Specifies the minimum I/O operations per second (IOPS) per disk volume to reserve for the resource pool. Default is 0.
* `max_iops_per_volume` - (Optional) Specifies the maximum I/O operations per second (IOPS) per disk volume to allow for the resource pool. 0 means unlimited. Default is 0.

## Attribute Reference

* `pool_id` - The ID of the resource pool.

## Import

Resource pools can be imported using the pool name:

```shell
terraform import sqlserver_resource_pool.example PoolName
```
