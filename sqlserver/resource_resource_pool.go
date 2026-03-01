package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceGovernorResourcePool() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourceGovernorResourcePoolCreate,
		ReadContext:   resourceResourceGovernorResourcePoolRead,
		UpdateContext: resourceResourceGovernorResourcePoolUpdate,
		DeleteContext: resourceResourceGovernorResourcePoolDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"min_memory_percent": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"max_memory_percent": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  100,
			},
			"min_cpu_percent": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"max_cpu_percent": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  100,
			},
			"cap_cpu_percent": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  100,
			},
		},
	}
}

func resourceResourceGovernorResourcePoolCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Get("name").(string)
	minMem := d.Get("min_memory_percent").(int)
	maxMem := d.Get("max_memory_percent").(int)
	minCPU := d.Get("min_cpu_percent").(int)
	maxCPU := d.Get("max_cpu_percent").(int)
	capCPU := d.Get("cap_cpu_percent").(int)

	_, err := conn.ExecContext(ctx, fmt.Sprintf(
		`CREATE RESOURCE POOL [%s] WITH (MIN_MEMORY_PERCENT = %d, MAX_MEMORY_PERCENT = %d, MIN_CPU_PERCENT = %d, MAX_CPU_PERCENT = %d, CAP_CPU_PERCENT = %d)`,
		name, minMem, maxMem, minCPU, maxCPU, capCPU,
	))
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(name)
	return resourceResourceGovernorResourcePoolRead(ctx, d, m)
}

func resourceResourceGovernorResourcePoolRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	var minMem, maxMem, minCPU, maxCPU, capCPU int
	err := conn.QueryRowContext(ctx, `SELECT min_memory_percent, max_memory_percent, min_cpu_percent, max_cpu_percent, cap_cpu_percent FROM sys.resource_governor_resource_pools WHERE name = @p1`, name).Scan(&minMem, &maxMem, &minCPU, &maxCPU, &capCPU)
	if err != nil {
		if err == sql.ErrNoRows {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	d.Set("min_memory_percent", minMem)
	d.Set("max_memory_percent", maxMem)
	d.Set("min_cpu_percent", minCPU)
	d.Set("max_cpu_percent", maxCPU)
	d.Set("cap_cpu_percent", capCPU)
	return nil
}

func resourceResourceGovernorResourcePoolUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	minMem := d.Get("min_memory_percent").(int)
	maxMem := d.Get("max_memory_percent").(int)
	minCPU := d.Get("min_cpu_percent").(int)
	maxCPU := d.Get("max_cpu_percent").(int)
	capCPU := d.Get("cap_cpu_percent").(int)

	_, err := conn.ExecContext(ctx, fmt.Sprintf(
		`ALTER RESOURCE POOL [%s] WITH (MIN_MEMORY_PERCENT = %d, MAX_MEMORY_PERCENT = %d, MIN_CPU_PERCENT = %d, MAX_CPU_PERCENT = %d, CAP_CPU_PERCENT = %d)`,
		name, minMem, maxMem, minCPU, maxCPU, capCPU,
	))
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE")
	if err != nil {
		return diag.FromErr(err)
	}
	return resourceResourceGovernorResourcePoolRead(ctx, d, m)
}

func resourceResourceGovernorResourcePoolDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	_, err := conn.ExecContext(ctx, fmt.Sprintf("DROP RESOURCE POOL [%s]", name))
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
