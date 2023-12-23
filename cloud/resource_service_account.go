package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourceServiceAccount() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServiceAccountCreate,
		ReadContext:   resourceServiceAccountRead,
		UpdateContext: resourceServiceAccountUpdate,
		DeleteContext: resourceServiceAccountDelete,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				_ = d.Set("service_account", d.Id())
				err := resourceServiceAccountRead(ctx, d, meta)
				if err.HasError() {
					return nil, fmt.Errorf("import %q: %s", d.Id(), err[0].Summary)
				}
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["organization"],
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["name"],
			},
			"private_key_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["private_key_type"],
			},
			"private_key_data": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["private_key_data"],
			},
		},
	}
}

func resourceServiceAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	//	Todo: create service account or return error
	return resourceServiceAccountRead(ctx, d, meta)
}

func resourceServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	privateKeyData := d.Get("private_key_data").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(err)
	}
	serviceAccount, err := clientSet.CloudV1alpha1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(err)
	}
	_ = d.Set("name", serviceAccount.Name)
	_ = d.Set("organization", serviceAccount.Namespace)
	fmt.Println(serviceAccount)
	//var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = serviceAccount.Status.PrivateKeyData
	}
	_ = d.Set("private_key_data", privateKeyData)
	_ = d.Set("private_key_type", serviceAccount.Status.PrivateKeyType)
	d.SetId(serviceAccount.Name)

	return nil
}

func resourceServiceAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceServiceAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}
