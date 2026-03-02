package sql

import (
	"context"
	"database/sql"
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetWorkloadGroup(ctx context.Context, name string) (*model.WorkloadGroup, error) {
	var group model.WorkloadGroup
	err := c.QueryRowContext(ctx,
		`SELECT 
			g.group_id,
			g.name,
			p.name AS pool_name,
			g.importance,
			g.request_max_memory_grant_percent,
			g.request_max_cpu_time_sec,
			g.request_memory_grant_timeout_sec,
			g.max_dop,
			g.group_max_requests
		FROM sys.resource_governor_workload_groups g
		INNER JOIN sys.resource_governor_resource_pools p ON g.pool_id = p.pool_id
		WHERE g.name = @name`,
		func(r *sql.Row) error {
			return r.Scan(
				&group.GroupID,
				&group.Name,
				&group.PoolName,
				&group.Importance,
				&group.RequestMaxMemoryGrantPercent,
				&group.RequestMaxCPUTimeSec,
				&group.RequestMemoryGrantTimeoutSec,
				&group.MaxDOP,
				&group.GroupMaxRequests,
			)
		},
		sql.Named("name", name),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &group, nil
}

func (c *Connector) CreateWorkloadGroup(ctx context.Context, group *model.WorkloadGroup) error {
	cmd := fmt.Sprintf(`CREATE WORKLOAD GROUP %s WITH (
		IMPORTANCE = %s,
		REQUEST_MAX_MEMORY_GRANT_PERCENT = %d,
		REQUEST_MAX_CPU_TIME_SEC = %d,
		REQUEST_MEMORY_GRANT_TIMEOUT_SEC = %d,
		MAX_DOP = %d,
		GROUP_MAX_REQUESTS = %d
	) USING %s`,
		quoteName(group.Name),
		group.Importance,
		group.RequestMaxMemoryGrantPercent,
		group.RequestMaxCPUTimeSec,
		group.RequestMemoryGrantTimeoutSec,
		group.MaxDOP,
		group.GroupMaxRequests,
		quoteName(group.PoolName),
	)

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Apply the configuration
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
}

func (c *Connector) UpdateWorkloadGroup(ctx context.Context, group *model.WorkloadGroup) error {
	cmd := fmt.Sprintf(`ALTER WORKLOAD GROUP %s WITH (
		IMPORTANCE = %s,
		REQUEST_MAX_MEMORY_GRANT_PERCENT = %d,
		REQUEST_MAX_CPU_TIME_SEC = %d,
		REQUEST_MEMORY_GRANT_TIMEOUT_SEC = %d,
		MAX_DOP = %d,
		GROUP_MAX_REQUESTS = %d
	) USING %s`,
		quoteName(group.Name),
		group.Importance,
		group.RequestMaxMemoryGrantPercent,
		group.RequestMaxCPUTimeSec,
		group.RequestMemoryGrantTimeoutSec,
		group.MaxDOP,
		group.GroupMaxRequests,
		quoteName(group.PoolName),
	)

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Apply the configuration
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
}

func (c *Connector) DeleteWorkloadGroup(ctx context.Context, name string) error {
	cmd := fmt.Sprintf("DROP WORKLOAD GROUP %s", quoteName(name))

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Apply the configuration
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
}
