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

type AgentOperatorConnector interface {
	EnsureAgentAvailable(ctx context.Context) error
	GetAgentOperator(ctx context.Context, operatorID int) (*model.AgentOperator, error)
	CreateAgentOperator(ctx context.Context, op *model.AgentOperator) (int, error)
	UpdateAgentOperator(ctx context.Context, name string, op *model.AgentOperator) error
	DeleteAgentOperator(ctx context.Context, name string) error
}

func resourceAgentOperator() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAgentOperatorCreate,
		ReadContext:   resourceAgentOperatorRead,
		UpdateContext: resourceAgentOperatorUpdate,
		DeleteContext: resourceAgentOperatorDelete,
		Schema: map[string]*schema.Schema{
			agentNameProp: {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "The name of the operator. Changing it renames the operator in place.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			agentEnabledProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the operator receives notifications.",
			},
			agentEmailAddressProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The email address that notifications are sent to. Mail is delivered by Database Mail, which must be set up separately.",
			},
			agentCategoryNameProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          agentOperatorDefaultCategory,
				Description:      "The name of an existing operator category.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			agentOperatorIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the operator in msdb.",
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

func agentOperatorFromData(data *schema.ResourceData) *model.AgentOperator {
	return &model.AgentOperator{
		Name:         data.Get(agentNameProp).(string),
		Enabled:      data.Get(agentEnabledProp).(bool),
		EmailAddress: data.Get(agentEmailAddressProp).(string),
		CategoryName: data.Get(agentCategoryNameProp).(string),
	}
}

func resourceAgentOperatorCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_operator", "create")
	op := agentOperatorFromData(data)
	logger.Debug().Msgf("Create agent operator [%s]", op.Name)

	connector, err := getAgentOperatorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = connector.EnsureAgentAvailable(ctx); err != nil {
		return diag.FromErr(err)
	}

	id, err := connector.CreateAgentOperator(ctx, op)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create agent operator [%s]", op.Name))
	}

	logger.Info().Msgf("created agent operator [%s]", op.Name)

	if err = data.Set(agentOperatorIdProp, id); err != nil {
		return diag.FromErr(err)
	}
	data.SetId(getAgentOperatorID(meta, id))

	return resourceAgentOperatorRead(ctx, data, meta)
}

func resourceAgentOperatorRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_operator", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	connector, err := getAgentOperatorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	id := data.Get(agentOperatorIdProp).(int)
	op, err := connector.GetAgentOperator(ctx, id)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read agent operator [%d]", id))
	}
	if op == nil {
		logger.Info().Msgf("No agent operator found for [%d]", id)
		data.SetId("")
		return nil
	}

	if err = data.Set(agentNameProp, op.Name); err != nil {
		return diag.FromErr(err)
	}
	if err = data.Set(agentEnabledProp, op.Enabled); err != nil {
		return diag.FromErr(err)
	}
	if err = data.Set(agentEmailAddressProp, op.EmailAddress); err != nil {
		return diag.FromErr(err)
	}
	if err = data.Set(agentCategoryNameProp, op.CategoryName); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceAgentOperatorUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_operator", "update")
	logger.Debug().Msgf("Update %s", data.Id())

	connector, err := getAgentOperatorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	oldName, _ := data.GetChange(agentNameProp)
	op := agentOperatorFromData(data)
	if err = connector.UpdateAgentOperator(ctx, oldName.(string), op); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update agent operator [%s]", oldName))
	}

	logger.Info().Msgf("updated agent operator [%s]", op.Name)

	return resourceAgentOperatorRead(ctx, data, meta)
}

func resourceAgentOperatorDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_operator", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	connector, err := getAgentOperatorConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	name := data.Get(agentNameProp).(string)
	if err = connector.DeleteAgentOperator(ctx, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete agent operator [%s]", name))
	}

	logger.Info().Msgf("deleted agent operator [%s]", name)

	data.SetId("")

	return nil
}

func getAgentOperatorID(meta interface{}, operatorID int) string {
	provider := meta.(sqlserverProvider)
	return fmt.Sprintf("sqlserver://%s:%s/agent_operator/%d", provider.host, provider.port, operatorID)
}

func getAgentOperatorConnector(meta interface{}, data *schema.ResourceData) (AgentOperatorConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(AgentOperatorConnector), nil
}
