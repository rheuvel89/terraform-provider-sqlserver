package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceGovernor() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourceGovernorCreate,
		ReadContext:   resourceResourceGovernorRead,
		UpdateContext: resourceResourceGovernorUpdate,
		DeleteContext: resourceResourceGovernorDelete,
		Schema: map[string]*schema.Schema{
			"enabled": {
				Type:     schema.TypeBool,
				Required: true,
			},
			"classifier_function": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceResourceGovernorCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	enabled := d.Get("enabled").(bool)
	classifier := d.Get("classifier_function").(string)

	var err error
	if enabled {
		_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR RECONFIGURE; ALTER RESOURCE GOVERNOR ENABLE")
	} else {
		_, err = conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR DISABLE")
	}
	if err != nil {
		return diag.FromErr(err)
	}

	if classifier != "" {
		_, err = conn.ExecContext(ctx, fmt.Sprintf("ALTER RESOURCE GOVERNOR WITH (CLASSIFIER_FUNCTION = %s)", classifier))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId("resource_governor")
	return resourceResourceGovernorRead(ctx, d, m)
}

func resourceResourceGovernorRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	var enabled int
	err := conn.QueryRowContext(ctx, "SELECT is_enabled FROM sys.resource_governor_configuration").Scan(&enabled)
	if err != nil {
		return diag.FromErr(err)
	}
	d.Set("enabled", enabled == 1)

	var classifier sql.NullString
	err = conn.QueryRowContext(ctx, "SELECT OBJECT_NAME(classifier_function_id) FROM sys.resource_governor_configuration").Scan(&classifier)
	if err != nil {
		return diag.FromErr(err)
	}
	if classifier.Valid {
		d.Set("classifier_function", classifier.String)
	} else {
		d.Set("classifier_function", "")
	}
	return nil
}

func resourceResourceGovernorUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceResourceGovernorCreate(ctx, d, m)
}

func resourceResourceGovernorDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	_, err := conn.ExecContext(ctx, "ALTER RESOURCE GOVERNOR DISABLE")
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
