package sqlserver

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
)

type ClassifierFunctionConnector interface {
	CreateClassifierFunction(ctx context.Context, fn *model.ClassifierFunction) error
	GetClassifierFunction(ctx context.Context, schemaName, name string) (*model.ClassifierFunction, error)
	UpdateClassifierFunction(ctx context.Context, fn *model.ClassifierFunction) error
	DeleteClassifierFunction(ctx context.Context, schemaName, name string) error
}

func resourceClassifierFunction() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClassifierFunctionCreate,
		ReadContext:   resourceClassifierFunctionRead,
		UpdateContext: resourceClassifierFunctionUpdate,
		DeleteContext: resourceClassifierFunctionDelete,
		Schema: map[string]*schema.Schema{
			classifierFunctionNameProp: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the classifier function.",
			},
			schemaNameProp: {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Default:     "dbo",
				Description: "The schema name for the classifier function. Defaults to 'dbo'.",
			},
			functionBodyProp: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The body of the classifier function. This should contain the logic that returns the workload group name (SYSNAME). Do not include CREATE FUNCTION or BEGIN/END - just the function body logic.",
			},
			objectIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The object ID of the classifier function.",
			},
			fullyQualifiedNameProp: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The fully qualified name of the function (schema.name) for use with sqlserver_resource_governor.",
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

func resourceClassifierFunctionCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "classifier_function", "create")
	logger.Debug().Msgf("Create classifier function %s", data.Get(classifierFunctionNameProp).(string))

	connector, err := getClassifierFunctionConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	fn := &model.ClassifierFunction{
		SchemaName: data.Get(schemaNameProp).(string),
		Name:       data.Get(classifierFunctionNameProp).(string),
		Definition: data.Get(functionBodyProp).(string),
	}

	if err = connector.CreateClassifierFunction(ctx, fn); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create classifier function [%s].[%s]", fn.SchemaName, fn.Name))
	}

	data.SetId(getClassifierFunctionID(meta, data))
	logger.Info().Msgf("created classifier function [%s].[%s]", fn.SchemaName, fn.Name)

	return resourceClassifierFunctionRead(ctx, data, meta)
}

func resourceClassifierFunctionRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "classifier_function", "read")
	logger.Debug().Msgf("Read classifier function %s", data.Id())

	schemaName := data.Get(schemaNameProp).(string)
	name := data.Get(classifierFunctionNameProp).(string)

	connector, err := getClassifierFunctionConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	fn, err := connector.GetClassifierFunction(ctx, schemaName, name)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read classifier function [%s].[%s]", schemaName, name))
	}
	if fn == nil {
		logger.Info().Msgf("No classifier function found for [%s].[%s]", schemaName, name)
		data.SetId("")
		return nil
	}

	data.Set(classifierFunctionNameProp, fn.Name)
	data.Set(schemaNameProp, fn.SchemaName)
	// We don't update function_body from read since SQL Server may reformat it
	data.Set(objectIdProp, fn.ObjectID)
	data.Set(fullyQualifiedNameProp, fmt.Sprintf("%s.%s", fn.SchemaName, fn.Name))

	return nil
}

func resourceClassifierFunctionUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "classifier_function", "update")
	logger.Debug().Msgf("Update classifier function %s", data.Id())

	connector, err := getClassifierFunctionConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	fn := &model.ClassifierFunction{
		SchemaName: data.Get(schemaNameProp).(string),
		Name:       data.Get(classifierFunctionNameProp).(string),
		Definition: data.Get(functionBodyProp).(string),
	}

	if err = connector.UpdateClassifierFunction(ctx, fn); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update classifier function [%s].[%s]", fn.SchemaName, fn.Name))
	}

	logger.Info().Msgf("updated classifier function [%s].[%s]", fn.SchemaName, fn.Name)

	return resourceClassifierFunctionRead(ctx, data, meta)
}

func resourceClassifierFunctionDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "classifier_function", "delete")
	logger.Debug().Msgf("Delete classifier function %s", data.Id())

	schemaName := data.Get(schemaNameProp).(string)
	name := data.Get(classifierFunctionNameProp).(string)

	connector, err := getClassifierFunctionConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = connector.DeleteClassifierFunction(ctx, schemaName, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete classifier function [%s].[%s]", schemaName, name))
	}

	data.SetId("")
	logger.Info().Msgf("deleted classifier function [%s].[%s]", schemaName, name)

	return nil
}

func getClassifierFunctionConnector(meta interface{}, data *schema.ResourceData) (ClassifierFunctionConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(ClassifierFunctionConnector), nil
}

func getClassifierFunctionID(meta interface{}, data *schema.ResourceData) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port
	schemaName := data.Get(schemaNameProp).(string)
	name := data.Get(classifierFunctionNameProp).(string)
	return fmt.Sprintf("sqlserver://%s:%s/classifier_function/%s.%s", host, port, schemaName, name)
}

// Helper function to parse schema.name format
func parseSchemaAndName(qualifiedName string) (schema, name string) {
	parts := strings.SplitN(qualifiedName, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "dbo", qualifiedName
}
