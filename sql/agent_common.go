package sql

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
)

// engineEditionAzureSQLDatabase is the SERVERPROPERTY('EngineEdition') value of Azure SQL Database,
// which has no SQL Server Agent.
const engineEditionAzureSQLDatabase = 5

// EnsureAgentAvailable returns a clear error when the server has no SQL Server Agent.
func (c *Connector) EnsureAgentAvailable(ctx context.Context) error {
	var edition int
	err := c.QueryRowContext(ctx, `SELECT CAST(SERVERPROPERTY('EngineEdition') AS int)`,
		func(r *sql.Row) error {
			return r.Scan(&edition)
		},
	)
	if err != nil {
		return err
	}
	if edition == engineEditionAzureSQLDatabase {
		return errors.New("SQL Server Agent is not available on Azure SQL Database; agent resources need SQL Server (on-premises or VM) or Azure SQL Managed Instance")
	}
	return nil
}

// nullIfEmpty passes an empty string to a stored procedure as NULL, which the msdb
// procedures treat as "use the default" (add) or "keep the current value" (update).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
