package sql

import (
	"context"
	"database/sql"
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetClassifierFunction(ctx context.Context, schemaName, name string) (*model.ClassifierFunction, error) {
	var fn model.ClassifierFunction
	err := c.QueryRowContext(ctx,
		`SELECT 
			o.object_id,
			SCHEMA_NAME(o.schema_id) AS schema_name,
			o.name,
			OBJECT_DEFINITION(o.object_id) AS definition
		FROM sys.objects o
		WHERE o.type = 'FN'
		AND SCHEMA_NAME(o.schema_id) = @schema_name
		AND o.name = @name`,
		func(r *sql.Row) error {
			return r.Scan(
				&fn.ObjectID,
				&fn.SchemaName,
				&fn.Name,
				&fn.Definition,
			)
		},
		sql.Named("schema_name", schemaName),
		sql.Named("name", name),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &fn, nil
}

func (c *Connector) CreateClassifierFunction(ctx context.Context, fn *model.ClassifierFunction) error {
	// The definition should be the complete CREATE FUNCTION statement body
	// We wrap it in a proper CREATE FUNCTION statement
	cmd := fmt.Sprintf(`CREATE FUNCTION %s.%s()
RETURNS SYSNAME
WITH SCHEMABINDING
AS
BEGIN
%s
END`,
		quoteName(fn.SchemaName),
		quoteName(fn.Name),
		fn.Definition,
	)

	return c.ExecContext(ctx, cmd)
}

func (c *Connector) UpdateClassifierFunction(ctx context.Context, fn *model.ClassifierFunction) error {
	// Drop and recreate since ALTER FUNCTION has limitations
	cmd := fmt.Sprintf(`
		-- First check if this function is the classifier and temporarily remove it
		DECLARE @isClassifier BIT = 0
		IF EXISTS (SELECT 1 FROM sys.resource_governor_configuration WHERE classifier_function_id = OBJECT_ID('%s.%s'))
		BEGIN
			SET @isClassifier = 1
			ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = NULL)
			ALTER RESOURCE GOVERNOR RECONFIGURE
		END

		-- Drop and recreate
		DROP FUNCTION %s.%s

		-- Recreate will be done in the next statement
	`,
		fn.SchemaName, fn.Name,
		quoteName(fn.SchemaName), quoteName(fn.Name),
	)

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Create the new function
	return c.CreateClassifierFunction(ctx, fn)
}

func (c *Connector) DeleteClassifierFunction(ctx context.Context, schemaName, name string) error {
	// First remove from resource governor if it's the classifier
	cmd := fmt.Sprintf(`
		IF EXISTS (SELECT 1 FROM sys.resource_governor_configuration WHERE classifier_function_id = OBJECT_ID('%s.%s'))
		BEGIN
			ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = NULL)
			ALTER RESOURCE GOVERNOR RECONFIGURE
		END

		IF OBJECT_ID('%s.%s', 'FN') IS NOT NULL
			DROP FUNCTION %s.%s
	`,
		schemaName, name,
		schemaName, name,
		quoteName(schemaName), quoteName(name),
	)

	return c.ExecContext(ctx, cmd)
}
