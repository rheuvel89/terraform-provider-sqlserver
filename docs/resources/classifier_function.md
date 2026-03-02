# sqlserver_classifier_function

The `sqlserver_classifier_function` resource creates and manages a classifier function for SQL Server Resource Governor.

A classifier function is a scalar user-defined function that Resource Governor uses to classify incoming sessions into workload groups. The function is evaluated for each new session and returns the name of the workload group to which the session should be assigned.

## Example Usage

### Basic Classifier Function

```hcl
resource "sqlserver_classifier_function" "workload_classifier" {
  name        = "WorkloadClassifier"
  schema_name = "dbo"
  function_body = <<-EOF
    DECLARE @grp_name SYSNAME
    SET @grp_name = 'default'
    RETURN @grp_name
EOF
}
```

### Classifier Based on Application Name

```hcl
resource "sqlserver_resource_pool" "reporting" {
  name            = "ReportingPool"
  max_cpu_percent = 30
}

resource "sqlserver_workload_group" "reporting" {
  name               = "ReportingGroup"
  resource_pool_name = sqlserver_resource_pool.reporting.name
  importance         = "LOW"
}

resource "sqlserver_classifier_function" "app_classifier" {
  name        = "ApplicationClassifier"
  schema_name = "dbo"
  function_body = <<-EOF
    DECLARE @grp_name SYSNAME
    
    IF APP_NAME() LIKE 'Report%'
        SET @grp_name = 'ReportingGroup'
    ELSE IF APP_NAME() LIKE 'SSMS%'
        SET @grp_name = 'default'
    ELSE
        SET @grp_name = 'default'
    
    RETURN @grp_name
EOF

  depends_on = [sqlserver_workload_group.reporting]
}

resource "sqlserver_resource_governor" "config" {
  enabled             = true
  classifier_function = sqlserver_classifier_function.app_classifier.fully_qualified_name

  depends_on = [sqlserver_classifier_function.app_classifier]
}
```

### Classifier Based on Login Name

```hcl
resource "sqlserver_classifier_function" "login_classifier" {
  name        = "LoginClassifier"
  schema_name = "dbo"
  function_body = <<-EOF
    DECLARE @grp_name SYSNAME
    DECLARE @login_name SYSNAME = SUSER_SNAME()
    
    IF @login_name IN ('report_user', 'etl_user')
        SET @grp_name = 'BatchProcessingGroup'
    ELSE IF @login_name LIKE 'app_%'
        SET @grp_name = 'ApplicationGroup'
    ELSE
        SET @grp_name = 'default'
    
    RETURN @grp_name
EOF
}
```

### Classifier Based on Time of Day

```hcl
resource "sqlserver_classifier_function" "time_classifier" {
  name        = "TimeBasedClassifier"
  schema_name = "dbo"
  function_body = <<-EOF
    DECLARE @grp_name SYSNAME
    DECLARE @hour INT = DATEPART(HOUR, GETDATE())
    
    -- During business hours (8 AM - 6 PM), prioritize OLTP
    IF @hour >= 8 AND @hour < 18
    BEGIN
        IF APP_NAME() LIKE 'Report%'
            SET @grp_name = 'LowPriorityReports'
        ELSE
            SET @grp_name = 'OLTPGroup'
    END
    ELSE
    BEGIN
        -- Off-hours: allow reports full resources
        SET @grp_name = 'default'
    END
    
    RETURN @grp_name
EOF
}
```

## Argument Reference

* `name` - (Required) The name of the classifier function.
* `schema_name` - (Optional) The schema name for the classifier function. Defaults to `dbo`.
* `function_body` - (Required) The body of the classifier function. This should contain the T-SQL logic that determines which workload group a session belongs to. The function must:
  - Declare a variable of type `SYSNAME` to hold the workload group name
  - Return the workload group name
  - Not include `CREATE FUNCTION`, `BEGIN`, or `END` statements (these are added automatically)

## Attribute Reference

* `object_id` - The object ID of the classifier function in SQL Server.
* `fully_qualified_name` - The fully qualified name of the function (`schema_name.name`) for use with `sqlserver_resource_governor.classifier_function`.

## Import

Classifier functions can be imported using the schema and name:

```shell
terraform import sqlserver_classifier_function.example dbo.ClassifierFunction
```

## Notes

1. The function body should return a valid workload group name that exists in the system.
2. If the returned workload group doesn't exist, the session will be assigned to the `default` workload group.
3. The function is created with `SCHEMABINDING` for Resource Governor compatibility.
4. When updating a classifier function that is currently in use by Resource Governor, it will be temporarily removed, updated, and the users must re-assign it to Resource Governor.
5. Deleting a classifier function will automatically remove it from Resource Governor if it's configured as the classifier.

## Best Practices

1. Keep classifier functions simple and fast - they run for every new connection.
2. Always include a default case that returns `'default'`.
3. Test your classifier logic thoroughly before applying to production.
4. Use `depends_on` to ensure workload groups exist before the classifier references them.
5. Consider using `SUSER_SNAME()`, `APP_NAME()`, `HOST_NAME()`, or other session functions for classification.
