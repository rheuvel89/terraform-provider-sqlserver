package sql

import (
	"context"
	"database/sql"
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"
)

func (c *Connector) GetResourcePool(ctx context.Context, name string) (*model.ResourcePool, error) {
	var pool model.ResourcePool
	err := c.QueryRowContext(ctx,
		`SELECT 
			pool_id,
			name,
			min_cpu_percent,
			max_cpu_percent,
			min_memory_percent,
			max_memory_percent,
			cap_cpu_percent,
			min_iops_per_volume,
			max_iops_per_volume
		FROM sys.resource_governor_resource_pools
		WHERE name = @name`,
		func(r *sql.Row) error {
			return r.Scan(
				&pool.PoolID,
				&pool.Name,
				&pool.MinCPUPercent,
				&pool.MaxCPUPercent,
				&pool.MinMemoryPercent,
				&pool.MaxMemoryPercent,
				&pool.CapCPUPercent,
				&pool.MinIOPSPerVolume,
				&pool.MaxIOPSPerVolume,
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
	return &pool, nil
}

func (c *Connector) CreateResourcePool(ctx context.Context, pool *model.ResourcePool) error {
	cmd := fmt.Sprintf(`CREATE RESOURCE POOL %s WITH (
		MIN_CPU_PERCENT = %d,
		MAX_CPU_PERCENT = %d,
		MIN_MEMORY_PERCENT = %d,
		MAX_MEMORY_PERCENT = %d,
		CAP_CPU_PERCENT = %d,
		MIN_IOPS_PER_VOLUME = %d,
		MAX_IOPS_PER_VOLUME = %d
	)`,
		quoteName(pool.Name),
		pool.MinCPUPercent,
		pool.MaxCPUPercent,
		pool.MinMemoryPercent,
		pool.MaxMemoryPercent,
		pool.CapCPUPercent,
		pool.MinIOPSPerVolume,
		pool.MaxIOPSPerVolume,
	)

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Apply the configuration
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
}

func (c *Connector) UpdateResourcePool(ctx context.Context, pool *model.ResourcePool) error {
	cmd := fmt.Sprintf(`ALTER RESOURCE POOL %s WITH (
		MIN_CPU_PERCENT = %d,
		MAX_CPU_PERCENT = %d,
		MIN_MEMORY_PERCENT = %d,
		MAX_MEMORY_PERCENT = %d,
		CAP_CPU_PERCENT = %d,
		MIN_IOPS_PER_VOLUME = %d,
		MAX_IOPS_PER_VOLUME = %d
	)`,
		quoteName(pool.Name),
		pool.MinCPUPercent,
		pool.MaxCPUPercent,
		pool.MinMemoryPercent,
		pool.MaxMemoryPercent,
		pool.CapCPUPercent,
		pool.MinIOPSPerVolume,
		pool.MaxIOPSPerVolume,
	)

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Apply the configuration
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
}

func (c *Connector) DeleteResourcePool(ctx context.Context, name string) error {
	cmd := fmt.Sprintf("DROP RESOURCE POOL %s", quoteName(name))

	if err := c.ExecContext(ctx, cmd); err != nil {
		return err
	}

	// Apply the configuration
	return c.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
}

// quoteName quotes a SQL Server identifier
func quoteName(name string) string {
	return "[" + name + "]"
}
