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

type AgentScheduleConnector interface {
	EnsureAgentAvailable(ctx context.Context) error
	GetAgentSchedule(ctx context.Context, scheduleID int) (*model.AgentSchedule, error)
	CreateAgentSchedule(ctx context.Context, s *model.AgentSchedule) (int, error)
	UpdateAgentSchedule(ctx context.Context, s *model.AgentSchedule) error
	DeleteAgentSchedule(ctx context.Context, scheduleID int) error
}

// agentScheduleSpecProps are the attributes that make up an agentScheduleSpec.
var agentScheduleSpecProps = []string{
	agentFrequencyTypeProp, agentRecurrenceFactorProp, agentWeeklyDaysProp, agentMonthlyDayProp,
	agentMonthlyRelativeWeekProp, agentMonthlyRelativeDayProp, agentSubdayTypeProp, agentSubdayIntervalProp,
	agentActiveStartDateProp, agentActiveEndDateProp, agentActiveStartTimeProp, agentActiveEndTimeProp,
}

func resourceAgentSchedule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAgentScheduleCreate,
		ReadContext:   resourceAgentScheduleRead,
		UpdateContext: resourceAgentScheduleUpdate,
		DeleteContext: resourceAgentScheduleDelete,
		CustomizeDiff: resourceAgentScheduleCustomizeDiff,
		Schema: map[string]*schema.Schema{
			agentNameProp: {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "The name of the schedule. Names don't have to be unique. Changing it renames the schedule in place.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			agentEnabledProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the schedule starts jobs.",
			},
			agentFrequencyTypeProp: {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "How often the schedule fires: once, daily, weekly, monthly, monthly_relative, agent_start or idle.",
				ValidateDiagFunc: enumValidation(agentFrequencyTypes),
			},
			agentRecurrenceFactorProp: {
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          1,
				Description:      "Every N days, weeks or months for daily, weekly, monthly and monthly_relative schedules. Set active_start_date when N > 1: it decides which days count.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(1)),
			},
			agentWeeklyDaysProp: {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "The days a weekly schedule fires on: sunday to saturday. Required for weekly schedules.",
				Elem: &schema.Schema{
					Type:             schema.TypeString,
					ValidateDiagFunc: enumValidation(agentWeekDays),
				},
			},
			agentMonthlyDayProp: {
				Type:             schema.TypeInt,
				Optional:         true,
				Description:      "The day of the month (1-31) a monthly schedule fires on. Months without that day are skipped.",
				ValidateDiagFunc: validateAgentMonthlyDay,
			},
			agentMonthlyRelativeWeekProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "The week of the month for monthly_relative schedules: first, second, third, fourth or last.",
				ValidateDiagFunc: enumValidation(agentRelativeWeeks),
			},
			agentMonthlyRelativeDayProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "The day in that week for monthly_relative schedules: sunday to saturday, day, weekday or weekend_day.",
				ValidateDiagFunc: enumValidation(agentRelativeDays),
			},
			agentSubdayTypeProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "once",
				Description:      "How often the schedule fires on a day it runs: once (at active_start_time), seconds, minutes or hours.",
				ValidateDiagFunc: enumValidation(agentSubdayTypes),
			},
			agentSubdayIntervalProp: {
				Type:             schema.TypeInt,
				Optional:         true,
				Description:      "The number of seconds, minutes or hours between runs. Required unless subday_type is once; at least 10 for seconds.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(1)),
			},
			agentActiveStartDateProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				Description:      "The first day the schedule can fire, as YYYY-MM-DD. Defaults to the day the schedule is created.",
				ValidateDiagFunc: validateAgentDate,
			},
			agentActiveEndDateProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          agentMaxDate,
				Description:      "The last day the schedule can fire, as YYYY-MM-DD.",
				ValidateDiagFunc: validateAgentDate,
			},
			agentActiveStartTimeProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "00:00:00",
				Description:      "The time of day the schedule starts firing, as HH:MM:SS in the server's time zone.",
				ValidateDiagFunc: validateAgentTime,
			},
			agentActiveEndTimeProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          agentDefaultEndTime,
				Description:      "The time of day the schedule stops firing, as HH:MM:SS. An end time before the start time means a window that crosses midnight.",
				ValidateDiagFunc: validateAgentTime,
			},
			agentOwnerLoginProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The login that owns the schedule. Defaults to the login the provider connects with.",
			},
			agentScheduleIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the schedule in msdb.",
			},
			agentScheduleUidProp: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The GUID of the schedule.",
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

type valueGetter interface {
	Get(key string) interface{}
}

func agentScheduleSpecFrom(d valueGetter) agentScheduleSpec {
	var days []string
	for _, day := range d.Get(agentWeeklyDaysProp).(*schema.Set).List() {
		days = append(days, day.(string))
	}
	return agentScheduleSpec{
		FrequencyType:       d.Get(agentFrequencyTypeProp).(string),
		RecurrenceFactor:    d.Get(agentRecurrenceFactorProp).(int),
		WeeklyDays:          days,
		MonthlyDay:          d.Get(agentMonthlyDayProp).(int),
		MonthlyRelativeWeek: d.Get(agentMonthlyRelativeWeekProp).(string),
		MonthlyRelativeDay:  d.Get(agentMonthlyRelativeDayProp).(string),
		SubdayType:          d.Get(agentSubdayTypeProp).(string),
		SubdayInterval:      d.Get(agentSubdayIntervalProp).(int),
		ActiveStartDate:     d.Get(agentActiveStartDateProp).(string),
		ActiveEndDate:       d.Get(agentActiveEndDateProp).(string),
		ActiveStartTime:     d.Get(agentActiveStartTimeProp).(string),
		ActiveEndTime:       d.Get(agentActiveEndTimeProp).(string),
	}
}

func resourceAgentScheduleCustomizeDiff(_ context.Context, diff *schema.ResourceDiff, _ interface{}) error {
	unknown := map[string]bool{}
	for _, prop := range agentScheduleSpecProps {
		unknown[prop] = !diff.NewValueKnown(prop)
	}
	if unknown[agentFrequencyTypeProp] {
		return nil
	}
	return validateAgentScheduleSpec(agentScheduleSpecFrom(diff), unknown)
}

func agentScheduleFromData(data *schema.ResourceData) (*model.AgentSchedule, error) {
	s, err := encodeAgentSchedule(agentScheduleSpecFrom(data))
	if err != nil {
		return nil, err
	}
	s.ScheduleID = data.Get(agentScheduleIdProp).(int)
	s.Name = data.Get(agentNameProp).(string)
	s.Enabled = data.Get(agentEnabledProp).(bool)
	s.OwnerLoginName = data.Get(agentOwnerLoginProp).(string)
	return &s, nil
}

func resourceAgentScheduleCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_schedule", "create")
	logger.Debug().Msgf("Create agent schedule [%s]", data.Get(agentNameProp))

	connector, err := getAgentScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = connector.EnsureAgentAvailable(ctx); err != nil {
		return diag.FromErr(err)
	}

	s, err := agentScheduleFromData(data)
	if err != nil {
		return diag.FromErr(err)
	}
	id, err := connector.CreateAgentSchedule(ctx, s)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create agent schedule [%s]", s.Name))
	}

	logger.Info().Msgf("created agent schedule [%s] (%d)", s.Name, id)

	if err = data.Set(agentScheduleIdProp, id); err != nil {
		return diag.FromErr(err)
	}
	data.SetId(getAgentScheduleID(meta, id))

	return resourceAgentScheduleRead(ctx, data, meta)
}

func resourceAgentScheduleRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_schedule", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	connector, err := getAgentScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	id := data.Get(agentScheduleIdProp).(int)
	s, err := connector.GetAgentSchedule(ctx, id)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read agent schedule [%d]", id))
	}
	if s == nil {
		logger.Info().Msgf("No agent schedule found for [%d]", id)
		data.SetId("")
		return nil
	}

	spec := decodeAgentSchedule(*s)
	values := map[string]interface{}{
		agentNameProp:                s.Name,
		agentEnabledProp:             s.Enabled,
		agentOwnerLoginProp:          s.OwnerLoginName,
		agentScheduleUidProp:         s.ScheduleUID,
		agentFrequencyTypeProp:       spec.FrequencyType,
		agentRecurrenceFactorProp:    spec.RecurrenceFactor,
		agentWeeklyDaysProp:          spec.WeeklyDays,
		agentMonthlyDayProp:          spec.MonthlyDay,
		agentMonthlyRelativeWeekProp: spec.MonthlyRelativeWeek,
		agentMonthlyRelativeDayProp:  spec.MonthlyRelativeDay,
		agentSubdayTypeProp:          spec.SubdayType,
		agentSubdayIntervalProp:      spec.SubdayInterval,
		agentActiveStartDateProp:     spec.ActiveStartDate,
		agentActiveEndDateProp:       spec.ActiveEndDate,
		agentActiveStartTimeProp:     spec.ActiveStartTime,
		agentActiveEndTimeProp:       spec.ActiveEndTime,
	}
	for k, v := range values {
		if err = data.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceAgentScheduleUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_schedule", "update")
	logger.Debug().Msgf("Update %s", data.Id())

	connector, err := getAgentScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	s, err := agentScheduleFromData(data)
	if err != nil {
		return diag.FromErr(err)
	}
	if !data.HasChange(agentOwnerLoginProp) {
		// Leave the owner alone unless it changes: changing it needs sysadmin.
		s.OwnerLoginName = ""
	}
	if err = connector.UpdateAgentSchedule(ctx, s); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to update agent schedule [%s]", s.Name))
	}

	logger.Info().Msgf("updated agent schedule [%s]", s.Name)

	return resourceAgentScheduleRead(ctx, data, meta)
}

func resourceAgentScheduleDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_schedule", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	connector, err := getAgentScheduleConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	id := data.Get(agentScheduleIdProp).(int)
	if err = connector.DeleteAgentSchedule(ctx, id); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete agent schedule [%s]", data.Get(agentNameProp)))
	}

	logger.Info().Msgf("deleted agent schedule [%d]", id)

	data.SetId("")

	return nil
}

func getAgentScheduleID(meta interface{}, scheduleID int) string {
	provider := meta.(sqlserverProvider)
	return fmt.Sprintf("sqlserver://%s:%s/agent_schedule/%d", provider.host, provider.port, scheduleID)
}

func getAgentScheduleConnector(meta interface{}, data *schema.ResourceData) (AgentScheduleConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(AgentScheduleConnector), nil
}
