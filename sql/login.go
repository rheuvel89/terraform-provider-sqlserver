package sql

import (
	"context"
	"database/sql"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetLogin(ctx context.Context, name string) (*model.Login, error) {
	var (
		login model.Login
		roles string
	)
	err := c.QueryRowContext(ctx,
		`SELECT p.[principal_id], p.[name], CONVERT(VARCHAR(1000), p.[sid], 1), p.[type_desc], COALESCE(STRING_AGG(r.[name], ',') WITHIN GROUP (ORDER BY r.[name]), ''), COALESCE(p.[is_disabled], 0)
         FROM sys.server_principals p
           LEFT JOIN sys.sql_logins l ON p.principal_id = l.principal_id
           LEFT JOIN sys.server_role_members rm ON p.principal_id = rm.member_principal_id
           LEFT JOIN sys.server_principals r ON rm.role_principal_id = r.principal_id AND r.[type] = 'R' AND r.[name] != 'public'
         WHERE p.[name] = @name
         GROUP BY p.[principal_id], p.[name], p.[sid], p.[type_desc], p.[is_disabled]`,
		func(r *sql.Row) error {
			result := r.Scan(&login.PrincipalID, &login.LoginName, &login.SIDStr, &login.SourceType, &roles, &login.IsDisabled)
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
	if roles == "" {
		login.Roles = make([]string, 0)
	} else {
		login.Roles = strings.Split(roles, ",")
	}
	return &login, nil
}

func (c *Connector) CreateLogin(ctx context.Context, name, password, sourceType string, roles []string) error {
	cmd := `DECLARE @sql nvarchar(max)
          IF @sourceType = 'EXTERNAL_GROUP' OR @sourceType = 'EXTERNAL_USER'
            BEGIN
              SET @sql = 'CREATE LOGIN ' + QuoteName(@name) + ' FROM EXTERNAL PROVIDER'
            END
          ELSE
            BEGIN
              SET @sql = 'CREATE LOGIN ' + QuoteName(@name) + ' WITH PASSWORD = ' + QuoteName(@password, '''')
            END
          EXEC (@sql)

          DECLARE role_cur CURSOR FAST_FORWARD FOR
            SELECT [name] FROM sys.server_principals
            WHERE [type] = 'R' AND [name] != 'public'
              AND [name] COLLATE SQL_Latin1_General_CP1_CI_AS IN (SELECT value FROM STRING_SPLIT(@roles, ','))
          DECLARE @role nvarchar(max)
          DECLARE @roleSql nvarchar(max)
          OPEN role_cur
          FETCH NEXT FROM role_cur INTO @role
          WHILE @@FETCH_STATUS = 0
          BEGIN
            SET @roleSql = 'ALTER SERVER ROLE ' + QuoteName(@role) + ' ADD MEMBER ' + QuoteName(@name)
            EXEC (@roleSql)
            FETCH NEXT FROM role_cur INTO @role
          END
          CLOSE role_cur
          DEALLOCATE role_cur`

	database := "master"
	return c.
		setDatabase(&database).
		ExecContext(ctx, cmd,
			sql.Named("name", name),
			sql.Named("password", password),
			sql.Named("sourceType", sourceType),
			sql.Named("roles", strings.Join(roles, ",")))
}

func (c *Connector) CreateLoginWithOptions(ctx context.Context, name, password, sourceType string, roles []string, isDisabled bool) error {
	cmd := `DECLARE @sql nvarchar(max)
          IF @sourceType = 'EXTERNAL_GROUP' OR @sourceType = 'EXTERNAL_USER'
            BEGIN
              SET @sql = 'CREATE LOGIN ' + QuoteName(@name) + ' FROM EXTERNAL PROVIDER'
            END
          ELSE
            BEGIN
              SET @sql = 'CREATE LOGIN ' + QuoteName(@name) + ' WITH PASSWORD = ' + QuoteName(@password, '''')
            END
          EXEC (@sql)

          -- CREATE LOGIN has no DISABLE option, so disable the login afterwards if requested
          IF @isDisabled = 1
            BEGIN
              SET @sql = 'ALTER LOGIN ' + QuoteName(@name) + ' DISABLE'
              EXEC (@sql)
            END

          DECLARE role_cur CURSOR FAST_FORWARD FOR
            SELECT [name] FROM sys.server_principals
            WHERE [type] = 'R' AND [name] != 'public'
              AND [name] COLLATE SQL_Latin1_General_CP1_CI_AS IN (SELECT value FROM STRING_SPLIT(@roles, ','))
          DECLARE @role nvarchar(max)
          DECLARE @roleSql nvarchar(max)
          OPEN role_cur
          FETCH NEXT FROM role_cur INTO @role
          WHILE @@FETCH_STATUS = 0
          BEGIN
            SET @roleSql = 'ALTER SERVER ROLE ' + QuoteName(@role) + ' ADD MEMBER ' + QuoteName(@name)
            EXEC (@roleSql)
            FETCH NEXT FROM role_cur INTO @role
          END
          CLOSE role_cur
          DEALLOCATE role_cur`

	database := "master"
	return c.
		setDatabase(&database).
		ExecContext(ctx, cmd,
			sql.Named("name", name),
			sql.Named("password", password),
			sql.Named("sourceType", sourceType),
			sql.Named("roles", strings.Join(roles, ",")),
			sql.Named("isDisabled", isDisabled))
}

func (c *Connector) SetLoginDisableState(ctx context.Context, name string, isDisabled bool) error {
	cmd := `DECLARE @sql nvarchar(max)
          SET @sql = 'ALTER LOGIN ' + QuoteName(@name) + ' ' +
                     CASE WHEN @isDisabled = 1 THEN 'DISABLE' ELSE 'ENABLE' END
          EXEC (@sql)`
	return c.ExecContext(ctx, cmd,
		sql.Named("name", name),
		sql.Named("isDisabled", isDisabled))
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

func (c *Connector) UpdateLoginRoles(ctx context.Context, name string, roles []string) error {
	cmd := `DECLARE @role_name nvarchar(max);
          DECLARE @cmd nvarchar(max);

          -- 1. Remove roles the login has but shouldn't
          DECLARE del_role_cur CURSOR FAST_FORWARD FOR
            SELECT r.[name]
            FROM sys.server_role_members rm
              JOIN sys.server_principals r ON rm.role_principal_id = r.principal_id
              JOIN sys.server_principals m ON rm.member_principal_id = m.principal_id
            WHERE m.[name] = @name
              AND r.[name] != 'public'
              AND r.[name] COLLATE SQL_Latin1_General_CP1_CI_AS NOT IN (SELECT value FROM STRING_SPLIT(@roles, ','))

          OPEN del_role_cur
          FETCH NEXT FROM del_role_cur INTO @role_name
          WHILE @@FETCH_STATUS = 0
          BEGIN
            SET @cmd = 'ALTER SERVER ROLE ' + QuoteName(@role_name) + ' DROP MEMBER ' + QuoteName(@name)
            EXEC (@cmd)
            FETCH NEXT FROM del_role_cur INTO @role_name
          END
          CLOSE del_role_cur
          DEALLOCATE del_role_cur

          -- 2. Add roles the login needs but doesn't have
          DECLARE add_role_cur CURSOR FAST_FORWARD FOR
            SELECT [name] FROM sys.server_principals
            WHERE [type] = 'R' AND [name] != 'public'
              AND [name] NOT IN (
                SELECT r.[name] FROM sys.server_role_members rm
                  JOIN sys.server_principals r ON rm.role_principal_id = r.principal_id
                  JOIN sys.server_principals m ON rm.member_principal_id = m.principal_id
                WHERE m.[name] = @name
              )
              AND [name] COLLATE SQL_Latin1_General_CP1_CI_AS IN (SELECT value FROM STRING_SPLIT(@roles, ','))

          OPEN add_role_cur
          FETCH NEXT FROM add_role_cur INTO @role_name
          WHILE @@FETCH_STATUS = 0
          BEGIN
            SET @cmd = 'ALTER SERVER ROLE ' + QuoteName(@role_name) + ' ADD MEMBER ' + QuoteName(@name)
            EXEC (@cmd)
            FETCH NEXT FROM add_role_cur INTO @role_name
          END
          CLOSE add_role_cur
          DEALLOCATE add_role_cur`
	return c.ExecContext(ctx, cmd,
		sql.Named("name", name),
		sql.Named("roles", strings.Join(roles, ",")))
}

func (c *Connector) DeleteLogin(ctx context.Context, name string) error {
	if err := c.killSessionsForLogin(ctx, name); err != nil {
		return err
	}
	cmd := `DECLARE @sql nvarchar(max)
          SET @sql = 'IF EXISTS (SELECT 1 FROM [master].[sys].[server_principals] WHERE [name] = ' + QuoteName(@name, '''') + ') ' +
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
