package sql

import (
	"context"
	"database/sql"
)

func (c *Connector) GetAgentJobSchedule(ctx context.Context, jobID string, scheduleID int) (bool, error) {
	var exists int
	err := c.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM [msdb].[dbo].[sysjobschedules]
         WHERE [job_id] = TRY_CONVERT(uniqueidentifier, @jobId) AND [schedule_id] = @scheduleId`,
		func(r *sql.Row) error {
			return r.Scan(&exists)
		},
		sql.Named("jobId", jobID),
		sql.Named("scheduleId", scheduleID),
	)
	return exists > 0, err
}

func (c *Connector) CreateAgentJobSchedule(ctx context.Context, jobID string, scheduleID int) error {
	return c.agentExec(ctx,
		`EXEC [msdb].[dbo].[sp_attach_schedule] @job_id = @jobId, @schedule_id = @scheduleId`,
		sql.Named("jobId", jobID),
		sql.Named("scheduleId", scheduleID),
	)
}

// DeleteAgentJobSchedule detaches the schedule from the job but never deletes the schedule, which is managed separately.
// Terraform detaches a schedule before deleting it, since the attachment depends on the schedule; sp_delete_schedule
// with @force_delete = 0 refuses to delete a schedule that is still attached. When a schedule is deleted outside
// Terraform, SQL Server removes its attachments and the next refresh drops this resource from the state.
func (c *Connector) DeleteAgentJobSchedule(ctx context.Context, jobID string, scheduleID int) error {
	return c.agentExec(ctx,
		`IF EXISTS (SELECT 1 FROM [msdb].[dbo].[sysjobschedules]
                    WHERE [job_id] = TRY_CONVERT(uniqueidentifier, @jobId) AND [schedule_id] = @scheduleId)
           EXEC [msdb].[dbo].[sp_detach_schedule] @job_id = @jobId, @schedule_id = @scheduleId, @delete_unused_schedule = 0`,
		sql.Named("jobId", jobID),
		sql.Named("scheduleId", scheduleID),
	)
}
