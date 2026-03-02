package sqlserver

import (
	"context"

	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
)

var UserSourceTypes = []string{
	UserSourceTypeInstance,
	UserSourceTypeDatabase,
	UserSourceTypeExternal,
}

func resourceUser() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceUserCreate,
		ReadContext:   resourceUserRead,
		UpdateContext: resourceUserUpdate,
		DeleteContext: resourceUserDelete,
		// Importer: &schema.ResourceImporter{
		// 	StateContext: resourceUserImport,
		// },
		Schema: map[string]*schema.Schema{
			databaseProp: {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "master",
			},
			UserSourceTypeInstance: {
				Type:         schema.TypeList,
				MaxItems:     1,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: UserSourceTypes,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						usernameProp: {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						loginNameProp: {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
			},
			UserSourceTypeDatabase: {
				Type:         schema.TypeList,
				MaxItems:     1,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: UserSourceTypes,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						usernameProp: {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						passwordProp: {
							Type:      schema.TypeString,
							Required:  true,
							ForceNew:  true,
							Sensitive: true,
						},
					},
				},
			},
			UserSourceTypeExternal: {
				Type:         schema.TypeList,
				MaxItems:     1,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: UserSourceTypes,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						usernameProp: {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						objectIdProp: {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
			sidStrProp: {
				Type:     schema.TypeString,
				Computed: true,
			},
			authenticationTypeProp: {
				Type:     schema.TypeString,
				Computed: true,
			},
			principalIdProp: {
				Type:     schema.TypeInt,
				Computed: true,
			},
			rolesProp: {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: defaultTimeout,
			Read:    defaultTimeout,
		},
	}
}

type UserConnector interface {
	CreateUser(ctx context.Context, database string, user *model.User) error
	GetUser(ctx context.Context, database, username string) (*model.User, error)
	UpdateUser(ctx context.Context, database string, user *model.User) error
	DeleteUser(ctx context.Context, database, username string) error
}

// getUsernameFromData extracts the username from the appropriate nested block
func getUsernameFromData(data *schema.ResourceData) (string, string, error) {
	if instanceUser, ok := data.GetOk(UserSourceTypeInstance); ok {
		userData := instanceUser.([]interface{})[0].(map[string]interface{})
		return userData[usernameProp].(string), "INSTANCE", nil
	}
	if databaseUser, ok := data.GetOk(UserSourceTypeDatabase); ok {
		userData := databaseUser.([]interface{})[0].(map[string]interface{})
		return userData[usernameProp].(string), "DATABASE", nil
	}
	if externalUser, ok := data.GetOk(UserSourceTypeExternal); ok {
		userData := externalUser.([]interface{})[0].(map[string]interface{})
		return userData[usernameProp].(string), "EXTERNAL", nil
	}
	return "", "", errors.New("one of instance_user, database_user, or external_user must be specified")
}

func resourceUserCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "user", "create")
	logger.Debug().Msgf("Create %s", getUserID(meta, data))

	database := data.Get(databaseProp).(string)
	roles := data.Get(rolesProp).(*schema.Set).List()

	connector, err := getUserConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	var user *model.User

	if instanceUser, hasInstanceUser := data.GetOk(UserSourceTypeInstance); hasInstanceUser {
		userData := instanceUser.([]interface{})[0].(map[string]interface{})
		username := userData[usernameProp].(string)
		loginName := userData[loginNameProp].(string)

		user = &model.User{
			Username:  username,
			LoginName: loginName,
			AuthType:  "INSTANCE",
			Roles:     toStringSlice(roles),
		}

		logger.Info().Msgf("creating instance user [%s].[%s] for login [%s]", database, username, loginName)
	} else if databaseUser, hasDatabaseUser := data.GetOk(UserSourceTypeDatabase); hasDatabaseUser {
		userData := databaseUser.([]interface{})[0].(map[string]interface{})
		username := userData[usernameProp].(string)
		password := userData[passwordProp].(string)

		user = &model.User{
			Username: username,
			Password: password,
			AuthType: "DATABASE",
			Roles:    toStringSlice(roles),
		}

		logger.Info().Msgf("creating database user [%s].[%s]", database, username)
	} else if externalUser, hasExternalUser := data.GetOk(UserSourceTypeExternal); hasExternalUser {
		userData := externalUser.([]interface{})[0].(map[string]interface{})
		username := userData[usernameProp].(string)
		objectId := ""
		if v, ok := userData[objectIdProp]; ok && v != nil {
			objectId = v.(string)
		}

		user = &model.User{
			Username: username,
			ObjectId: objectId,
			AuthType: "EXTERNAL",
			Roles:    toStringSlice(roles),
		}

		logger.Info().Msgf("creating external user [%s].[%s]", database, username)
	} else {
		return diag.Errorf("one of instance_user, database_user, or external_user must be specified")
	}

	if err = connector.CreateUser(ctx, database, user); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create user [%s].[%s]", database, user.Username))
	}

	data.SetId(getUserID(meta, data))

	logger.Info().Msgf("created user [%s].[%s]", database, user.Username)

	return resourceUserRead(ctx, data, meta)
}

func resourceUserRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "user", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	database := data.Get(databaseProp).(string)
	username, _, err := getUsernameFromData(data)
	if err != nil {
		return diag.FromErr(err)
	}

	connector, err := getUserConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	user, err := connector.GetUser(ctx, database, username)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read user [%s].[%s]", database, username))
	}
	if user == nil {
		logger.Info().Msgf("No user found for [%s].[%s]", database, username)
		data.SetId("")
	} else {
		if err = data.Set(sidStrProp, user.SIDStr); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(authenticationTypeProp, user.AuthType); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(principalIdProp, user.PrincipalID); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(rolesProp, user.Roles); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceUserUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "user", "update")
	logger.Debug().Msgf("Update %s", data.Id())

	database := data.Get(databaseProp).(string)
	username, _, err := getUsernameFromData(data)
	if err != nil {
		return diag.FromErr(err)
	}
	roles := data.Get(rolesProp).(*schema.Set).List()

	connector, err := getUserConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	user := &model.User{
		Username: username,
		Roles:    toStringSlice(roles),
	}
	if err = connector.UpdateUser(ctx, database, user); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update user [%s].[%s]", database, username))
	}

	data.SetId(getUserID(meta, data))

	logger.Info().Msgf("updated user [%s].[%s]", database, username)

	return resourceUserRead(ctx, data, meta)
}

func resourceUserDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "user", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	database := data.Get(databaseProp).(string)
	username, _, err := getUsernameFromData(data)
	if err != nil {
		return diag.FromErr(err)
	}

	connector, err := getUserConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = connector.DeleteUser(ctx, database, username); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete user [%s].[%s]", database, username))
	}

	logger.Info().Msgf("deleted user [%s].[%s]", database, username)

	// d.SetId("") is automatically called assuming delete returns no errors, but it is added here for explicitness.
	data.SetId("")

	return nil
}

// func resourceUserImport(ctx context.Context, data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
// 	logger := loggerFromMeta(meta, "user", "import")
// 	logger.Debug().Msgf("Import %s", data.Id())

// 	server, u, err := serverFromId(data.Id())
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err = data.Set(serverProp, server); err != nil {
// 		return nil, err
// 	}

// 	parts := strings.Split(u.Path, "/")
// 	if len(parts) != 3 {
// 		return nil, errors.New("invalid ID")
// 	}
// 	if err = data.Set(databaseProp, parts[1]); err != nil {
// 		return nil, err
// 	}
// 	if err = data.Set(usernameProp, parts[2]); err != nil {
// 		return nil, err
// 	}

// 	data.SetId(getUserID(meta, data))

// 	database := data.Get(databaseProp).(string)
// 	username := data.Get(usernameProp).(string)

// 	connector, err := getUserConnector(meta, data)
// 	if err != nil {
// 		return nil, err
// 	}

// 	login, err := connector.GetUser(ctx, database, username)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "unable to read user [%s].[%s] for import", database, username)
// 	}

// 	if login == nil {
// 		return nil, errors.Errorf("no user [%s].[%s] found for import", database, username)
// 	}

// 	if err = data.Set(authenticationTypeProp, login.AuthType); err != nil {
// 		return nil, err
// 	}
// 	if err = data.Set(principalIdProp, login.PrincipalID); err != nil {
// 		return nil, err
// 	}
// 	if err = data.Set(rolesProp, login.Roles); err != nil {
// 		return nil, err
// 	}

// 	return []*schema.ResourceData{data}, nil
// }

func getUserConnector(meta interface{}, data *schema.ResourceData) (UserConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(UserConnector), nil
}

func toStringSlice(values []interface{}) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = v.(string)
	}
	return result
}
