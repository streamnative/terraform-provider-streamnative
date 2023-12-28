package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	serviceAccount, err := clientSet.CloudV1alpha1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT: %w", err))
	}
	_ = d.Set("name", serviceAccount.Name)
	_ = d.Set("organization", serviceAccount.Namespace)
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = serviceAccount.Status.PrivateKeyData
	}
	_ = d.Set("private_key_data", privateKeyData)
	if serviceAccount.Annotations != nil && serviceAccount.Annotations[ServiceAccountAdminAnnotation] == "admin" {
		_ = d.Set("admin", true)
	} else {
		_ = d.Set("admin", false)
	}
	d.SetId(fmt.Sprintf("%s/%s", serviceAccount.Namespace, serviceAccount.Name))

	return nil
}
