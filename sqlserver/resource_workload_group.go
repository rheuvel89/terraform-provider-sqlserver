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

type WorkloadGroupConnector interface {
	CreateWorkloadGroup(ctx context.Context, group *model.WorkloadGroup) error
	GetWorkloadGroup(ctx context.Context, name string) (*model.WorkloadGroup, error)
	UpdateWorkloadGroup(ctx context.Context, group *model.WorkloadGroup) error
	DeleteWorkloadGroup(ctx context.Context, name string) error
}

var validImportanceValues = []string{"LOW", "MEDIUM", "HIGH"}

func resourceWorkloadGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceWorkloadGroupCreate,
		ReadContext:   resourceWorkloadGroupRead,
		UpdateContext: resourceWorkloadGroupUpdate,
		DeleteContext: resourceWorkloadGroupDelete,
		Schema: map[string]*schema.Schema{
			workloadGroupNameProp: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the workload group.",
			},
			resourcePoolNameRefProp: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the resource pool to associate with the workload group.",
			},
			importanceProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "MEDIUM",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(validImportanceValues, false)),
				Description:      "Specifies the relative importance of a request in the workload group. Valid values are LOW, MEDIUM, and HIGH. Default is MEDIUM.",
			},
			requestMaxMemoryGrantPercentProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      25,
				ValidateFunc: validation.IntBetween(1, 100),
				Description:  "Specifies the maximum amount of memory that a single request can take from the pool. Range is 1 to 100. Default is 25.",
			},
			requestMaxCPUTimeSecProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Specifies the maximum amount of CPU time, in seconds, that a request can use. 0 = unlimited. Default is 0.",
			},
			requestMemoryGrantTimeoutSecProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Specifies the maximum time, in seconds, that a query can wait for a memory grant (work buffer memory) to become available. 0 = unlimited. Default is 0.",
			},
			maxDOPProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Specifies the maximum degree of parallelism (MAXDOP) for parallel query execution. 0 = use global setting. Default is 0.",
			},
			groupMaxRequestsProp: {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Specifies the maximum number of simultaneous requests that are allowed to execute in the workload group. 0 = unlimited. Default is 0.",
			},
			groupIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the workload group.",
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

func resourceWorkloadGroupCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "workload_group", "create")
	logger.Debug().Msgf("Create workload group %s", data.Get(workloadGroupNameProp).(string))

	connector, err := getWorkloadGroupConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	group := &model.WorkloadGroup{
		Name:                         data.Get(workloadGroupNameProp).(string),
		PoolName:                     data.Get(resourcePoolNameRefProp).(string),
		Importance:                   data.Get(importanceProp).(string),
		RequestMaxMemoryGrantPercent: data.Get(requestMaxMemoryGrantPercentProp).(int),
		RequestMaxCPUTimeSec:         data.Get(requestMaxCPUTimeSecProp).(int),
		RequestMemoryGrantTimeoutSec: data.Get(requestMemoryGrantTimeoutSecProp).(int),
		MaxDOP:                       data.Get(maxDOPProp).(int),
		GroupMaxRequests:             data.Get(groupMaxRequestsProp).(int),
	}

	if err = connector.CreateWorkloadGroup(ctx, group); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create workload group [%s]", group.Name))
	}

	data.SetId(getWorkloadGroupID(meta, data))
	logger.Info().Msgf("created workload group [%s]", group.Name)

	return resourceWorkloadGroupRead(ctx, data, meta)
}

func resourceWorkloadGroupRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "workload_group", "read")
	logger.Debug().Msgf("Read workload group %s", data.Id())

	name := data.Get(workloadGroupNameProp).(string)

	connector, err := getWorkloadGroupConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	group, err := connector.GetWorkloadGroup(ctx, name)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read workload group [%s]", name))
	}
	if group == nil {
		logger.Info().Msgf("No workload group found for [%s]", name)
		data.SetId("")
		return nil
	}

	data.Set(workloadGroupNameProp, group.Name)
	data.Set(resourcePoolNameRefProp, group.PoolName)
	data.Set(importanceProp, strings.ToUpper(group.Importance))
	data.Set(requestMaxMemoryGrantPercentProp, group.RequestMaxMemoryGrantPercent)
	data.Set(requestMaxCPUTimeSecProp, group.RequestMaxCPUTimeSec)
	data.Set(requestMemoryGrantTimeoutSecProp, group.RequestMemoryGrantTimeoutSec)
	data.Set(maxDOPProp, group.MaxDOP)
	data.Set(groupMaxRequestsProp, group.GroupMaxRequests)
	data.Set(groupIdProp, group.GroupID)

	return nil
}

func resourceWorkloadGroupUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "workload_group", "update")
	logger.Debug().Msgf("Update workload group %s", data.Id())

	connector, err := getWorkloadGroupConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	group := &model.WorkloadGroup{
		Name:                         data.Get(workloadGroupNameProp).(string),
		PoolName:                     data.Get(resourcePoolNameRefProp).(string),
		Importance:                   data.Get(importanceProp).(string),
		RequestMaxMemoryGrantPercent: data.Get(requestMaxMemoryGrantPercentProp).(int),
		RequestMaxCPUTimeSec:         data.Get(requestMaxCPUTimeSecProp).(int),
		RequestMemoryGrantTimeoutSec: data.Get(requestMemoryGrantTimeoutSecProp).(int),
		MaxDOP:                       data.Get(maxDOPProp).(int),
		GroupMaxRequests:             data.Get(groupMaxRequestsProp).(int),
	}

	if err = connector.UpdateWorkloadGroup(ctx, group); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update workload group [%s]", group.Name))
	}

	logger.Info().Msgf("updated workload group [%s]", group.Name)

	return resourceWorkloadGroupRead(ctx, data, meta)
}

func resourceWorkloadGroupDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "workload_group", "delete")
	logger.Debug().Msgf("Delete workload group %s", data.Id())

	name := data.Get(workloadGroupNameProp).(string)

	connector, err := getWorkloadGroupConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = connector.DeleteWorkloadGroup(ctx, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete workload group [%s]", name))
	}

	data.SetId("")
	logger.Info().Msgf("deleted workload group [%s]", name)

	return nil
}

func getWorkloadGroupConnector(meta interface{}, data *schema.ResourceData) (WorkloadGroupConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(WorkloadGroupConnector), nil
}

func getWorkloadGroupID(meta interface{}, data *schema.ResourceData) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port
	name := data.Get(workloadGroupNameProp).(string)
	return fmt.Sprintf("sqlserver://%s:%s/workload_group/%s", host, port, name)
}
