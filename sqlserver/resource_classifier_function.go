package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceGovernorClassifierFunction() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourceGovernorClassifierFunctionCreate,
		ReadContext:   resourceResourceGovernorClassifierFunctionRead,
		UpdateContext: resourceResourceGovernorClassifierFunctionUpdate,
		DeleteContext: resourceResourceGovernorClassifierFunctionDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"definition": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceResourceGovernorClassifierFunctionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Get("name").(string)
	definition := d.Get("definition").(string)

	_, err := conn.ExecContext(ctx, fmt.Sprintf("CREATE FUNCTION [%s]() RETURNS sysname WITH SCHEMABINDING AS BEGIN %s END", name, definition))
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(name)
	return resourceResourceGovernorClassifierFunctionRead(ctx, d, m)
}

func resourceResourceGovernorClassifierFunctionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	var definition string
	err := conn.QueryRowContext(ctx, `SELECT OBJECT_DEFINITION(OBJECT_ID(@p1))`, name).Scan(&definition)
	if err != nil {
		if err == sql.ErrNoRows {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	d.Set("definition", definition)
	return nil
}

func resourceResourceGovernorClassifierFunctionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	definition := d.Get("definition").(string)

	_, err := conn.ExecContext(ctx, fmt.Sprintf("ALTER FUNCTION [%s]() RETURNS sysname WITH SCHEMABINDING AS BEGIN %s END", name, definition))
	if err != nil {
		return diag.FromErr(err)
	}
	return resourceResourceGovernorClassifierFunctionRead(ctx, d, m)
}

func resourceResourceGovernorClassifierFunctionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn := m.(*sql.DB)
	name := d.Id()
	_, err := conn.ExecContext(ctx, fmt.Sprintf("DROP FUNCTION [%s]", name))
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
