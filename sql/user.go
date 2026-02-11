package sql

import (
	"context"
	"database/sql"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetUser(ctx context.Context, database, username string) (*model.User, error) {
	cmd := `DECLARE @stmt nvarchar(max)
          IF @@VERSION LIKE 'Microsoft SQL Azure%'
            BEGIN
              SET @stmt = 'WITH CTE_Roles (principal_id, role_principal_id) AS ' +
                          '(' +
                          '  SELECT member_principal_id, role_principal_id FROM [sys].[database_role_members] WHERE member_principal_id = DATABASE_PRINCIPAL_ID(' + QuoteName(@username, '''') + ')' +
                          '  UNION ALL ' +
                          '  SELECT member_principal_id, drm.role_principal_id FROM [sys].[database_role_members] drm' +
                          '    INNER JOIN CTE_Roles cr ON drm.member_principal_id = cr.role_principal_id' +
                          ') ' +
                          'SELECT p.principal_id, p.name, p.authentication_type_desc, p.sid, CONVERT(VARCHAR(1000), p.sid, 1) AS sidStr, '''', COALESCE(STRING_AGG(USER_NAME(r.role_principal_id), '',''), '''') ' +
                          'FROM [sys].[database_principals] p' +
                          '  LEFT JOIN CTE_Roles r ON p.principal_id = r.principal_id ' +
                          'WHERE p.name = ' + QuoteName(@username, '''') + ' ' +
                          'GROUP BY p.principal_id, p.name, p.authentication_type_desc, p.sid'
            END
          ELSE
            BEGIN
              SET @stmt = 'WITH CTE_Roles (principal_id, role_principal_id) AS ' +
                          '(' +
                          '  SELECT member_principal_id, role_principal_id FROM ' + QuoteName(@database) + '.[sys].[database_role_members] WHERE member_principal_id = DATABASE_PRINCIPAL_ID(' + QuoteName(@username, '''') + ')' +
                          '  UNION ALL ' +
                          '  SELECT member_principal_id, drm.role_principal_id FROM ' + QuoteName(@database) + '.[sys].[database_role_members] drm' +
                          '    INNER JOIN CTE_Roles cr ON drm.member_principal_id = cr.role_principal_id' +
                          ') ' +
                          'SELECT p.principal_id, p.name, p.authentication_type_desc, p.sid, CONVERT(VARCHAR(1000), p.sid, 1) AS sidStr, COALESCE(sl.name, ''''), COALESCE(STRING_AGG(USER_NAME(r.role_principal_id), '',''), '''') ' +
                          'FROM ' + QuoteName(@database) + '.[sys].[database_principals] p' +
                          '  LEFT JOIN CTE_Roles r ON p.principal_id = r.principal_id ' +
                          '  LEFT JOIN [master].[sys].[sql_logins] sl ON p.sid = sl.sid ' +
                          'WHERE p.name = ' + QuoteName(@username, '''') + ' ' +
                          'GROUP BY p.principal_id, p.name, p.authentication_type_desc, p.sid, sl.name'
            END
          EXEC (@stmt)`
	var (
		user  model.User
		sid   []byte
		roles string
	)
	err := c.
		setDatabase(&database).
		QueryRowContext(ctx, cmd,
			func(r *sql.Row) error {
				return r.Scan(&user.PrincipalID, &user.Username, &user.AuthType, &sid, &user.SIDStr, &user.LoginName, &roles)
			},
			sql.Named("database", database),
			sql.Named("username", username),
		)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if user.AuthType == "INSTANCE" && user.LoginName == "" {
		cmd = "SELECT name FROM [sys].[sql_logins] WHERE sid = @sid"
		c.Database = "master"
		err = c.QueryRowContext(ctx, cmd,
			func(r *sql.Row) error {
				return r.Scan(&user.LoginName)
			},
			sql.Named("sid", sid),
		)
		if err != nil {
			return nil, err
		}
	}
	if roles == "" {
		user.Roles = make([]string, 0)
	} else {
		user.Roles = strings.Split(roles, ",")
	}
	return &user, nil
}

func (c *Connector) CreateUser(ctx context.Context, database string, user *model.User) error {
	cmd := `DECLARE @stmt nvarchar(max)
          IF @authType = 'INSTANCE'
            BEGIN
              SET @stmt = 'CREATE USER ' + QuoteName(@username) + ' FOR LOGIN ' + QuoteName(@loginName)
            END
          IF @authType = 'DATABASE'
            BEGIN
              SET @stmt = 'CREATE USER ' + QuoteName(@username) + ' WITH PASSWORD = ' + QuoteName(@password, '''')
            END
          IF @authType = 'EXTERNAL'
            BEGIN
              SET @stmt = 'CREATE USER ' + QuoteName(@username) + ' FROM EXTERNAL PROVIDER'
            END

          SET @stmt = @stmt + '; ' +
                      'DECLARE role_cur CURSOR FOR SELECT name FROM ' + QuoteName(@database) + '.[sys].[database_principals] WHERE type = ''R'' AND name != ''public'' AND name COLLATE SQL_Latin1_General_CP1_CI_AS IN (SELECT value FROM STRING_SPLIT(' + QuoteName(@roles, '''') + ', '',''));' +
                      'DECLARE @role nvarchar(max);' +
                      'OPEN role_cur;' +
                      'FETCH NEXT FROM role_cur INTO @role;' +
                      'WHILE @@FETCH_STATUS = 0' +
                      '  BEGIN' +
                      '    DECLARE @sql nvarchar(max);' +
                      '    SET @sql = ''ALTER ROLE '' + QuoteName(@role) + '' ADD MEMBER ' + QuoteName(@username) + ''';' +
                      '    EXEC (@sql);' +
                      '    FETCH NEXT FROM role_cur INTO @role;' +
                      '  END;' +
                      'CLOSE role_cur;' +
                      'DEALLOCATE role_cur;'
          EXEC (@stmt)`
	if user.AuthType != "EXTERNAL" {
		// External users do not have a server login
		_, err := c.GetLogin(ctx, user.LoginName)
		if err != nil {
			return err
		}
	}
	return c.
		setDatabase(&database).
		ExecContext(ctx, cmd,
			sql.Named("database", database),
			sql.Named("username", user.Username),
			sql.Named("loginName", user.LoginName),
			sql.Named("password", user.Password),
			sql.Named("authType", user.AuthType),
			sql.Named("roles", strings.Join(user.Roles, ",")),
		)
}

func (c *Connector) UpdateUser(ctx context.Context, database string, user *model.User) error {
	// We build a dynamic SQL string (@stmt) that constructs the cursor logic.
	// This double-dynamic approach allows us to inject the specific @database name
	// into the query strings for table references.
	cmd := `
	DECLARE @stmt nvarchar(max);
	SET @stmt =  'DECLARE @role_name nvarchar(max); ' +
				 'DECLARE @cmd nvarchar(max); ' +

				 /* 1. CURSOR: Identify and remove roles the user has but shouldn't */
				 'DECLARE del_role_cur CURSOR FOR ' +
				 'SELECT name FROM ' + QuoteName(@database) + '.[sys].[database_principals] ' +
				 'WHERE type = ''R'' AND name != ''public'' ' +
				 'AND name IN (' +
					'SELECT r.name FROM ' + QuoteName(@database) + '.[sys].[database_role_members] drm ' +
					'JOIN ' + QuoteName(@database) + '.[sys].[database_principals] r ON drm.role_principal_id = r.principal_id ' +
					'JOIN ' + QuoteName(@database) + '.[sys].[database_principals] m ON drm.member_principal_id = m.principal_id ' +
					'WHERE m.name = ' + QuoteName(@username, '''') +
				 ') ' +
				 'AND name COLLATE SQL_Latin1_General_CP1_CI_AS NOT IN (SELECT value FROM STRING_SPLIT(' + QuoteName(@roles, '''') + ', '','')); ' +

				 'OPEN del_role_cur; ' +
				 'FETCH NEXT FROM del_role_cur INTO @role_name; ' +
				 'WHILE @@FETCH_STATUS = 0 ' +
				 'BEGIN ' +
					'SET @cmd = ''ALTER ROLE '' + QuoteName(@role_name) + '' DROP MEMBER ' + QuoteName(@username) + '''; ' +
					'EXEC (@cmd); ' +
					'FETCH NEXT FROM del_role_cur INTO @role_name; ' +
				 'END; ' +
				 'CLOSE del_role_cur; ' +
				 'DEALLOCATE del_role_cur; ' +

				 /* 2. CURSOR: Identify and add roles the user needs but doesn't have */
				 'DECLARE add_role_cur CURSOR FOR ' +
				 'SELECT name FROM ' + QuoteName(@database) + '.[sys].[database_principals] ' +
				 'WHERE type = ''R'' AND name != ''public'' ' +
				 'AND name NOT IN (' +
					'SELECT r.name FROM ' + QuoteName(@database) + '.[sys].[database_role_members] drm ' +
					'JOIN ' + QuoteName(@database) + '.[sys].[database_principals] r ON drm.role_principal_id = r.principal_id ' +
					'JOIN ' + QuoteName(@database) + '.[sys].[database_principals] m ON drm.member_principal_id = m.principal_id ' +
					'WHERE m.name = ' + QuoteName(@username, '''') +
				 ') ' +
				 'AND name COLLATE SQL_Latin1_General_CP1_CI_AS IN (SELECT value FROM STRING_SPLIT(' + QuoteName(@roles, '''') + ', '','')); ' +

				 'OPEN add_role_cur; ' +
				 'FETCH NEXT FROM add_role_cur INTO @role_name; ' +
				 'WHILE @@FETCH_STATUS = 0 ' +
				 'BEGIN ' +
					'SET @cmd = ''ALTER ROLE '' + QuoteName(@role_name) + '' ADD MEMBER ' + QuoteName(@username) + '''; ' +
					'EXEC (@cmd); ' +
					'FETCH NEXT FROM add_role_cur INTO @role_name; ' +
				 'END; ' +
				 'CLOSE add_role_cur; ' +
				 'DEALLOCATE add_role_cur; '

	EXEC (@stmt)
	`

	return c.
		setDatabase(&database).
		ExecContext(ctx, cmd,
			sql.Named("database", database),
			sql.Named("username", user.Username),
			sql.Named("roles", strings.Join(user.Roles, ",")),
		)
}

func (c *Connector) DeleteUser(ctx context.Context, database, username string) error {
	cmd := `DECLARE @stmt nvarchar(max)
          SET @stmt = 'IF EXISTS (SELECT 1 FROM ' + QuoteName(@database) + '.[sys].[database_principals] WHERE [name] = ' + QuoteName(@username, '''') + ') ' +
                      'DROP USER ' + QuoteName(@username)
          EXEC (@stmt)`
	return c.
		setDatabase(&database).
		ExecContext(ctx, cmd, sql.Named("database", database), sql.Named("username", username))
}

func (c *Connector) setDatabase(database *string) *Connector {
	if *database == "" {
		*database = "master"
	}
	c.Database = *database
	return c
}
