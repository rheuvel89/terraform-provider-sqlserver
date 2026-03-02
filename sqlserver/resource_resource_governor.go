package sqlserver

import (
	"context"
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
)

type ResourceGovernorConnector interface {
	GetResourceGovernor(ctx context.Context) (*model.ResourceGovernor, error)
	EnableResourceGovernor(ctx context.Context, classifierFunction string) error
	DisableResourceGovernor(ctx context.Context) error
	UpdateResourceGovernor(ctx context.Context, rg *model.ResourceGovernor) error
}

func resourceResourceGovernor() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourceGovernorCreate,
		ReadContext:   resourceResourceGovernorRead,
		UpdateContext: resourceResourceGovernorUpdate,
		DeleteContext: resourceResourceGovernorDelete,
		Schema: map[string]*schema.Schema{
			enabledProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Specifies whether the resource governor is enabled. Default is true.",
			},
			classifierFunctionProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "The fully qualified name of the classifier function (schema.function_name). This function classifies sessions into workload groups.",
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

func resourceResourceGovernorCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_governor", "create")
	logger.Debug().Msg("Create/configure resource governor")

	connector, err := getResourceGovernorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	enabled := data.Get(enabledProp).(bool)
	classifierFunction := data.Get(classifierFunctionProp).(string)

	if enabled {
		if err = connector.EnableResourceGovernor(ctx, classifierFunction); err != nil {
			return diag.FromErr(errors.Wrap(err, "unable to enable resource governor"))
		}
	} else {
		if err = connector.DisableResourceGovernor(ctx); err != nil {
			return diag.FromErr(errors.Wrap(err, "unable to disable resource governor"))
		}
	}

	data.SetId(getResourceGovernorID(meta))
	logger.Info().Msg("configured resource governor")

	return resourceResourceGovernorRead(ctx, data, meta)
}

func resourceResourceGovernorRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_governor", "read")
	logger.Debug().Msgf("Read resource governor %s", data.Id())

	connector, err := getResourceGovernorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	rg, err := connector.GetResourceGovernor(ctx)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "unable to read resource governor configuration"))
	}

	data.Set(enabledProp, rg.IsEnabled)
	data.Set(classifierFunctionProp, rg.ClassifierFunction)

	return nil
}

func resourceResourceGovernorUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_governor", "update")
	logger.Debug().Msgf("Update resource governor %s", data.Id())

	connector, err := getResourceGovernorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	rg := &model.ResourceGovernor{
		IsEnabled:          data.Get(enabledProp).(bool),
		ClassifierFunction: data.Get(classifierFunctionProp).(string),
	}

	if err = connector.UpdateResourceGovernor(ctx, rg); err != nil {
		return diag.FromErr(errors.Wrap(err, "unable to update resource governor"))
	}

	logger.Info().Msg("updated resource governor")

	return resourceResourceGovernorRead(ctx, data, meta)
}

func resourceResourceGovernorDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_governor", "delete")
	logger.Debug().Msgf("Delete resource governor configuration %s", data.Id())

	connector, err := getResourceGovernorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	// Disable resource governor on delete
	if err = connector.DisableResourceGovernor(ctx); err != nil {
		return diag.FromErr(errors.Wrap(err, "unable to disable resource governor"))
	}

	data.SetId("")
	logger.Info().Msg("disabled resource governor")

	return nil
}

func getResourceGovernorConnector(meta interface{}, data *schema.ResourceData) (ResourceGovernorConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(ResourceGovernorConnector), nil
}

func getResourceGovernorID(meta interface{}) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port
	return fmt.Sprintf("sqlserver://%s:%s/resource_governor", host, port)
}
