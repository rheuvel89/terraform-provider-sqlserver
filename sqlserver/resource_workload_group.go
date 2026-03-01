package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceGovernorWorkloadGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourceGovernorWorkloadGroupCreate,
		ReadContext:   resourceResourceGovernorWorkloadGroupRead,
		UpdateContext: resourceResourceGovernorWorkloadGroupUpdate,
		DeleteContext: resourceResourceGovernorWorkloadGroupDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"pool_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"importance": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Medium",
			},
			"request_max_memory_grant_percent": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  25,
			},
			"request_max_cpu_time_sec": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"request_memory_grant_timeout_sec": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"max_dop": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
		},
	}
}

func resourceResourceGovernorWorkloadGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Get("name").(string)
	pool := d.Get("pool_name").(string)
	importance := d.Get("importance").(string)
	maxMemGrant := d.Get("request_max_memory_grant_percent").(int)
	maxCPUTime := d.Get("request_max_cpu_time_sec").(int)
	memGrantTimeout := d.Get("request_memory_grant_timeout_sec").(int)
	maxDop := d.Get("max_dop").(int)

	_, err := conn.ExecContext(ctx, fmt.Sprintf(
		`CREATE WORKLOAD GROUP [%s] USING [%s] WITH (IMPORTANCE = '%s', REQUEST_MAX_MEMORY_GRANT_PERCENT = %d, REQUEST_MAX_CPU_TIME_SEC = %d, REQUEST_MEMORY_GRANT_TIMEOUT_SEC = %d, MAX_DOP = %d)`,
		name, pool, importance, maxMemGrant, maxCPUTime, memGrantTimeout, maxDop,
	))
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(name)
	return resourceResourceGovernorWorkloadGroupRead(ctx, d, m)
}

func resourceResourceGovernorWorkloadGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	var pool, importance string
	var maxMemGrant, maxCPUTime, memGrantTimeout, maxDop int
	err := conn.QueryRowContext(ctx, `SELECT p.name, g.importance, g.request_max_memory_grant_percent, g.request_max_cpu_time_sec, g.request_memory_grant_timeout_sec, g.max_dop FROM sys.resource_governor_workload_groups g JOIN sys.resource_governor_resource_pools p ON g.pool_id = p.pool_id WHERE g.name = @p1`, name).Scan(&pool, &importance, &maxMemGrant, &maxCPUTime, &memGrantTimeout, &maxDop)
	if err != nil {
		if err == sql.ErrNoRows {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	d.Set("pool_name", pool)
	d.Set("importance", importance)
	d.Set("request_max_memory_grant_percent", maxMemGrant)
	d.Set("request_max_cpu_time_sec", maxCPUTime)
	d.Set("request_memory_grant_timeout_sec", memGrantTimeout)
	d.Set("max_dop", maxDop)
	return nil
}

func resourceResourceGovernorWorkloadGroupUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	pool := d.Get("pool_name").(string)
	importance := d.Get("importance").(string)
	maxMemGrant := d.Get("request_max_memory_grant_percent").(int)
	maxCPUTime := d.Get("request_max_cpu_time_sec").(int)
	memGrantTimeout := d.Get("request_memory_grant_timeout_sec").(int)
	maxDop := d.Get("max_dop").(int)

	_, err := conn.ExecContext(ctx, fmt.Sprintf(
		`ALTER WORKLOAD GROUP [%s] USING [%s] WITH (IMPORTANCE = '%s', REQUEST_MAX_MEMORY_GRANT_PERCENT = %d, REQUEST_MAX_CPU_TIME_SEC = %d, REQUEST_MEMORY_GRANT_TIMEOUT_SEC = %d, MAX_DOP = %d)`,
		name, pool, importance, maxMemGrant, maxCPUTime, memGrantTimeout, maxDop,
	))
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
	if err != nil {
		return diag.FromErr(err)
	}
	return resourceResourceGovernorWorkloadGroupRead(ctx, d, m)
}

func resourceResourceGovernorWorkloadGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	_, err := conn.ExecContext(ctx, fmt.Sprintf("DROP WORKLOAD GROUP [%s]", name))
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
