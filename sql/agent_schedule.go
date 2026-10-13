package sql

import (
	"context"
	"database/sql"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetAgentSchedule(ctx context.Context, scheduleID int) (*model.AgentSchedule, error) {
	var s model.AgentSchedule
	err := c.QueryRowContext(ctx,
		`SELECT [schedule_id], CONVERT(nvarchar(36), [schedule_uid]), [name], CAST([enabled] AS bit),
                [freq_type], [freq_interval], [freq_subday_type], [freq_subday_interval], [freq_relative_interval], [freq_recurrence_factor],
                [active_start_date], [active_end_date], [active_start_time], [active_end_time], COALESCE(SUSER_SNAME([owner_sid]), '')
         FROM [msdb].[dbo].[sysschedules]
         WHERE [schedule_id] = @id`,
		func(r *sql.Row) error {
			return r.Scan(&s.ScheduleID, &s.ScheduleUID, &s.Name, &s.Enabled,
				&s.FreqType, &s.FreqInterval, &s.FreqSubdayType, &s.FreqSubdayInterval, &s.FreqRelativeInterval, &s.FreqRecurrenceFactor,
				&s.ActiveStartDate, &s.ActiveEndDate, &s.ActiveStartTime, &s.ActiveEndTime, &s.OwnerLoginName)
		},
		sql.Named("id", scheduleID),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// CreateAgentSchedule creates the schedule and returns its ID. A zero active start date lets SQL Server use today.
func (c *Connector) CreateAgentSchedule(ctx context.Context, s *model.AgentSchedule) (int, error) {
	var id int
	err := c.agentExec(ctx,
		`SET NOCOUNT ON
         EXEC [msdb].[dbo].[sp_add_schedule] @schedule_name = @name, @enabled = @enabled,
              @freq_type = @freqType, @freq_interval = @freqInterval, @freq_subday_type = @subdayType,
              @freq_subday_interval = @subdayInterval, @freq_relative_interval = @relativeInterval,
              @freq_recurrence_factor = @recurrenceFactor, @active_start_date = @startDate, @active_end_date = @endDate,
              @active_start_time = @startTime, @active_end_time = @endTime, @owner_login_name = @owner,
              @schedule_id = @newId OUTPUT`,
		append(agentScheduleArgs(s), sql.Named("newId", sql.Out{Dest: &id}))...,
	)
	return id, err
}

// UpdateAgentSchedule sets every attribute of the schedule to the values in s. A zero active start date and an empty
// owner keep the current values.
func (c *Connector) UpdateAgentSchedule(ctx context.Context, s *model.AgentSchedule) error {
	return c.agentExec(ctx,
		`EXEC [msdb].[dbo].[sp_update_schedule] @schedule_id = @scheduleId, @new_name = @name, @enabled = @enabled,
              @freq_type = @freqType, @freq_interval = @freqInterval, @freq_subday_type = @subdayType,
              @freq_subday_interval = @subdayInterval, @freq_relative_interval = @relativeInterval,
              @freq_recurrence_factor = @recurrenceFactor, @active_start_date = @startDate, @active_end_date = @endDate,
              @active_start_time = @startTime, @active_end_time = @endTime, @owner_login_name = @owner`,
		append(agentScheduleArgs(s), sql.Named("scheduleId", s.ScheduleID))...,
	)
}

// DeleteAgentSchedule deletes the schedule. It fails while the schedule is still attached to a job.
func (c *Connector) DeleteAgentSchedule(ctx context.Context, scheduleID int) error {
	return c.agentExec(ctx,
		`IF EXISTS (SELECT 1 FROM [msdb].[dbo].[sysschedules] WHERE [schedule_id] = @id)
           EXEC [msdb].[dbo].[sp_delete_schedule] @schedule_id = @id, @force_delete = 0`,
		sql.Named("id", scheduleID),
	)
}

func agentScheduleArgs(s *model.AgentSchedule) []interface{} {
	var startDate interface{}
	if s.ActiveStartDate != 0 {
		startDate = s.ActiveStartDate
	}
	return []interface{}{
		sql.Named("name", s.Name),
		sql.Named("enabled", s.Enabled),
		sql.Named("freqType", s.FreqType),
		sql.Named("freqInterval", s.FreqInterval),
		sql.Named("subdayType", s.FreqSubdayType),
		sql.Named("subdayInterval", s.FreqSubdayInterval),
		sql.Named("relativeInterval", s.FreqRelativeInterval),
		sql.Named("recurrenceFactor", s.FreqRecurrenceFactor),
		sql.Named("startDate", startDate),
		sql.Named("endDate", s.ActiveEndDate),
		sql.Named("startTime", s.ActiveStartTime),
		sql.Named("endTime", s.ActiveEndTime),
		sql.Named("owner", nullIfEmpty(s.OwnerLoginName)),
	}
}
