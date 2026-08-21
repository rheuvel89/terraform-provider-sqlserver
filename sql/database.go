package sql

import (
	"context"
	"database/sql"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetDatabase(ctx context.Context, name string) (*model.Database, error) {
	var db model.Database
	err := c.QueryRowContext(ctx,
		`SELECT [name], COALESCE(SUSER_SNAME([owner_sid]), ''), COALESCE([collation_name], ''), COALESCE([recovery_model_desc], ''), [compatibility_level]
         FROM [sys].[databases]
         WHERE [name] = @name`,
		func(r *sql.Row) error {
			return r.Scan(&db.Name, &db.Owner, &db.Collation, &db.RecoveryModel, &db.CompatibilityLevel)
		},
		sql.Named("name", name),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &db, nil
}

func (c *Connector) CreateDatabase(ctx context.Context, name, collation string) error {
	cmd := `DECLARE @sql nvarchar(max)
          IF @collation = ''
            BEGIN
              SET @sql = 'CREATE DATABASE ' + QuoteName(@name)
            END
          ELSE
            BEGIN
              SET @sql = 'CREATE DATABASE ' + QuoteName(@name) + ' COLLATE ' + @collation
            END
          EXEC (@sql)`
	return c.ExecContext(ctx, cmd,
		sql.Named("name", name),
		sql.Named("collation", collation))
}

func (c *Connector) UpdateDatabase(ctx context.Context, name string, db *model.Database) error {
	// We build individual ALTER statements for each property that can be changed
	cmd := `DECLARE @sql nvarchar(max) = ''

          IF @collation != ''
            BEGIN
              SET @sql = @sql + 'ALTER DATABASE ' + QuoteName(@name) + ' COLLATE ' + @collation + '; '
            END

          IF @recoveryModel != ''
            BEGIN
              SET @sql = @sql + 'ALTER DATABASE ' + QuoteName(@name) + ' SET RECOVERY ' + @recoveryModel + '; '
            END

          IF @compatibilityLevel > 0
            BEGIN
              SET @sql = @sql + 'ALTER DATABASE ' + QuoteName(@name) + ' SET COMPATIBILITY_LEVEL = ' + CAST(@compatibilityLevel AS nvarchar(10)) + '; '
            END

          IF @sql != ''
            BEGIN
              EXEC (@sql)
            END`
	return c.ExecContext(ctx, cmd,
		sql.Named("name", name),
		sql.Named("collation", db.Collation),
		sql.Named("recoveryModel", db.RecoveryModel),
		sql.Named("compatibilityLevel", db.CompatibilityLevel))
}

func (c *Connector) DeleteDatabase(ctx context.Context, name string) error {
	cmd := `DECLARE @sql nvarchar(max)
          SET @sql = 'IF EXISTS (SELECT 1 FROM [sys].[databases] WHERE [name] = ' + QuoteName(@name, '''') + ') ' +
                      'DROP DATABASE ' + QuoteName(@name)
          EXEC (@sql)`
	return c.ExecContext(ctx, cmd, sql.Named("name", name))
}
