package sqlserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"terraform-provider-sqlserver/sql"
	"terraform-provider-sqlserver/sqlserver/model"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type sqlserverProvider struct {
	factory model.ConnectorFactory
	logger  *zerolog.Logger
	host    string
	port    string
	login   interface{}
}

const (
	providerLogFile = "terraform-provider-sqlserver.log"
)

var (
	defaultTimeout = schema.DefaultTimeout(10 * time.Minute)
)

func New(version, commit string) func() *schema.Provider {
	return func() *schema.Provider {
		return Provider(sql.GetFactory())
	}
}

var LoginMethods = []string{
	"login",
	"azure_login",
	"azuread_default_chain_auth",
	"azuread_managed_identity_auth",
}

func Provider(factory model.ConnectorFactory) *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"debug": {
				Type:        schema.TypeBool,
				Description: fmt.Sprintf("Enable provider debug logging (logs to file %s)", providerLogFile),
				Optional:    true,
				Default:     false,
			},
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_HOST", nil),
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.EqualFold(old, new)
				},
			},
			"port": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_PORT", DefaultPort),
				Default:     DefaultPort,
			},
			"login": {
				Type:          schema.TypeList,
				MaxItems:      1,
				Optional:      true,
				ConflictsWith: []string{"azure_login", "azuread_default_chain_auth", "azuread_managed_identity_auth"},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"username": {
							Type:        schema.TypeString,
							Optional:    true,
							DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_USERNAME", nil),
						},
						"password": {
							Type:        schema.TypeString,
							Optional:    true,
							Sensitive:   true,
							DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_PASSWORD", nil),
						},
					},
				},
			},
			"azure_login": {
				Type:          schema.TypeList,
				MaxItems:      1,
				Optional:      true,
				ConflictsWith: []string{"login", "azuread_default_chain_auth", "azuread_managed_identity_auth"},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"tenant_id": {
							Type:        schema.TypeString,
							Optional:    true,
							DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_TENANT_ID", nil),
						},
						"client_id": {
							Type:        schema.TypeString,
							Optional:    true,
							DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_CLIENT_ID", nil),
						},
						"client_secret": {
							Type:        schema.TypeString,
							Optional:    true,
							Sensitive:   true,
							DefaultFunc: schema.EnvDefaultFunc("TF_SQLSERVER_CLIENT_SECRET", nil),
						},
					},
				},
			},
			"azuread_default_chain_auth": {
				Type:          schema.TypeList,
				MaxItems:      1,
				Optional:      true,
				ConflictsWith: []string{"login", "azure_login", "azuread_managed_identity_auth"},
				Elem:          &schema.Resource{},
			},
			"azuread_managed_identity_auth": {
				Type:          schema.TypeList,
				MaxItems:      1,
				Optional:      true,
				ConflictsWith: []string{"login", "azure_login", "azuread_default_chain_auth"},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"user_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"sqlserver_login": resourceLogin(),
			"sqlserver_user":  resourceUser(),
			"sqlserver_resource_governor": resourceResourceGovernor(),
			"sqlserver_resource_pool": resourceResourceGovernorResourcePool(),
			"sqlserver_workload_group": resourceResourceGovernorWorkloadGroup(),
			"sqlserver_classifier_function": resourceResourceGovernorClassifierFunction(),
		},
		DataSourcesMap: map[string]*schema.Resource{},
		ConfigureContextFunc: func(ctx context.Context, data *schema.ResourceData) (interface{}, diag.Diagnostics) {
			return providerConfigure(ctx, data, factory)
		},
	}
}

func providerConfigure(ctx context.Context, data *schema.ResourceData, factory model.ConnectorFactory) (model.Provider, diag.Diagnostics) {
	isDebug := data.Get("debug").(bool)
	host := data.Get("host").(string)
	port := data.Get("port").(string)
	logger := newLogger(isDebug)

	var login interface{}
	if admin, ok := data.GetOk("login"); ok {
		admin := admin.([]interface{})
		adminMap := admin[0].(map[string]interface{})
		login = model.SqlLogin{
			Username: adminMap["username"].(string),
			Password: adminMap["password"].(string),
		}
	}

	if admin, ok := data.GetOk("azure_login"); ok {
		admin := admin.([]interface{})
		adminMap := admin[0].(map[string]interface{})
		login = model.AzureLogin{
			TenantID:     adminMap["tenant_id"].(string),
			ClientID:     adminMap["client_id"].(string),
			ClientSecret: adminMap["client_secret"].(string),
		}
	}

	if admin, ok := data.GetOk("azuread_managed_identity_auth"); ok {
		admin := admin.([]interface{})
		adminMap := admin[0].(map[string]interface{})
		login = model.FedauthMSI{
			UserID: adminMap["user_id"].(string),
		}
	}

	logger.Info().Msgf("Created provider with %s:%s", host, port)

	return sqlserverProvider{factory: factory, logger: logger, host: host, port: port, login: login}, nil
}

func (p sqlserverProvider) GetConnector(data *schema.ResourceData) (interface{}, error) {
	return p.factory.GetConnector(data, p.host, p.port, p.login)
}

func (p sqlserverProvider) ResourceLogger(resource, function string) zerolog.Logger {
	return p.logger.With().Str("resource", resource).Str("func", function).Logger()
}

func (p sqlserverProvider) DataSourceLogger(datasource, function string) zerolog.Logger {
	return p.logger.With().Str("datasource", datasource).Str("func", function).Logger()
}

func newLogger(isDebug bool) *zerolog.Logger {
	var writer io.Writer = nil
	logLevel := zerolog.Disabled
	if isDebug {
		f, err := os.OpenFile(providerLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Err(err).Msg("error opening file")
		}
		writer = f
		logLevel = zerolog.DebugLevel
	}
	logger := zerolog.New(writer).Level(logLevel).With().Timestamp().Logger()
	return &logger
}
