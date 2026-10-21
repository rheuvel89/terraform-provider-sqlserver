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

type AgentJobConnector interface {
	EnsureAgentAvailable(ctx context.Context) error
	IsManagedInstance(ctx context.Context) (bool, error)
	GetAgentJob(ctx context.Context, jobID string) (*model.AgentJob, error)
	CreateAgentJob(ctx context.Context, job *model.AgentJob) (string, error)
	UpdateAgentJob(ctx context.Context, job *model.AgentJob) error
	DeleteAgentJob(ctx context.Context, jobID string) error
}

func resourceAgentJob() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAgentJobCreate,
		ReadContext:   resourceAgentJobRead,
		UpdateContext: resourceAgentJobUpdate,
		DeleteContext: resourceAgentJobDelete,
		CustomizeDiff: resourceAgentJobCustomizeDiff,
		Schema: map[string]*schema.Schema{
			agentNameProp: {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "The name of the job. Changing it renames the job in place.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			agentDescriptionProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "A description of the job.",
			},
			agentEnabledProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the job runs on its schedules.",
			},
			agentCategoryNameProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          agentJobDefaultCategory,
				Description:      "The name of an existing job category.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			agentOwnerLoginProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The login that owns the job. Defaults to the login the provider connects with.",
			},
			agentNotifyEventlog: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "on_failure",
				Description:      "When to write the job outcome to the Windows application log: never, on_success, on_failure or always.",
				ValidateDiagFunc: enumValidation(agentNotifyLevels),
			},
			agentNotifyEmail: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "never",
				Description:      "When to email notify_email_operator_name: never, on_success, on_failure or always.",
				ValidateDiagFunc: enumValidation(agentNotifyLevels),
			},
			agentNotifyEmailOpProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The name of the operator to email. Required when notify_level_email is not never.",
			},
			agentDeleteLevelProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "never",
				Description:      "When to delete the job after it runs: never, on_success, on_failure or always.",
				ValidateDiagFunc: enumValidation(agentNotifyLevels),
			},
			agentJobIdProp: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The job's GUID.",
			},
			agentStartStepIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The step the job starts at. Read-only.",
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

func resourceAgentJobCustomizeDiff(_ context.Context, diff *schema.ResourceDiff, _ interface{}) error {
	if diff.Get(agentNotifyEmail).(string) != "never" && diff.Get(agentNotifyEmailOpProp).(string) == "" && diff.NewValueKnown(agentNotifyEmailOpProp) {
		return errors.Errorf("%s is required when %s is not \"never\"", agentNotifyEmailOpProp, agentNotifyEmail)
	}
	return nil
}

func agentJobFromData(data *schema.ResourceData) *model.AgentJob {
	return &model.AgentJob{
		JobID:                   data.Get(agentJobIdProp).(string),
		Name:                    data.Get(agentNameProp).(string),
		Description:             data.Get(agentDescriptionProp).(string),
		Enabled:                 data.Get(agentEnabledProp).(bool),
		CategoryName:            data.Get(agentCategoryNameProp).(string),
		OwnerLoginName:          data.Get(agentOwnerLoginProp).(string),
		NotifyLevelEventlog:     agentNotifyLevels[data.Get(agentNotifyEventlog).(string)],
		NotifyLevelEmail:        agentNotifyLevels[data.Get(agentNotifyEmail).(string)],
		NotifyEmailOperatorName: data.Get(agentNotifyEmailOpProp).(string),
		DeleteLevel:             agentNotifyLevels[data.Get(agentDeleteLevelProp).(string)],
	}
}

func resourceAgentJobCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job", "create")
	job := agentJobFromData(data)
	logger.Debug().Msgf("Create agent job [%s]", job.Name)

	connector, err := getAgentJobConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = connector.EnsureAgentAvailable(ctx); err != nil {
		return diag.FromErr(err)
	}

	jobID, err := connector.CreateAgentJob(ctx, job)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create agent job [%s]", job.Name))
	}

	logger.Info().Msgf("created agent job [%s] (%s)", job.Name, jobID)

	if err = data.Set(agentJobIdProp, jobID); err != nil {
		return diag.FromErr(err)
	}
	data.SetId(getAgentJobID(meta, jobID))

	return resourceAgentJobRead(ctx, data, meta)
}

func resourceAgentJobRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	connector, err := getAgentJobConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID := data.Get(agentJobIdProp).(string)
	job, err := connector.GetAgentJob(ctx, jobID)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read agent job [%s]", jobID))
	}
	if job == nil {
		logger.Info().Msgf("No agent job found for [%s]", jobID)
		data.SetId("")
		return nil
	}

	eventlog := enumName(agentNotifyLevels, job.NotifyLevelEventlog)
	if job.NotifyLevelEventlog == 0 && data.Get(agentNotifyEventlog).(string) != eventlog {
		// Managed Instance has no Windows application log and stores 0 whatever level is set.
		// Keep the configured level there, or every plan would try to set it again.
		mi, err := connector.IsManagedInstance(ctx)
		if err != nil {
			return diag.FromErr(err)
		}
		if mi {
			eventlog = data.Get(agentNotifyEventlog).(string)
		}
	}

	values := map[string]interface{}{
		agentNameProp:          job.Name,
		agentDescriptionProp:   job.Description,
		agentEnabledProp:       job.Enabled,
		agentCategoryNameProp:  job.CategoryName,
		agentOwnerLoginProp:    job.OwnerLoginName,
		agentNotifyEventlog:    eventlog,
		agentNotifyEmail:       enumName(agentNotifyLevels, job.NotifyLevelEmail),
		agentNotifyEmailOpProp: job.NotifyEmailOperatorName,
		agentDeleteLevelProp:   enumName(agentNotifyLevels, job.DeleteLevel),
		agentStartStepIdProp:   job.StartStepID,
	}
	for k, v := range values {
		if err = data.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceAgentJobUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job", "update")
	logger.Debug().Msgf("Update %s", data.Id())

	connector, err := getAgentJobConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	job := agentJobFromData(data)
	if !data.HasChange(agentOwnerLoginProp) {
		// Leave the owner alone unless it changes: changing it needs sysadmin.
		job.OwnerLoginName = ""
	}
	if err = connector.UpdateAgentJob(ctx, job); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update agent job [%s]", job.Name))
	}

	logger.Info().Msgf("updated agent job [%s]", job.Name)

	return resourceAgentJobRead(ctx, data, meta)
}

func resourceAgentJobDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	connector, err := getAgentJobConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID := data.Get(agentJobIdProp).(string)
	if err = connector.DeleteAgentJob(ctx, jobID); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete agent job [%s]", data.Get(agentNameProp)))
	}

	logger.Info().Msgf("deleted agent job [%s]", jobID)

	data.SetId("")

	return nil
}

func getAgentJobID(meta interface{}, jobID string) string {
	provider := meta.(sqlserverProvider)
	return fmt.Sprintf("sqlserver://%s:%s/agent_job/%s", provider.host, provider.port, jobID)
}

func getAgentJobConnector(meta interface{}, data *schema.ResourceData) (AgentJobConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(AgentJobConnector), nil
}
