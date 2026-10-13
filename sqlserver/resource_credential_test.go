package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestCredentialValidation(t *testing.T) {
	names := []struct {
		name     string
		severity diag.Severity // -1 for no diagnostics
	}{
		{"https://backupsa.blob.core.windows.net/sql-backups", -1},
		{"https://backupsa.blob.core.windows.net/sql-backups/", diag.Warning},
		{"HTTPS://backupsa.blob.core.windows.net/sql-backups/", diag.Warning},
		{"backup_credential", -1},
		{"not/a/url/", -1},
		{strings.Repeat("a", 128), -1},
		{strings.Repeat("a", 129), diag.Error},
		{"", diag.Error},
		{"   ", diag.Error},
	}
	for _, tt := range names {
		diags := validateCredentialName(tt.name, nil)
		switch {
		case tt.severity == -1 && len(diags) != 0:
			t.Errorf("name %q: expected no diagnostics, got %v", tt.name, diags)
		case tt.severity != -1 && (len(diags) != 1 || diags[0].Severity != tt.severity):
			t.Errorf("name %q: expected one diagnostic with severity %v, got %v", tt.name, tt.severity, diags)
		}
	}

	for identity, ok := range map[string]bool{
		"Managed Identity":         true,
		"managed identity":         true,
		"SHARED ACCESS SIGNATURE":  false,
		"S3 Access Key":            false,
		"backupsa":                 false,
		"Managed Identity, please": false,
	} {
		diags := validateCredentialIdentity(identity, nil)
		if diags.HasError() == ok {
			t.Errorf("identity %q: accepted = %v, want %v (%v)", identity, !diags.HasError(), ok, diags)
		}
	}
}

const testCredentialName = "https://tftestbackupsa.blob.core.windows.net/sql-backups"

func testAccCredentialConfig(name string) string {
	return testAccProviderBlock + fmt.Sprintf(`
      resource "sqlserver_credential" "backup" {
        name     = %q
        identity = "Managed Identity"
      }`, name)
}

func TestAccCredential_Local_ManagedIdentity(t *testing.T) {
	const renamed = "https://tftestbackupsa.blob.core.windows.net/other-container"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckCredentialDestroy(testCredentialName, renamed),
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialConfig(testCredentialName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCredential(testCredentialName, "Managed Identity"),
					resource.TestCheckResourceAttr("sqlserver_credential.backup", "name", testCredentialName),
					resource.TestCheckResourceAttr("sqlserver_credential.backup", "identity", "Managed Identity"),
					resource.TestCheckResourceAttrSet("sqlserver_credential.backup", "credential_id"),
				),
			},
			{
				// A new name replaces the credential: names can't be changed in SQL Server.
				Config: testAccCredentialConfig(renamed),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCredential(renamed, "Managed Identity"),
					testAccCheckCredentialDestroy(testCredentialName),
				),
			},
		},
	})
}

func TestAccCredential_Local_DeletedOutsideTerraform(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckCredentialDestroy(testCredentialName),
		Steps: []resource.TestStep{
			{Config: testAccCredentialConfig(testCredentialName)},
			{
				PreConfig: func() {
					connector, err := getTestConnector(nil)
					if err != nil {
						t.Fatal(err)
					}
					if err = connector.ExecContext(`DECLARE @sql nvarchar(max) = N'DROP CREDENTIAL ' + QUOTENAME(@name); EXEC (@sql)`,
						sql.Named("name", testCredentialName)); err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccCredentialConfig(testCredentialName),
				Check:  testAccCheckCredential(testCredentialName, "Managed Identity"),
			},
		},
	})
}

func TestAccCredential_Local_UnsupportedIdentity(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderBlock + `
                  resource "sqlserver_credential" "sas" {
                    name     = "https://tftestbackupsa.blob.core.windows.net/sql-backups"
                    identity = "SHARED ACCESS SIGNATURE"
                  }`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`identity "SHARED ACCESS SIGNATURE" isn't supported yet`),
			},
		},
	})
}

func TestAccCredential_Local_UsedByAgentJobStep(t *testing.T) {
	// A job step that names the credential in its command is created after it and deleted before it.
	config := testAccCredentialConfig(testCredentialName) + `
      resource "sqlserver_agent_job" "backup" {
        name = "tf_test_credential_backup"
      }
      resource "sqlserver_agent_job_step" "backup" {
        job_id  = sqlserver_agent_job.backup.job_id
        name    = "backup"
        command = "BACKUP DATABASE [master] TO URL = N'${sqlserver_credential.backup.name}/master.bak' WITH COPY_ONLY"
      }`
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy: func(state *terraform.State) error {
			if err := testAccCheckAgentJobDestroy(state); err != nil {
				return err
			}
			return testAccCheckCredentialDestroy(testCredentialName)(state)
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCredential(testCredentialName, "Managed Identity"),
					resource.TestCheckResourceAttr("sqlserver_agent_job_step.backup", "command",
						"BACKUP DATABASE [master] TO URL = N'"+testCredentialName+"/master.bak' WITH COPY_ONLY"),
				),
			},
		},
	})
}

func testAccCheckCredential(name, identity string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		connector, err := getTestConnector(nil)
		if err != nil {
			return err
		}
		cred, err := connector.GetCredential(name)
		if err != nil {
			return err
		}
		if cred == nil {
			return fmt.Errorf("credential [%s] does not exist", name)
		}
		if cred.Identity != identity {
			return fmt.Errorf("credential [%s] has identity %q, expected %q", name, cred.Identity, identity)
		}
		return nil
	}
}

func testAccCheckCredentialDestroy(names ...string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		connector, err := getTestConnector(nil)
		if err != nil {
			return err
		}
		for _, name := range names {
			cred, err := connector.GetCredential(name)
			if err != nil {
				return err
			}
			if cred != nil {
				return fmt.Errorf("credential [%s] still exists", name)
			}
		}
		return nil
	}
}

func TestAccCredential_Local_IdentityCaseAndDrift(t *testing.T) {
	config := func(identity string) string {
		return testAccProviderBlock + fmt.Sprintf(`
          resource "sqlserver_credential" "backup" {
            name     = %q
            identity = %q
          }`, testCredentialName, identity)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckCredentialDestroy(testCredentialName),
		Steps: []resource.TestStep{
			{
				// Any case is accepted and stored as written, so the plan afterwards is empty (checked by the framework).
				Config: config("managed identity"),
				Check:  testAccCheckCredential(testCredentialName, "managed identity"),
			},
			{
				// Changing only the case in the config replaces the credential, since identity is ForceNew.
				Config: config("Managed Identity"),
				Check:  testAccCheckCredential(testCredentialName, "Managed Identity"),
			},
			{
				// The identity is changed outside Terraform while the config stays the same: the refresh must notice,
				// and the apply replaces the credential.
				PreConfig: func() {
					connector, err := getTestConnector(nil)
					if err != nil {
						t.Fatal(err)
					}
					if err = connector.ExecContext(`DECLARE @sql nvarchar(max) = N'ALTER CREDENTIAL ' + QUOTENAME(@name) + N' WITH IDENTITY = N''SHARED ACCESS SIGNATURE'''; EXEC (@sql)`,
						sql.Named("name", testCredentialName)); err != nil {
						t.Fatal(err)
					}
				},
				Config: config("Managed Identity"),
				Check:  testAccCheckCredential(testCredentialName, "Managed Identity"),
			},
		},
	})
}

func TestAccCredential_Local_SpecialCharacters(t *testing.T) {
	// Brackets and quotes must survive the dynamic CREATE and DROP statements, and a 128-character name (the maximum)
	// must work.
	names := []string{
		"https://tftestbackupsa.blob.core.windows.net/it's]a[test",
		strings.Repeat("c", 121) + "]'][x'y",
	}
	var config strings.Builder
	config.WriteString(testAccProviderBlock)
	for i, name := range names {
		fmt.Fprintf(&config, `
          resource "sqlserver_credential" "c%d" {
            name     = %q
            identity = "Managed Identity"
          }`, i, name)
	}
	if len([]rune(names[1])) != 128 {
		t.Fatalf("the long name has %d characters, want 128", len([]rune(names[1])))
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckCredentialDestroy(names...),
		Steps: []resource.TestStep{
			{
				Config: config.String(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCredential(names[0], "Managed Identity"),
					testAccCheckCredential(names[1], "Managed Identity"),
				),
			},
		},
	})
}

func TestAccCredential_Local_NameTooLongForQuotename(t *testing.T) {
	// The plan-time check stops names over 128 characters, so call the SQL layer directly: QUOTENAME returns NULL for
	// them, and the statement must fail instead of silently running nothing.
	if !runLocalAccTests {
		t.Skip("needs a SQL Server (TF_ACC_LOCAL)")
	}
	testAccPreCheck(t)
	connector, err := getTestConnector(nil)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("d", 129)
	err = connector.(testConnector).c.(CredentialConnector).CreateCredential(context.Background(), name, "Managed Identity")
	if err == nil || !strings.Contains(err.Error(), "longer than 128 characters") {
		t.Fatalf("expected the 128-character error, got %v", err)
	}
	if cred, err := connector.GetCredential(name); err != nil || cred != nil {
		t.Fatalf("no credential should exist, got %v, %v", cred, err)
	}
}

func TestAccCredential_Local_DeleteWhileInUse(t *testing.T) {
	// SQL Server refuses to drop a credential a login is mapped to; the error must reach the user, with the name.
	const login = "tf_test_credential_login"
	if runLocalAccTests {
		defer func() {
			connector, _ := getTestConnector(nil)
			_ = connector.ExecContext(`DECLARE @sql nvarchar(max) = N'DROP LOGIN ' + QUOTENAME(@login); IF SUSER_ID(@login) IS NOT NULL EXEC (@sql)`, sql.Named("login", login))
		}()
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		IsUnitTest:        runLocalAccTests,
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckCredentialDestroy(testCredentialName),
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialConfig(testCredentialName),
				Check: func(*terraform.State) error {
					connector, err := getTestConnector(nil)
					if err != nil {
						return err
					}
					return connector.ExecContext(`DECLARE @sql nvarchar(max) = N'CREATE LOGIN ' + QUOTENAME(@login) + N' WITH PASSWORD = ''Cred!Passw0rd1'', CREDENTIAL = ' + QUOTENAME(@name); EXEC (@sql)`,
						sql.Named("login", login), sql.Named("name", testCredentialName))
				},
			},
			{
				Config:      testAccProviderBlock,
				ExpectError: regexp.MustCompile(`unable to delete credential \[` + regexp.QuoteMeta(testCredentialName) + `\]`),
			},
			{
				// Remove the login that uses the credential, then the credential can be deleted.
				PreConfig: func() {
					connector, err := getTestConnector(nil)
					if err != nil {
						t.Fatal(err)
					}
					if err = connector.ExecContext(`DECLARE @sql nvarchar(max) = N'DROP LOGIN ' + QUOTENAME(@login); EXEC (@sql)`,
						sql.Named("login", login)); err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccProviderBlock,
			},
		},
	})
}
