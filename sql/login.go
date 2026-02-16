package sql

import (
	"context"
	"database/sql"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetLogin(ctx context.Context, name string) (*model.Login, error) {
	var login model.Login
	err := c.QueryRowContext(ctx,
		"SELECT principal_id, name, CONVERT(VARCHAR(1000), [sid], 1) FROM [master].[sys].[sql_logins] WHERE [name] = @name",
		func(r *sql.Row) error {
			result := r.Scan(&login.PrincipalID, &login.LoginName, &login.SIDStr)
			return result
		},
		sql.Named("name", name),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &login, nil
}

func (c *Connector) CreateLogin(ctx context.Context, name, password, sourceType string) error {
	cmd := `DECLARE @sql nvarchar(max)
          IF @sourceType = 'external'
            BEGIN
              SET @sql = 'CREATE LOGIN ' + QuoteName(@name) + ' FROM EXTERNAL PROVIDER'
            END
          ELSE
            BEGIN
              SET @sql = 'CREATE LOGIN ' + QuoteName(@name) + ' ' + 'WITH PASSWORD = ' + QuoteName(@password, '''')
            END
          EXEC (@sql)`

	database := "master"
	return c.
		setDatabase(&database).
		ExecContext(ctx, cmd,
			sql.Named("name", name),
			sql.Named("password", password),
			sql.Named("sourceType", sourceType))
}

func (c *Connector) UpdateLogin(ctx context.Context, name string, password string) error {
	cmd := `DECLARE @sql nvarchar(max)
          SET @sql = 'ALTER LOGIN ' + QuoteName(@name) + ' ' +
                     'WITH PASSWORD = ' + QuoteName(@password, '''')
          EXEC (@sql)`
	return c.ExecContext(ctx, cmd,
		sql.Named("name", name),
		sql.Named("password", password))
}

func (c *Connector) DeleteLogin(ctx context.Context, name string) error {
	if err := c.killSessionsForLogin(ctx, name); err != nil {
		return err
	}
	cmd := `DECLARE @sql nvarchar(max)
          SET @sql = 'IF EXISTS (SELECT 1 FROM [master].[sys].[sql_logins] WHERE [name] = ' + QuoteName(@name, '''') + ') ' +
                     'DROP LOGIN ' + QuoteName(@name)
          EXEC (@sql)`
	return c.ExecContext(ctx, cmd, sql.Named("name", name))
}

func (c *Connector) killSessionsForLogin(ctx context.Context, name string) error {
	cmd := `-- adapted from https://stackoverflow.com/a/5178097/38055
          DECLARE sessionsToKill CURSOR FAST_FORWARD FOR
            SELECT session_id
            FROM sys.dm_exec_sessions
            WHERE login_name = @name
          OPEN sessionsToKill
          DECLARE @sessionId INT
          DECLARE @statement NVARCHAR(200)
          FETCH NEXT FROM sessionsToKill INTO @sessionId
          WHILE @@FETCH_STATUS = 0
          BEGIN
            PRINT 'Killing session ' + CAST(@sessionId AS NVARCHAR(20)) + ' for login ' + @name
            SET @statement = 'KILL ' + CAST(@sessionId AS NVARCHAR(20))
            EXEC sp_executesql @statement
            FETCH NEXT FROM sessionsToKill INTO @sessionId
          END
          CLOSE sessionsToKill
          DEALLOCATE sessionsToKill`
	return c.ExecContext(ctx, cmd, sql.Named("name", name))
}
