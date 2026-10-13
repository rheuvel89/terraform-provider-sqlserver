package sql

import (
	"context"
	"database/sql"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/pkg/errors"
)

const (
	errDeadlockVictim     = 1205
	agentDeadlockAttempts = 5
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

// agentExec runs a write against msdb, retrying when SQL Server picks it as a deadlock victim. msdb's Agent
// procedures deadlock when Terraform creates several jobs or steps in parallel. The victim's transaction is
// rolled back, so a command is only safe to retry when all of its changes are in one transaction.
func (c *Connector) agentExec(ctx context.Context, command string, args ...interface{}) error {
	return retryOnDeadlock(ctx, func() error {
		return c.ExecContext(ctx, command, args...)
	})
}

func retryOnDeadlock(ctx context.Context, f func() error) error {
	return withRetry(ctx, agentDeadlockAttempts,
		func(attempt int) time.Duration { return time.Duration(attempt*100) * time.Millisecond },
		isDeadlockVictim,
		func(context.Context) error { return f() },
	)
}

func isDeadlockVictim(err error) bool {
	var sqlErr mssql.Error
	return errors.As(err, &sqlErr) && sqlErr.Number == errDeadlockVictim
}
