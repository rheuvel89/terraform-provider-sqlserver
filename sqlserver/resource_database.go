package sqlserver

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/pkg/errors"
)

type DatabaseConnector interface {
	CreateDatabase(ctx context.Context, name, collation string) error
	GetDatabase(ctx context.Context, name string) (*model.Database, error)
	UpdateDatabase(ctx context.Context, name string, db *model.Database) error
	DeleteDatabase(ctx context.Context, name string) error
}

var validRecoveryModels = []string{
	"FULL",
	"SIMPLE",
	"BULK_LOGGED",
}

func resourceDatabase() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDatabaseCreate,
		ReadContext:   resourceDatabaseRead,
		UpdateContext: resourceDatabaseUpdate,
		DeleteContext: resourceDatabaseDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceDatabaseImport,
		},
		Schema: map[string]*schema.Schema{
			databaseNameProp: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the database.",
			},
			collationProp: {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    false,
				Computed:    true,
				Description: "The collation of the database. If not specified, the server default collation is used. Changing this value requires the database to be in single-user mode.",
			},
			recoveryModelProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				Description:      "The recovery model of the database. Valid values are FULL, SIMPLE, BULK_LOGGED.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(validRecoveryModels, false)),
			},
			compatibilityLevelProp: {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "The SQL Server compatibility level (e.g. 100, 110, 130, 150, 160).",
			},
			ownerProp: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The owner of the database (server login name).",
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

func resourceDatabaseCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "database", "create")
	name := data.Get(databaseNameProp).(string)
	logger.Debug().Msgf("Create %s", getDatabaseID(meta, data))

	connector, err := getDatabaseConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	collation := data.Get(collationProp).(string)

	if err = connector.CreateDatabase(ctx, name, collation); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create database [%s]", name))
	}

	logger.Info().Msgf("created database [%s]", name)

	// recovery_model and compatibility_level cannot be specified in CREATE DATABASE,
	// so apply them with an ALTER DATABASE right after creation when the user set them.
	db := &model.Database{
		RecoveryModel:      data.Get(recoveryModelProp).(string),
		CompatibilityLevel: data.Get(compatibilityLevelProp).(int),
	}
	if db.RecoveryModel != "" || db.CompatibilityLevel > 0 {
		if err = connector.UpdateDatabase(ctx, name, db); err != nil {
			return diag.FromErr(errors.Wrapf(err, "unable to set options on database [%s]", name))
		}
	}

	data.SetId(getDatabaseID(meta, data))

	return resourceDatabaseRead(ctx, data, meta)
}

func resourceDatabaseRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "database", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	name := data.Get(databaseNameProp).(string)

	connector, err := getDatabaseConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	db, err := connector.GetDatabase(ctx, name)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read database [%s]", name))
	}
	if db == nil {
		logger.Info().Msgf("No database found for [%s]", name)
		data.SetId("")
	} else {
		if err = data.Set(collationProp, db.Collation); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(recoveryModelProp, db.RecoveryModel); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(compatibilityLevelProp, db.CompatibilityLevel); err != nil {
			return diag.FromErr(err)
		}
		if err = data.Set(ownerProp, db.Owner); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceDatabaseUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "database", "update")
	logger.Debug().Msgf("Update %s", data.Id())

	name := data.Get(databaseNameProp).(string)

	connector, err := getDatabaseConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	db := &model.Database{}

	if data.HasChange(collationProp) {
		db.Collation = data.Get(collationProp).(string)
	}
	if data.HasChange(recoveryModelProp) {
		db.RecoveryModel = data.Get(recoveryModelProp).(string)
	}
	if data.HasChange(compatibilityLevelProp) {
		db.CompatibilityLevel = data.Get(compatibilityLevelProp).(int)
	}

	if err = connector.UpdateDatabase(ctx, name, db); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update database [%s]", name))
	}

	logger.Info().Msgf("updated database [%s]", name)

	return resourceDatabaseRead(ctx, data, meta)
}

func resourceDatabaseDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "database", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	name := data.Get(databaseNameProp).(string)

	connector, err := getDatabaseConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = connector.DeleteDatabase(ctx, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete database [%s]", name))
	}

	logger.Info().Msgf("deleted database [%s]", name)

	data.SetId("")

	return nil
}

func resourceDatabaseImport(ctx context.Context, data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	logger := loggerFromMeta(meta, "database", "import")
	logger.Debug().Msgf("Import %s", data.Id())

	id := data.Id()
	_, u, err := serverFromId(id)
	if err != nil {
		return nil, err
	}

	parts := strings.FieldsFunc(u.Path, func(c rune) bool { return c == '/' })
	if len(parts) != 2 {
		return nil, errors.New("invalid ID: expected format sqlserver://host:port/database/DatabaseName")
	}
	databaseName := parts[1]

	connector, err := getDatabaseConnector(meta, data)
	if err != nil {
		return nil, err
	}

	db, err := connector.GetDatabase(ctx, databaseName)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read database [%s] for import", databaseName)
	}

	if db == nil {
		return nil, errors.Errorf("no database [%s] found for import", databaseName)
	}

	if err = data.Set(databaseNameProp, db.Name); err != nil {
		return nil, err
	}
	if err = data.Set(collationProp, db.Collation); err != nil {
		return nil, err
	}
	if err = data.Set(recoveryModelProp, db.RecoveryModel); err != nil {
		return nil, err
	}
	if err = data.Set(compatibilityLevelProp, db.CompatibilityLevel); err != nil {
		return nil, err
	}
	if err = data.Set(ownerProp, db.Owner); err != nil {
		return nil, err
	}

	data.SetId(getDatabaseID(meta, data))

	return []*schema.ResourceData{data}, nil
}

func getDatabaseID(meta interface{}, data *schema.ResourceData) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port
	name := data.Get(databaseNameProp).(string)

	return fmt.Sprintf("sqlserver://%s:%s/database/%s", host, port, name)
}

func getDatabaseConnector(meta interface{}, data *schema.ResourceData) (DatabaseConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(DatabaseConnector), nil
}
