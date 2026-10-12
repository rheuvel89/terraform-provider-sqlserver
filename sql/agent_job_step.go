package sql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"
)

// agentJobStepUpdatable lists the sp_update_jobstep parameters UpdateAgentJobStep accepts.
var agentJobStepUpdatable = map[string]bool{
	"command": true, "database_name": true, "database_user_name": true,
	"on_success_action": true, "on_success_step_id": true, "on_fail_action": true, "on_fail_step_id": true,
	"retry_attempts": true, "retry_interval": true, "output_file_name": true, "flags": true,
}

// ListAgentJobSteps returns every step of the job, ordered by step number.
func (c *Connector) ListAgentJobSteps(ctx context.Context, jobID string) ([]model.AgentJobStep, error) {
	var steps []model.AgentJobStep
	err := c.QueryContext(ctx,
		`SELECT CONVERT(nvarchar(36), [job_id]), [step_id], CONVERT(nvarchar(36), [step_uid]), [step_name], COALESCE([command], ''),
                COALESCE([database_name], 'master'), COALESCE([database_user_name], ''),
                [on_success_action], [on_success_step_id], [on_fail_action], [on_fail_step_id],
                [retry_attempts], [retry_interval], COALESCE([output_file_name], ''), [flags]
         FROM [msdb].[dbo].[sysjobsteps]
         WHERE [job_id] = TRY_CONVERT(uniqueidentifier, @jobId)
         ORDER BY [step_id]`,
		func(rows *sql.Rows) error {
			for rows.Next() {
				var s model.AgentJobStep
				if err := rows.Scan(&s.JobID, &s.StepID, &s.StepUID, &s.Name, &s.Command, &s.DatabaseName, &s.DatabaseUserName,
					&s.OnSuccessAction, &s.OnSuccessStepID, &s.OnFailAction, &s.OnFailStepID,
					&s.RetryAttempts, &s.RetryInterval, &s.OutputFileName, &s.Flags); err != nil {
					return err
				}
				steps = append(steps, s)
			}
			return rows.Err()
		},
		sql.Named("jobId", jobID),
	)
	return steps, err
}

// AddAgentJobStep inserts a T-SQL step at s.StepID. msdb renumbers the steps and jumps after it.
func (c *Connector) AddAgentJobStep(ctx context.Context, s *model.AgentJobStep) error {
	return c.agentExec(ctx,
		`EXEC [msdb].[dbo].[sp_add_jobstep] @job_id = @jobId, @step_id = @stepId, @step_name = @name, @subsystem = N'TSQL',
              @command = @command, @database_name = @database, @database_user_name = @databaseUser,
              @on_success_action = @onSuccessAction, @on_success_step_id = @onSuccessStepId,
              @on_fail_action = @onFailAction, @on_fail_step_id = @onFailStepId,
              @retry_attempts = @retryAttempts, @retry_interval = @retryInterval,
              @output_file_name = @outputFile, @flags = @flags`,
		sql.Named("jobId", s.JobID),
		sql.Named("stepId", s.StepID),
		sql.Named("name", s.Name),
		sql.Named("command", s.Command),
		sql.Named("database", nullIfEmpty(s.DatabaseName)),
		sql.Named("databaseUser", nullIfEmpty(s.DatabaseUserName)),
		sql.Named("onSuccessAction", s.OnSuccessAction),
		sql.Named("onSuccessStepId", s.OnSuccessStepID),
		sql.Named("onFailAction", s.OnFailAction),
		sql.Named("onFailStepId", s.OnFailStepID),
		sql.Named("retryAttempts", s.RetryAttempts),
		sql.Named("retryInterval", s.RetryInterval),
		sql.Named("outputFile", nullIfEmpty(s.OutputFileName)),
		sql.Named("flags", s.Flags),
	)
}

// UpdateAgentJobStep passes only the given sp_update_jobstep parameters; the procedure keeps the current value of
// every parameter left out. Pass "" to clear database_user_name or output_file_name.
func (c *Connector) UpdateAgentJobStep(ctx context.Context, jobID string, stepID int, changes map[string]interface{}) error {
	if stepID < 1 {
		return fmt.Errorf("refusing to update step number %d of job %s", stepID, jobID)
	}
	if len(changes) == 0 {
		return nil
	}
	names := make([]string, 0, len(changes))
	for name := range changes {
		if !agentJobStepUpdatable[name] {
			return fmt.Errorf("sp_update_jobstep parameter %q is not supported", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)

	assignments := []string{"@job_id = @jobId", "@step_id = @stepId"}
	args := []interface{}{sql.Named("jobId", jobID), sql.Named("stepId", stepID)}
	for _, name := range names {
		assignments = append(assignments, fmt.Sprintf("@%s = @p_%s", name, name))
		args = append(args, sql.Named("p_"+name, changes[name]))
	}
	return c.agentExec(ctx, `EXEC [msdb].[dbo].[sp_update_jobstep] `+strings.Join(assignments, ", "), args...)
}

// DeleteAgentJobStep deletes one step. sp_delete_jobstep deletes every step of the job when given step 0,
// so anything below 1 is refused.
func (c *Connector) DeleteAgentJobStep(ctx context.Context, jobID string, stepID int) error {
	if stepID < 1 {
		return fmt.Errorf("refusing to delete step number %d of job %s: sp_delete_jobstep deletes all steps for 0", stepID, jobID)
	}
	return c.agentExec(ctx,
		`EXEC [msdb].[dbo].[sp_delete_jobstep] @job_id = @jobId, @step_id = @stepId`,
		sql.Named("jobId", jobID),
		sql.Named("stepId", stepID),
	)
}

// GetAgentRunAsInfo returns whether the connecting login is sysadmin, and the job's owner and whether that owner is sysadmin.
func (c *Connector) GetAgentRunAsInfo(ctx context.Context, jobID string) (callerIsSysadmin bool, owner string, ownerIsSysadmin bool, err error) {
	err = c.QueryRowContext(ctx,
		`SELECT CAST(COALESCE(IS_SRVROLEMEMBER('sysadmin'), 0) AS bit), COALESCE(SUSER_SNAME(j.[owner_sid]), ''),
                CAST(COALESCE(IS_SRVROLEMEMBER('sysadmin', SUSER_SNAME(j.[owner_sid])), 0) AS bit)
         FROM [msdb].[dbo].[sysjobs] j
         WHERE j.[job_id] = TRY_CONVERT(uniqueidentifier, @jobId)`,
		func(r *sql.Row) error {
			return r.Scan(&callerIsSysadmin, &owner, &ownerIsSysadmin)
		},
		sql.Named("jobId", jobID),
	)
	return
}
