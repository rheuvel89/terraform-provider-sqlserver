package model

type Login struct {
	PrincipalID int64
	LoginName   string
	SIDStr      string
	SourceType  string
	Roles       []string
	IsDisabled  bool
}

type SqlLogin struct {
	Username string
	Password string
}

type AzureLogin struct {
	TenantID     string
	ClientID     string
	ClientSecret string
}

type FedauthMSI struct {
	UserID string
}
