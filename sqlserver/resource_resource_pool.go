package sqlserver

import (
	"context"
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/pkg/errors"
)

type ResourcePoolConnector interface {
	CreateResourcePool(ctx context.Context, pool *model.ResourcePool) error
	GetResourcePool(ctx context.Context, name string) (*model.ResourcePool, error)
	UpdateResourcePool(ctx context.Context, pool *model.ResourcePool) error
	DeleteResourcePool(ctx context.Context, name string) error
}

func resourceResourcePool() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourcePoolCreate,
		ReadContext:   resourceResourcePoolRead,
		UpdateContext: resourceResourcePoolUpdate,
		DeleteContext: resourceResourcePoolDelete,
		Schema: map[string]*schema.Schema{
			resourcePoolNameProp: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the resource pool.",
			},
			minCPUPercentProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntBetween(0, 100),
				Description:  "Specifies the guaranteed average CPU bandwidth for all requests in the resource pool when there is CPU contention. Range is 0 to 100.",
			},
			maxCPUPercentProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      100,
				ValidateFunc: validation.IntBetween(1, 100),
				Description:  "Specifies the maximum average CPU bandwidth that all requests in resource pool will receive when there is CPU contention. Range is 1 to 100.",
			},
			minMemoryPercentProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntBetween(0, 100),
				Description:  "Specifies the minimum amount of memory reserved for this resource pool that can not be shared with other resource pools. Range is 0 to 100.",
			},
			maxMemoryPercentProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      100,
				ValidateFunc: validation.IntBetween(1, 100),
				Description:  "Specifies the total server memory that can be used by requests in this resource pool. Range is 1 to 100.",
			},
			capCPUPercentProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      100,
				ValidateFunc: validation.IntBetween(1, 100),
				Description:  "Specifies a hard cap on the CPU bandwidth that all requests in the resource pool will receive. Limits the maximum CPU bandwidth level to be the same as the specified value. Range is 1 to 100.",
			},
			minIOPSPerVolumeProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Specifies the minimum I/O operations per second (IOPS) per disk volume to reserve for the resource pool.",
			},
			maxIOPSPerVolumeProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Specifies the maximum I/O operations per second (IOPS) per disk volume to allow for the resource pool. 0 means unlimited.",
			},
			poolIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the resource pool.",
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

func resourceResourcePoolCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_pool", "create")
	logger.Debug().Msgf("Create resource pool %s", data.Get(resourcePoolNameProp).(string))

	connector, err := getResourcePoolConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	pool := &model.ResourcePool{
		Name:             data.Get(resourcePoolNameProp).(string),
		MinCPUPercent:    data.Get(minCPUPercentProp).(int),
		MaxCPUPercent:    data.Get(maxCPUPercentProp).(int),
		MinMemoryPercent: data.Get(minMemoryPercentProp).(int),
		MaxMemoryPercent: data.Get(maxMemoryPercentProp).(int),
		CapCPUPercent:    data.Get(capCPUPercentProp).(int),
		MinIOPSPerVolume: data.Get(minIOPSPerVolumeProp).(int),
		MaxIOPSPerVolume: data.Get(maxIOPSPerVolumeProp).(int),
	}

	if err = connector.CreateResourcePool(ctx, pool); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create resource pool [%s]", pool.Name))
	}

	data.SetId(getResourcePoolID(meta, data))
	logger.Info().Msgf("created resource pool [%s]", pool.Name)

	return resourceResourcePoolRead(ctx, data, meta)
}

func resourceResourcePoolRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_pool", "read")
	logger.Debug().Msgf("Read resource pool %s", data.Id())

	name := data.Get(resourcePoolNameProp).(string)

	connector, err := getResourcePoolConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	pool, err := connector.GetResourcePool(ctx, name)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read resource pool [%s]", name))
	}
	if pool == nil {
		logger.Info().Msgf("No resource pool found for [%s]", name)
		data.SetId("")
		return nil
	}

	data.Set(resourcePoolNameProp, pool.Name)
	data.Set(minCPUPercentProp, pool.MinCPUPercent)
	data.Set(maxCPUPercentProp, pool.MaxCPUPercent)
	data.Set(minMemoryPercentProp, pool.MinMemoryPercent)
	data.Set(maxMemoryPercentProp, pool.MaxMemoryPercent)
	data.Set(capCPUPercentProp, pool.CapCPUPercent)
	data.Set(minIOPSPerVolumeProp, pool.MinIOPSPerVolume)
	data.Set(maxIOPSPerVolumeProp, pool.MaxIOPSPerVolume)
	data.Set(poolIdProp, pool.PoolID)

	return nil
}

func resourceResourcePoolUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_pool", "update")
	logger.Debug().Msgf("Update resource pool %s", data.Id())

	connector, err := getResourcePoolConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	pool := &model.ResourcePool{
		Name:             data.Get(resourcePoolNameProp).(string),
		MinCPUPercent:    data.Get(minCPUPercentProp).(int),
		MaxCPUPercent:    data.Get(maxCPUPercentProp).(int),
		MinMemoryPercent: data.Get(minMemoryPercentProp).(int),
		MaxMemoryPercent: data.Get(maxMemoryPercentProp).(int),
		CapCPUPercent:    data.Get(capCPUPercentProp).(int),
		MinIOPSPerVolume: data.Get(minIOPSPerVolumeProp).(int),
		MaxIOPSPerVolume: data.Get(maxIOPSPerVolumeProp).(int),
	}

	if err = connector.UpdateResourcePool(ctx, pool); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update resource pool [%s]", pool.Name))
	}

	logger.Info().Msgf("updated resource pool [%s]", pool.Name)

	return resourceResourcePoolRead(ctx, data, meta)
}

func resourceResourcePoolDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "resource_pool", "delete")
	logger.Debug().Msgf("Delete resource pool %s", data.Id())

	name := data.Get(resourcePoolNameProp).(string)

	connector, err := getResourcePoolConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = connector.DeleteResourcePool(ctx, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete resource pool [%s]", name))
	}

	data.SetId("")
	logger.Info().Msgf("deleted resource pool [%s]", name)

	return nil
}

func getResourcePoolConnector(meta interface{}, data *schema.ResourceData) (ResourcePoolConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(ResourcePoolConnector), nil
}

func getResourcePoolID(meta interface{}, data *schema.ResourceData) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port
	name := data.Get(resourcePoolNameProp).(string)
	return fmt.Sprintf("sqlserver://%s:%s/resource_pool/%s", host, port, name)
}
