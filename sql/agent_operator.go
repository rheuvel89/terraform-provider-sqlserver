package sql

import (
	"context"
	"database/sql"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetAgentOperator(ctx context.Context, operatorID int) (*model.AgentOperator, error) {
	var op model.AgentOperator
	err := c.QueryRowContext(ctx,
		`SELECT o.[id], o.[name], CAST(o.[enabled] AS bit), COALESCE(o.[email_address], ''), COALESCE(c.[name], '')
         FROM [msdb].[dbo].[sysoperators] o
         LEFT JOIN [msdb].[dbo].[syscategories] c ON c.[category_id] = o.[category_id]
         WHERE o.[id] = @id`,
		func(r *sql.Row) error {
			return r.Scan(&op.OperatorID, &op.Name, &op.Enabled, &op.EmailAddress, &op.CategoryName)
		},
		sql.Named("id", operatorID),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &op, nil
}

func (c *Connector) CreateAgentOperator(ctx context.Context, op *model.AgentOperator) (int, error) {
	var id int
	err := c.QueryRowContext(ctx,
		`SET NOCOUNT ON
         EXEC [msdb].[dbo].[sp_add_operator] @name = @name, @enabled = @enabled, @email_address = @email, @category_name = @category
         SELECT [id] FROM [msdb].[dbo].[sysoperators] WHERE [name] = @name`,
		func(r *sql.Row) error {
			return r.Scan(&id)
		},
		sql.Named("name", op.Name),
		sql.Named("enabled", op.Enabled),
		sql.Named("email", nullIfEmpty(op.EmailAddress)),
		sql.Named("category", nullIfEmpty(op.CategoryName)),
	)
	return id, err
}

// UpdateAgentOperator updates the operator currently called name to the values in op, renaming it when op.Name differs.
func (c *Connector) UpdateAgentOperator(ctx context.Context, name string, op *model.AgentOperator) error {
	return c.ExecContext(ctx,
		`EXEC [msdb].[dbo].[sp_update_operator] @name = @name, @new_name = @newName, @enabled = @enabled, @email_address = @email, @category_name = @category`,
		sql.Named("name", name),
		sql.Named("newName", nullIfEmpty(op.Name)),
		sql.Named("enabled", op.Enabled),
		sql.Named("email", op.EmailAddress),
		sql.Named("category", nullIfEmpty(op.CategoryName)),
	)
}

func (c *Connector) DeleteAgentOperator(ctx context.Context, name string) error {
	return c.ExecContext(ctx,
		`IF EXISTS (SELECT 1 FROM [msdb].[dbo].[sysoperators] WHERE [name] = @name)
           EXEC [msdb].[dbo].[sp_delete_operator] @name = @name`,
		sql.Named("name", name),
	)
}
