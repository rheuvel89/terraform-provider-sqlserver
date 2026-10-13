package sql

import (
	"context"
	"database/sql"
	"terraform-provider-sqlserver/sqlserver/model"
)

// EnsureCredentialsAvailable returns a clear error when the server has no server-level credentials.
func (c *Connector) EnsureCredentialsAvailable(ctx context.Context) error {
	return c.ensureNotAzureSQLDatabase(ctx, "Azure SQL Database has no server-level credentials; sqlserver_credential needs Azure SQL Managed Instance or SQL Server (on-premises or VM)")
}

func (c *Connector) GetCredential(ctx context.Context, name string) (*model.Credential, error) {
	var cred model.Credential
	err := c.QueryRowContext(ctx,
		`SELECT [credential_id], [name], COALESCE([credential_identity], '')
         FROM [sys].[credentials]
         WHERE [name] = @name`,
		func(r *sql.Row) error {
			return r.Scan(&cred.CredentialID, &cred.Name, &cred.Identity)
		},
		sql.Named("name", name),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cred, nil
}

// CreateCredential creates a credential without a secret, such as a managed identity credential. CREATE CREDENTIAL
// doesn't take parameters, so the statement is built in T-SQL. QUOTENAME returns NULL for names over 128 characters,
// which would make EXEC run nothing; the statement fails instead.
func (c *Connector) CreateCredential(ctx context.Context, name, identity string) error {
	cmd := `DECLARE @sql nvarchar(max) =
            N'CREATE CREDENTIAL ' + QUOTENAME(@name) +
            N' WITH IDENTITY = N''' + REPLACE(@identity, N'''', N'''''') + N''''
          IF @sql IS NULL
            THROW 50000, N'The credential name is longer than 128 characters.', 1
          EXEC (@sql)`
	return c.ExecContext(ctx, cmd,
		sql.Named("name", name),
		sql.Named("identity", identity))
}

func (c *Connector) DeleteCredential(ctx context.Context, name string) error {
	cmd := `IF EXISTS (SELECT 1 FROM [sys].[credentials] WHERE [name] = @name)
          BEGIN
            DECLARE @sql nvarchar(max) = N'DROP CREDENTIAL ' + QUOTENAME(@name)
            EXEC (@sql)
          END`
	return c.ExecContext(ctx, cmd, sql.Named("name", name))
}
