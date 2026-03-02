# sqlserver_resource_governor

The `sqlserver_resource_governor` resource manages the SQL Server Resource Governor configuration.

Resource Governor enables you to manage SQL Server workload and system resource consumption. It allows you to specify limits on the amount of CPU, physical I/O, and memory that incoming application requests can use.

**Note:** There is only one Resource Governor configuration per SQL Server instance. Creating multiple `sqlserver_resource_governor` resources will result in conflicts.

## Example Usage

### Basic Configuration

```hcl
resource "sqlserver_resource_governor" "config" {
  enabled = true
}
```

### With Classifier Function

First, create a classifier function in SQL Server:

```sql
CREATE FUNCTION dbo.ClassifierFunction()
RETURNS SYSNAME WITH SCHEMABINDING
AS
BEGIN
    DECLARE @WorkloadGroup SYSNAME
    
    IF APP_NAME() LIKE 'Report%'
        SET @WorkloadGroup = 'ReportingGroup'
    ELSE IF APP_NAME() LIKE 'OLTP%'
        SET @WorkloadGroup = 'OLTPGroup'
    ELSE
        SET @WorkloadGroup = 'default'
    
    RETURN @WorkloadGroup
END
GO
```

Then configure Resource Governor with the classifier:

```hcl
resource "sqlserver_resource_pool" "reporting" {
  name               = "ReportingPool"
  max_cpu_percent    = 30
  max_memory_percent = 30
}

resource "sqlserver_workload_group" "reporting" {
  name               = "ReportingGroup"
  resource_pool_name = sqlserver_resource_pool.reporting.name
}

resource "sqlserver_resource_governor" "config" {
  enabled             = true
  classifier_function = "dbo.ClassifierFunction"
  
  depends_on = [
    sqlserver_workload_group.reporting
  ]
}
```

## Argument Reference

* `enabled` - (Optional) Specifies whether the resource governor is enabled. Default is `true`.
* `classifier_function` - (Optional) The fully qualified name of the classifier function (schema.function_name). This function classifies incoming sessions into workload groups. Leave empty or omit to use no classifier function (all sessions go to default workload group).

## Attribute Reference

No additional attributes are exported.

## Import

Resource Governor configuration can be imported:

```shell
terraform import sqlserver_resource_governor.config resource_governor
```

## Notes

1. The classifier function must exist before it can be assigned to Resource Governor.
2. When Resource Governor is disabled, all sessions run in the default workload group.
3. Destroying this resource will disable Resource Governor and clear the classifier function.
4. Changes to resource pools and workload groups are automatically applied by reconfiguring Resource Governor.
