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

type AgentJobScheduleConnector interface {
	EnsureAgentAvailable(ctx context.Context) error
	GetAgentJobSchedule(ctx context.Context, jobID string, scheduleID int) (bool, error)
	CreateAgentJobSchedule(ctx context.Context, jobID string, scheduleID int) error
	DeleteAgentJobSchedule(ctx context.Context, jobID string, scheduleID int) error
}

func resourceAgentJobSchedule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAgentJobScheduleCreate,
		ReadContext:   resourceAgentJobScheduleRead,
		DeleteContext: resourceAgentJobScheduleDelete,
		Schema: map[string]*schema.Schema{
			agentJobIdProp: {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				Description:      "The job_id of the job to attach the schedule to.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.IsUUID),
			},
			agentScheduleIdProp: {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The schedule_id of the schedule to attach.",
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: defaultTimeout,
			Read:    defaultTimeout,
			Create:  defaultTimeout,
			Delete:  defaultTimeout,
		},
	}
}

func resourceAgentJobScheduleCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_schedule", "create")
	jobID := data.Get(agentJobIdProp).(string)
	scheduleID := data.Get(agentScheduleIdProp).(int)
	logger.Debug().Msgf("Attach agent schedule [%d] to job [%s]", scheduleID, jobID)

	connector, err := getAgentJobScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = connector.EnsureAgentAvailable(ctx); err != nil {
		return diag.FromErr(err)
	}

	if err = connector.CreateAgentJobSchedule(ctx, jobID, scheduleID); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to attach agent schedule [%d] to job [%s]", scheduleID, jobID))
	}

	logger.Info().Msgf("attached agent schedule [%d] to job [%s]", scheduleID, jobID)

	data.SetId(getAgentJobScheduleID(meta, jobID, scheduleID))

	return resourceAgentJobScheduleRead(ctx, data, meta)
}

func resourceAgentJobScheduleRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_schedule", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	connector, err := getAgentJobScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID := data.Get(agentJobIdProp).(string)
	scheduleID := data.Get(agentScheduleIdProp).(int)
	attached, err := connector.GetAgentJobSchedule(ctx, jobID, scheduleID)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read attachment of agent schedule [%d] to job [%s]", scheduleID, jobID))
	}
	if !attached {
		logger.Info().Msgf("Agent schedule [%d] is not attached to job [%s]", scheduleID, jobID)
		data.SetId("")
	}

	return nil
}

func resourceAgentJobScheduleDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_schedule", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	connector, err := getAgentJobScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID := data.Get(agentJobIdProp).(string)
	scheduleID := data.Get(agentScheduleIdProp).(int)
	if err = connector.DeleteAgentJobSchedule(ctx, jobID, scheduleID); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to detach agent schedule [%d] from job [%s]", scheduleID, jobID))
	}

	logger.Info().Msgf("detached agent schedule [%d] from job [%s]", scheduleID, jobID)

	data.SetId("")

	return nil
}

func getAgentJobScheduleID(meta interface{}, jobID string, scheduleID int) string {
	provider := meta.(sqlserverProvider)
	return fmt.Sprintf("sqlserver://%s:%s/agent_job/%s/agent_schedule/%d", provider.host, provider.port, jobID, scheduleID)
}

func getAgentJobScheduleConnector(meta interface{}, data *schema.ResourceData) (AgentJobScheduleConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(AgentJobScheduleConnector), nil
}
