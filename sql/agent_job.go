package sql

import (
	"context"
	"database/sql"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetAgentJob(ctx context.Context, jobID string) (*model.AgentJob, error) {
	var job model.AgentJob
	err := c.QueryRowContext(ctx,
		`SELECT CONVERT(nvarchar(36), j.[job_id]), j.[name], COALESCE(j.[description], ''), CAST(j.[enabled] AS bit),
                COALESCE(c.[name], ''), COALESCE(SUSER_SNAME(j.[owner_sid]), ''),
                j.[notify_level_eventlog], j.[notify_level_email], COALESCE(o.[name], ''), j.[delete_level], j.[start_step_id]
         FROM [msdb].[dbo].[sysjobs] j
         LEFT JOIN [msdb].[dbo].[syscategories] c ON c.[category_id] = j.[category_id]
         LEFT JOIN [msdb].[dbo].[sysoperators] o ON o.[id] = j.[notify_email_operator_id]
         WHERE j.[job_id] = TRY_CONVERT(uniqueidentifier, @jobId)`,
		func(r *sql.Row) error {
			return r.Scan(&job.JobID, &job.Name, &job.Description, &job.Enabled, &job.CategoryName, &job.OwnerLoginName,
				&job.NotifyLevelEventlog, &job.NotifyLevelEmail, &job.NotifyEmailOperatorName, &job.DeleteLevel, &job.StartStepID)
		},
		sql.Named("jobId", jobID),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

// CreateAgentJob creates the job, targets it at the local server (a job without a target server never runs)
// and returns the new job ID.
func (c *Connector) CreateAgentJob(ctx context.Context, job *model.AgentJob) (string, error) {
	var id string
	err := c.agentExec(ctx,
		`SET NOCOUNT ON
         SET XACT_ABORT ON
         DECLARE @id uniqueidentifier, @rc int
         BEGIN TRANSACTION
         EXEC @rc = [msdb].[dbo].[sp_add_job] @job_name = @name, @enabled = @enabled, @description = @description,
              @category_name = @category, @owner_login_name = @owner,
              @notify_level_eventlog = @notifyEventlog, @notify_level_email = @notifyEmail,
              @notify_email_operator_name = @notifyEmailOperator, @delete_level = @deleteLevel, @job_id = @id OUTPUT
         IF @rc <> 0 BEGIN ROLLBACK TRANSACTION; RETURN END
         -- sp_add_job stores a placeholder text when the description is empty
         IF @description IS NULL
         BEGIN
           EXEC @rc = [msdb].[dbo].[sp_update_job] @job_id = @id, @description = N''
           IF @rc <> 0 BEGIN ROLLBACK TRANSACTION; RETURN END
         END
         EXEC @rc = [msdb].[dbo].[sp_add_jobserver] @job_id = @id, @server_name = N'(local)'
         IF @rc <> 0 BEGIN ROLLBACK TRANSACTION; RETURN END
         COMMIT TRANSACTION
         SET @newId = CONVERT(nvarchar(36), @id)`,
		sql.Named("name", job.Name),
		sql.Named("enabled", job.Enabled),
		sql.Named("description", nullIfEmpty(job.Description)),
		sql.Named("category", nullIfEmpty(job.CategoryName)),
		sql.Named("owner", nullIfEmpty(job.OwnerLoginName)),
		sql.Named("notifyEventlog", job.NotifyLevelEventlog),
		sql.Named("notifyEmail", job.NotifyLevelEmail),
		sql.Named("notifyEmailOperator", nullIfEmpty(job.NotifyEmailOperatorName)),
		sql.Named("deleteLevel", job.DeleteLevel),
		sql.Named("newId", sql.Out{Dest: &id}),
	)
	return id, err
}

// UpdateAgentJob sets every attribute of the job to the values in job. An empty description or operator
// name clears it; an empty owner keeps the current owner.
func (c *Connector) UpdateAgentJob(ctx context.Context, job *model.AgentJob) error {
	return c.agentExec(ctx,
		`EXEC [msdb].[dbo].[sp_update_job] @job_id = @jobId, @new_name = @name, @enabled = @enabled, @description = @description,
              @category_name = @category, @owner_login_name = @owner,
              @notify_level_eventlog = @notifyEventlog, @notify_level_email = @notifyEmail,
              @notify_email_operator_name = @notifyEmailOperator, @delete_level = @deleteLevel`,
		sql.Named("jobId", job.JobID),
		sql.Named("name", job.Name),
		sql.Named("enabled", job.Enabled),
		sql.Named("description", job.Description),
		sql.Named("category", nullIfEmpty(job.CategoryName)),
		sql.Named("owner", nullIfEmpty(job.OwnerLoginName)),
		sql.Named("notifyEventlog", job.NotifyLevelEventlog),
		sql.Named("notifyEmail", job.NotifyLevelEmail),
		sql.Named("notifyEmailOperator", job.NotifyEmailOperatorName),
		sql.Named("deleteLevel", job.DeleteLevel),
	)
}

// DeleteAgentJob deletes the job. sp_delete_job itself deletes the job's steps, its target server and its schedule
// attachments; @delete_unused_schedule = 0 keeps the schedules themselves, which are managed separately.
func (c *Connector) DeleteAgentJob(ctx context.Context, jobID string) error {
	return c.agentExec(ctx,
		`IF EXISTS (SELECT 1 FROM [msdb].[dbo].[sysjobs] WHERE [job_id] = TRY_CONVERT(uniqueidentifier, @jobId))
           EXEC [msdb].[dbo].[sp_delete_job] @job_id = @jobId, @delete_unused_schedule = 0`,
		sql.Named("jobId", jobID),
	)
}
