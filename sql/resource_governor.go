package sql

import (
	"context"
	"database/sql"
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"
)

const nonCurrentUserSessionsWhereClause = `s.is_user_process = 1 AND s.session_id <> @@SPID`

func (c *Connector) GetResourceGovernor(ctx context.Context) (*model.ResourceGovernor, error) {
	var rg model.ResourceGovernor
	var classifierFunctionID sql.NullInt64
	err := c.QueryRowContext(ctx,
		`SELECT 
			is_enabled,
			classifier_function_id
		FROM sys.resource_governor_configuration`,
		func(r *sql.Row) error {
			return r.Scan(
				&rg.IsEnabled,
				&classifierFunctionID,
			)
		},
	)
	if err != nil {
		return nil, err
	}

	// Get the classifier function name if one is set
	if classifierFunctionID.Valid && classifierFunctionID.Int64 > 0 {
		err = c.QueryRowContext(ctx,
			`SELECT SCHEMA_NAME(schema_id) + '.' + name 
			FROM sys.objects 
			WHERE object_id = @objectId`,
			func(r *sql.Row) error {
				return r.Scan(&rg.ClassifierFunction)
			},
			sql.Named("objectId", classifierFunctionID.Int64),
		)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	return &rg, nil
}

func (c *Connector) EnableResourceGovernor(ctx context.Context, classifierFunction string) error {
	if err := c.killSessionsByWhereClause(ctx, nonCurrentUserSessionsWhereClause); err != nil {
		return err
	}

	// Set classifier function if provided
	if classifierFunction != "" {
		cmd := fmt.Sprintf("ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = %s)", classifierFunction)
		if err := c.ExecContext(ctx, cmd); err != nil {
			return err
		}
	}

	// Enable resource governor
	if err := c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE"); err != nil {
		return err
	}

	return nil
}

func (c *Connector) DisableResourceGovernor(ctx context.Context) error {
	if err := c.killSessionsByWhereClause(ctx, nonCurrentUserSessionsWhereClause); err != nil {
		return err
	}

	// Clear classifier function
	if err := c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = NULL)"); err != nil {
		return err
	}

	// Disable resource governor
	if err := c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR DISABLE"); err != nil {
		return err
	}

	return nil
}

func (c *Connector) UpdateResourceGovernor(ctx context.Context, rg *model.ResourceGovernor) error {
	if err := c.killSessionsByWhereClause(ctx, nonCurrentUserSessionsWhereClause); err != nil {
		return err
	}

	// Set classifier function
	var cmd string
	if rg.ClassifierFunction != "" {
		cmd = fmt.Sprintf("ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = %s)", rg.ClassifierFunction)
	} else {
		cmd = "ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = NULL)"
	}
	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Enable or disable based on the flag
	if rg.IsEnabled {
		return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
	}
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR DISABLE")
}

func (c *Connector) killSessionsByWhereClause(ctx context.Context, whereClause string, args ...interface{}) error {
	query := fmt.Sprintf(`
		SELECT s.session_id
		FROM sys.dm_exec_sessions s
		WHERE %s`, whereClause)

	sessionIDs := make([]int, 0)
	err := c.QueryContext(ctx, query, func(rows *sql.Rows) error {
		for rows.Next() {
			var sessionID int
			if err := rows.Scan(&sessionID); err != nil {
				return err
			}
			sessionIDs = append(sessionIDs, sessionID)
		}
		return rows.Err()
	}, args...)
	if err != nil {
		return err
	}

	for _, sessionID := range sessionIDs {
		if err := c.ExecContext(ctx, fmt.Sprintf("KILL %d", sessionID)); err != nil {
			return err
		}
	}

	return nil
}

func (c *Connector) killWorkloadGroupSessions(ctx context.Context, workloadGroupName string) error {
	return c.killSessionsByWhereClause(ctx, nonCurrentUserSessionsWhereClause+` AND s.group_id IN (
		SELECT g.group_id
		FROM sys.resource_governor_workload_groups g
		WHERE g.name = @workloadGroupName
	)`, sql.Named("workloadGroupName", workloadGroupName))
}

func (c *Connector) killResourcePoolSessions(ctx context.Context, resourcePoolName string) error {
	return c.killSessionsByWhereClause(ctx, nonCurrentUserSessionsWhereClause+` AND s.group_id IN (
		SELECT g.group_id
		FROM sys.resource_governor_workload_groups g
		INNER JOIN sys.resource_governor_resource_pools p ON p.pool_id = g.pool_id
		WHERE p.name = @resourcePoolName
	)`, sql.Named("resourcePoolName", resourcePoolName))
}
