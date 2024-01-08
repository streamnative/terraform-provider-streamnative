package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"strings"
)

func dataSourceServiceAccount() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourceServiceAccountRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(
				ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationServiceAccount := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationServiceAccount[0])
				_ = d.Set("name", organizationServiceAccount[1])
				err := resourceServiceAccountRead(ctx, d, meta)
				if err.HasError() {
					return nil, fmt.Errorf("import %q: %s", d.Id(), err[0].Summary)
				}
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["organization"],
				ValidateFunc: validateNotBlank,
			},
			"name": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  descriptions["name"],
				ValidateFunc: validateNotBlank,
			},
			"admin": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: descriptions["admin"],
				Computed:    true,
			},
			"private_key_data": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["private_key_data"],
				Computed:    true,
			},
		},
	}
}

func DataSourceServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	apiClient := getFactoryFromMeta(meta)
	serviceAccount, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.
		ReadNamespacedServiceAccount(ctx, name, namespace).Execute()
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT: %w", err))
	}
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = *serviceAccount.Status.PrivateKeyData
	}
	_ = d.Set("private_key_data", privateKeyData)
	metadata := serviceAccount.GetMetadata()
	if annotation, ok := metadata.GetAnnotations()[ServiceAccountAdminAnnotation]; ok {
		_ = d.Set("admin", annotation == "admin")
	}
	d.SetId(fmt.Sprintf("%s/%s", metadata.GetNamespace(), metadata.GetName()))

	return nil
}
