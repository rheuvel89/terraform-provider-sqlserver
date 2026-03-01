# Resource: sqlserver_resource_governor

This resource allows you to manage the SQL Server Resource Governor.

## Example Usage

```hcl
resource "sqlserver_resource_governor" "example" {
  enabled = true
  classifier_function = "my_classifier_function"
}
```

## Argument Reference

- `enabled` (Required) - Whether the Resource Governor is enabled.
- `classifier_function` (Optional) - The name of the classifier function to use.

## Import

This resource can be imported using:

```
terraform import sqlserver_resource_governor.example resource_governor
```

## Documentation

For more information, see the [Resource Governor documentation](https://learn.microsoft.com/en-us/sql/relational-databases/resource-governor/resource-governor).
