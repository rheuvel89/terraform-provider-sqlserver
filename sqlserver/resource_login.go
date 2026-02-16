package sqlserver

import (
	"context"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
)

type LoginConnector interface {
	CreateLogin(ctx context.Context, name string, password string, loginSourceType string) error
	GetLogin(ctx context.Context, name string) (*model.Login, error)
	UpdateLogin(ctx context.Context, name string, password string) error
	DeleteLogin(ctx context.Context, name string) error
}

var LoginSourceTypes = []string{
	"sql_login",
	"external_login",
}
var ExternalLoginTypes = []string{
	"external_login.user",
	"external_login.group",
}

func resourceLogin() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceLoginCreate,
		ReadContext:   resourceLoginRead,
		UpdateContext: resourceLoginUpdate,
		DeleteContext: resourceLoginDelete,
		// Importer: &schema.ResourceImporter{
		// 	StateContext: resourceLoginImport,
		// },
		Schema: map[string]*schema.Schema{
			"sql_login": {
				Type:         schema.TypeList,
				MaxItems:     1,
				Optional:     true,
				ExactlyOneOf: LoginSourceTypes,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						loginNameProp: {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						passwordProp: {
							Type:      schema.TypeString,
							Required:  true,
							Sensitive: true,
						},
					},
				},
			},
			"external_login": {
				Type:         schema.TypeList,
				MaxItems:     1,
				Optional:     true,
				ExactlyOneOf: LoginSourceTypes,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						loginNameProp: {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"external_login_type": {
							Type:         schema.TypeString,
							Optional:     true,
							ForceNew:     true,
							Default:      "external_login_type.user",
							ExactlyOneOf: ExternalLoginTypes,
						},
					},
				},
			},
			sidStrProp: {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			principalIdProp: {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"user": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"group": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: defaultTimeout,
			Read:    defaultTimeout,
			Create:  defaultTimeout,
			Update:  defaultTimeout,
			Delete:  defaultTimeout,
		},
	}
}

func resourceLoginCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "login", "create")
	logger.Debug().Msgf("Create %s", getLoginID(meta, data))

	// sid := data.Get(sidStrProp).(string)

	logger.Debug().Msgf("timeoutRead %s", data.Timeout(schema.TimeoutRead))
	logger.Debug().Msgf("timeoutCreate %s", data.Timeout(schema.TimeoutCreate))
	logger.Debug().Msgf("timeoutUpdate %s", data.Timeout(schema.TimeoutUpdate))
	logger.Debug().Msgf("timeoutDelete %s", data.Timeout(schema.TimeoutDelete))

	connector, err := getLoginConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if sqlLogin, hasSqlLogin := data.GetOk(LoginSourceTypeSQL); hasSqlLogin {
		sqlLogin := sqlLogin.([]interface{})[0].(map[string]interface{})

		loginName := sqlLogin[loginNameProp].(string)
		password := sqlLogin[passwordProp].(string)

		if err = connector.CreateLogin(ctx, loginName, password, "SQL"); err != nil {
			logger.Debug().Msgf("Error: %s", err)
			return diag.FromErr(errors.Wrapf(err, "unable to create login [%s]", loginName))
		}

		logger.Info().Msgf("created SQL login [%s]", loginName)
	} else if externalLogin, hasExternalLogin := data.GetOk(LoginSourceTypeExternal); hasExternalLogin {
		externalLogin := externalLogin.([]interface{})[0].(map[string]interface{})

		loginName := externalLogin[loginNameProp].(string)

		if err = connector.CreateLogin(ctx, loginName, "", "EXTERNAL"); err != nil {
			logger.Debug().Msgf("Error: %s", err)
			return diag.FromErr(errors.Wrapf(err, "unable to create external login [%s]", loginName))
		}

		logger.Info().Msgf("created external login [%s]", loginName)
	} else {
		return diag.Errorf("either sql_login or external_login must be specified")
	}

	loginID := getLoginID(meta, data)
	data.SetId(loginID)

	return resourceLoginRead(ctx, data, meta)
}

func resourceLoginRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "login", "read")
	logger.Debug().Msgf("Read %s", getLoginID(meta, data))

	var loginName string
	if sqlLogin, hasSqlLogin := data.GetOk(LoginSourceTypeSQL); hasSqlLogin {
		sqlLogin := sqlLogin.([]interface{})[0].(map[string]interface{})
		loginName = sqlLogin[loginNameProp].(string)
	} else if externalLogin, hasExternalLogin := data.GetOk(LoginSourceTypeExternal); hasExternalLogin {
		externalLogin := externalLogin.([]interface{})[0].(map[string]interface{})
		loginName = externalLogin[loginNameProp].(string)
	} else {
		return diag.Errorf("either sql_login or external_login must be specified")
	}

	connector, err := getLoginConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	login, err := connector.GetLogin(ctx, loginName)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read login [%s]", loginName))
	}
	if login == nil {
		logger.Info().Msgf("No login found for [%s]", loginName)
		data.SetId("")
	} else {

		if err = data.Set(principalIdProp, login.PrincipalID); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(sidStrProp, login.SIDStr); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceLoginUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "login", "update")
	logger.Debug().Msgf("Update %s", data.Id())

	connector, err := getLoginConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if sqlLogin, hasSqlLogin := data.GetOk(LoginSourceTypeSQL); hasSqlLogin {
		sqlLogin := sqlLogin.([]interface{})[0].(map[string]interface{})

		loginName := sqlLogin[loginNameProp].(string)
		password := sqlLogin[passwordProp].(string)

		if err = connector.UpdateLogin(ctx, loginName, password); err != nil {
			return diag.FromErr(errors.Wrapf(err, "unable to update login [%s]", loginName))
		}

		logger.Info().Msgf("updated SQL login [%s]", loginName)
	} else if _, hasExternalLogin := data.GetOk(LoginSourceTypeExternal); hasExternalLogin {
		panic("external login update is not supported")
	} else {
		return diag.Errorf("either sql_login or external_login must be specified")
	}

	return resourceLoginRead(ctx, data, meta)
}

func resourceLoginDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "login", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	var loginName string
	if sqlLogin, hasSqlLogin := data.GetOk(LoginSourceTypeSQL); hasSqlLogin {
		sqlLogin := sqlLogin.([]interface{})[0].(map[string]interface{})
		loginName = sqlLogin[loginNameProp].(string)
	} else if externalLogin, hasExternalLogin := data.GetOk(LoginSourceTypeExternal); hasExternalLogin {
		externalLogin := externalLogin.([]interface{})[0].(map[string]interface{})
		loginName = externalLogin[loginNameProp].(string)
	} else {
		return diag.Errorf("either sql_login or external_login must be specified")
	}

	connector, err := getLoginConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = connector.DeleteLogin(ctx, loginName); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete login [%s]", loginName))
	}

	logger.Info().Msgf("deleted login [%s]", loginName)

	// d.SetId("") is automatically called assuming delete returns no errors, but it is added here for explicitness.
	data.SetId("")

	return nil
}

func resourceLoginImport(ctx context.Context, data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	logger := loggerFromMeta(meta, "login", "import")
	logger.Debug().Msgf("Import %s", data.Id())

	id := data.Id()
	_, u, err := serverFromId(id)
	if err != nil {
		return nil, err
	}

	parts := strings.FieldsFunc(u.Path, func(c rune) bool { return c == '/' })
	if len(parts) != 2 {
		return nil, errors.New("invalid ID")
	}
	loginName := parts[1]
	if err = data.Set(loginNameProp, parts[1]); err != nil {
		return nil, err
	}

	data.SetId(getLoginID(meta, data))

	//	loginName := data.Get(loginNameProp).(string)

	connector, err := getLoginConnector(meta, data)
	if err != nil {
		return nil, err
	}

	login, err := connector.GetLogin(ctx, loginName)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read login [%s] for import", loginName)
	}

	if login == nil {
		return nil, errors.Errorf("no login [%s] found for import", loginName)
	}

	if err = data.Set(principalIdProp, login.PrincipalID); err != nil {
		return nil, err
	}
	if err = data.Set(sidStrProp, login.SIDStr); err != nil {
		return nil, err
	}

	return []*schema.ResourceData{data}, nil
}

func getLoginConnector(meta interface{}, data *schema.ResourceData) (LoginConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(LoginConnector), nil
}
