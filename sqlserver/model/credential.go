package model

// Credential is a server-level credential (see sys.credentials).
type Credential struct {
	CredentialID int
	Name         string
	Identity     string
}
