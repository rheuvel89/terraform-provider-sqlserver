package sqlserver

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
)

type CredentialConnector interface {
	EnsureCredentialsAvailable(ctx context.Context) error
	GetCredential(ctx context.Context, name string) (*model.Credential, error)
	CreateCredential(ctx context.Context, name, identity string) error
	DeleteCredential(ctx context.Context, name string) error
}

// resourceCredential is a preview: it only supports managed identity credentials, which need no secret, for backup to
// and restore from Azure Blob Storage. Other identities need secrets and aren't supported yet.
func resourceCredential() *schema.Resource {
	return &schema.Resource{
		Description:   "Preview: manages a server-level credential. Only managed identity credentials are supported for now.",
		CreateContext: resourceCredentialCreate,
		ReadContext:   resourceCredentialRead,
		DeleteContext: resourceCredentialDelete,
		Schema: map[string]*schema.Schema{
			credentialNameProp: {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				Description:      "The name of the credential. For backup to URL, the container URL without a trailing slash.",
				ValidateDiagFunc: validateCredentialName,
			},
			credentialIdentityProp: {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				Description:      "The identity of the credential. Only \"Managed Identity\" is supported for now.",
				ValidateDiagFunc: validateCredentialIdentity,
			},
			credentialIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the credential in sys.credentials.",
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: defaultTimeout,
			Read:    defaultTimeout,
			Create:  defaultTimeout,
			Delete:  defaultTimeout,
		},
	}
}

// validateCredentialName checks the length of the name (sysname) and warns about a URL with a trailing slash, which
// BACKUP TO URL won't match to the container URL.
func validateCredentialName(i interface{}, path cty.Path) diag.Diagnostics {
	name := i.(string)
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 128 {
		return diag.Diagnostics{{Severity: diag.Error, Summary: "name must be between 1 and 128 characters", AttributePath: path}}
	}
	lower := strings.ToLower(name)
	if (strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) && strings.HasSuffix(name, "/") {
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  "credential name ends with a slash",
			Detail: fmt.Sprintf("BACKUP TO URL only uses a credential whose name matches the container URL exactly, "+
				"without a trailing slash: %q.", strings.TrimRight(name, "/")),
			AttributePath: path,
		}}
	}
	return nil
}

func validateCredentialIdentity(i interface{}, path cty.Path) diag.Diagnostics {
	if !strings.EqualFold(i.(string), credentialManagedIdentity) {
		return diag.Diagnostics{{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("identity %q isn't supported yet", i.(string)),
			Detail: "sqlserver_credential is a preview and only supports managed identity credentials " +
				"(identity = \"Managed Identity\"). Credentials with a secret, such as SAS tokens, aren't supported yet.",
			AttributePath: path,
		}}
	}
	return nil
}

func resourceCredentialCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "credential", "create")
	name := data.Get(credentialNameProp).(string)
	logger.Debug().Msgf("Create credential [%s]", name)

	connector, err := getCredentialConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = connector.EnsureCredentialsAvailable(ctx); err != nil {
		return diag.FromErr(err)
	}

	if err = connector.CreateCredential(ctx, name, data.Get(credentialIdentityProp).(string)); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create credential [%s]", name))
	}

	logger.Info().Msgf("created credential [%s]", name)

	data.SetId(getCredentialID(meta, name))

	return resourceCredentialRead(ctx, data, meta)
}

func resourceCredentialRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "credential", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	connector, err := getCredentialConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	name := data.Get(credentialNameProp).(string)
	cred, err := connector.GetCredential(ctx, name)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read credential [%s]", name))
	}
	if cred == nil {
		logger.Info().Msgf("No credential found for [%s]", name)
		data.SetId("")
		return nil
	}

	if err = data.Set(credentialIdentityProp, cred.Identity); err != nil {
		return diag.FromErr(err)
	}
	if err = data.Set(credentialIdProp, cred.CredentialID); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceCredentialDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "credential", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	connector, err := getCredentialConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	name := data.Get(credentialNameProp).(string)
	if err = connector.DeleteCredential(ctx, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete credential [%s]", name))
	}

	logger.Info().Msgf("deleted credential [%s]", name)

	data.SetId("")

	return nil
}

func getCredentialID(meta interface{}, name string) string {
	provider := meta.(sqlserverProvider)
	return fmt.Sprintf("sqlserver://%s:%s/credential/%s", provider.host, provider.port, url.PathEscape(name))
}

func getCredentialConnector(meta interface{}, data *schema.ResourceData) (CredentialConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(CredentialConnector), nil
}
