# Resource: sqlserver_classifier_function

This resource allows you to manage SQL Server Resource Governor classifier functions.

## Example Usage

```hcl
resource "sqlserver_classifier_function" "example" {
  name       = "my_classifier"
  definition = "RETURN 'default';"
}
```

## Argument Reference

- `name` (Required) - The name of the classifier function.
- `definition` (Required) - The T-SQL function body (excluding CREATE FUNCTION ... and END).

## Import

This resource can be imported using:

```
terraform import sqlserver_classifier_function.example my_classifier
```

## Documentation

For more information, see the [Resource Governor documentation](https://learn.microsoft.com/en-us/sql/relational-databases/resource-governor/resource-governor).
