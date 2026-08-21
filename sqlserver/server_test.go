package sqlserver

import (
	"os"
	"testing"
)

// saveEnv stores the current value of a set of env vars and returns a restore function.
func saveEnv(keys ...string) func() {
	orig := make(map[string]string)
	for _, k := range keys {
		orig[k] = os.Getenv(k)
	}
	return func() {
		for k, v := range orig {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}
}

// isNilOrEmptySlice returns true when v is nil or an empty []map[string]interface{}.
func isNilOrEmptySlice(v interface{}) bool {
	if v == nil {
		return true
	}
	s, ok := v.([]map[string]interface{})
	if !ok {
		return false
	}
	return len(s) == 0
}

func TestServerFromId_NoCredentials(t *testing.T) {
	// Clear env vars that getLogin / getAzureLogin might pick up as fallback.
	defer saveEnv("TF_SQLSERVER_USERNAME", "TF_SQLSERVER_PASSWORD",
		"TF_SQLSERVER_TENANT_ID", "TF_SQLSERVER_CLIENT_ID", "TF_CLIENT_SECRET")()
	os.Unsetenv("TF_SQLSERVER_USERNAME")
	os.Unsetenv("TF_SQLSERVER_PASSWORD")
	os.Unsetenv("TF_SQLSERVER_TENANT_ID")
	os.Unsetenv("TF_SQLSERVER_CLIENT_ID")
	os.Unsetenv("TF_CLIENT_SECRET")

	server, u, err := serverFromId("sqlserver://10.230.6.4:1433/login/sqladmin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u == nil {
		t.Fatal("expected non-nil URL")
	}
	if u.Path != "/login/sqladmin" {
		t.Errorf("expected path /login/sqladmin, got %s", u.Path)
	}

	// Verify host and port
	if host, ok := server[0]["host"].(string); !ok || host != "10.230.6.4" {
		t.Errorf("expected host 10.230.6.4, got %v", server[0]["host"])
	}
	if port, ok := server[0]["port"].(string); !ok || port != "1433" {
		t.Errorf("expected port 1433, got %v", server[0]["port"])
	}

	// login should be an empty map (the fix: no error, just empty login data)
	login := server[0]["login"]
	if login == nil {
		t.Error("expected non-nil login (empty map) when no credentials in ID")
	}
	loginMap, ok := login.([]map[string]interface{})
	if !ok || len(loginMap) != 1 {
		t.Fatalf("expected login to be []map[string]interface{} with 1 element, got %T len=%d", login, len(loginMap))
	}
	if len(loginMap[0]) != 0 {
		t.Errorf("expected empty login map, got %v", loginMap[0])
	}

	// azure_login should be nil or empty (no Azure credentials available)
	if !isNilOrEmptySlice(server[0]["azure_login"]) {
		t.Errorf("expected azure_login to be nil or empty, got %v", server[0]["azure_login"])
	}
}

func TestServerFromId_NoCredentials_DefaultPort(t *testing.T) {
	defer saveEnv("TF_SQLSERVER_USERNAME", "TF_SQLSERVER_PASSWORD",
		"TF_SQLSERVER_TENANT_ID", "TF_SQLSERVER_CLIENT_ID", "TF_CLIENT_SECRET")()
	os.Unsetenv("TF_SQLSERVER_USERNAME")
	os.Unsetenv("TF_SQLSERVER_PASSWORD")
	os.Unsetenv("TF_SQLSERVER_TENANT_ID")
	os.Unsetenv("TF_SQLSERVER_CLIENT_ID")
	os.Unsetenv("TF_CLIENT_SECRET")

	server, u, err := serverFromId("sqlserver://myhost/login/sqladmin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host, ok := server[0]["host"].(string); !ok || host != "myhost" {
		t.Errorf("expected host myhost, got %v", server[0]["host"])
	}
	if port, ok := server[0]["port"].(string); !ok || port != "1433" {
		t.Errorf("expected default port 1433, got %v", server[0]["port"])
	}
	_ = u
}

func TestServerFromId_SQLCredentialsInUrl(t *testing.T) {
	defer saveEnv("TF_SQLSERVER_USERNAME", "TF_SQLSERVER_PASSWORD",
		"TF_SQLSERVER_TENANT_ID", "TF_SQLSERVER_CLIENT_ID", "TF_CLIENT_SECRET")()
	os.Unsetenv("TF_SQLSERVER_USERNAME")
	os.Unsetenv("TF_SQLSERVER_PASSWORD")
	os.Unsetenv("TF_SQLSERVER_TENANT_ID")
	os.Unsetenv("TF_SQLSERVER_CLIENT_ID")
	os.Unsetenv("TF_CLIENT_SECRET")

	server, _, err := serverFromId("sqlserver://10.230.6.4:1433/login/sqladmin?username=sa&password=MyP@ss")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	login := server[0]["login"].([]map[string]interface{})[0]
	if login["username"] != "sa" {
		t.Errorf("expected username 'sa', got %v", login["username"])
	}
	if login["password"] != "MyP@ss" {
		t.Errorf("expected password 'MyP@ss', got %v", login["password"])
	}

	// azure_login should be nil or empty when not provided
	if !isNilOrEmptySlice(server[0]["azure_login"]) {
		t.Errorf("expected azure_login to be nil or empty, got %v", server[0]["azure_login"])
	}
}

func TestServerFromId_AzureCredentialsInUrl(t *testing.T) {
	defer saveEnv("TF_SQLSERVER_USERNAME", "TF_SQLSERVER_PASSWORD",
		"TF_SQLSERVER_TENANT_ID", "TF_SQLSERVER_CLIENT_ID", "TF_CLIENT_SECRET")()
	os.Unsetenv("TF_SQLSERVER_USERNAME")
	os.Unsetenv("TF_SQLSERVER_PASSWORD")
	os.Unsetenv("TF_SQLSERVER_TENANT_ID")
	os.Unsetenv("TF_SQLSERVER_CLIENT_ID")
	os.Unsetenv("TF_CLIENT_SECRET")

	server, _, err := serverFromId("sqlserver://10.230.6.4:1433/login/sqladmin?tenant_id=t1&client_id=c1&client_secret=s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	azureLogin := server[0]["azure_login"].([]map[string]interface{})[0]
	if azureLogin["tenant_id"] != "t1" {
		t.Errorf("expected tenant_id 't1', got %v", azureLogin["tenant_id"])
	}
	if azureLogin["client_id"] != "c1" {
		t.Errorf("expected client_id 'c1', got %v", azureLogin["client_id"])
	}
	if azureLogin["client_secret"] != "s1" {
		t.Errorf("expected client_secret 's1', got %v", azureLogin["client_secret"])
	}

	// login should be nil or empty when not provided
	if !isNilOrEmptySlice(server[0]["login"]) {
		t.Errorf("expected login to be nil or empty, got %v", server[0]["login"])
	}
}

func TestServerFromId_InvalidScheme(t *testing.T) {
	_, _, err := serverFromId("https://host/login/sqladmin")
	if err == nil {
		t.Fatal("expected error for invalid scheme")
	}
	if err.Error() != "invalid schema in ID" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestServerFromId_BothLoginTypesInValues(t *testing.T) {
	defer saveEnv("TF_SQLSERVER_USERNAME", "TF_SQLSERVER_PASSWORD",
		"TF_SQLSERVER_TENANT_ID", "TF_SQLSERVER_CLIENT_ID", "TF_CLIENT_SECRET")()
	os.Unsetenv("TF_SQLSERVER_USERNAME")
	os.Unsetenv("TF_SQLSERVER_PASSWORD")
	os.Unsetenv("TF_SQLSERVER_TENANT_ID")
	os.Unsetenv("TF_SQLSERVER_CLIENT_ID")
	os.Unsetenv("TF_CLIENT_SECRET")

	// Both explicitly in values (inValues=true for both) → should error
	_, _, err := serverFromId("sqlserver://host/login/u?username=u&password=p&tenant_id=t&client_id=c&client_secret=s")
	if err == nil {
		t.Fatal("expected error when both login types are explicitly in values")
	}
	if err.Error() != "both login and azure login specified in resource" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestServerFromId_MssqlScheme(t *testing.T) {
	defer saveEnv("TF_SQLSERVER_USERNAME", "TF_SQLSERVER_PASSWORD")()
	os.Unsetenv("TF_SQLSERVER_USERNAME")
	os.Unsetenv("TF_SQLSERVER_PASSWORD")

	server, _, err := serverFromId("mssql://10.230.6.4:1433/login/sqladmin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host, ok := server[0]["host"].(string); !ok || host != "10.230.6.4" {
		t.Errorf("expected host 10.230.6.4, got %v", server[0]["host"])
	}
}

func TestServerFromId_InvalidUrl(t *testing.T) {
	_, _, err := serverFromId("://invalid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
