package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

func resourceServiceAccount() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServiceAccountCreate,
		ReadContext:   resourceServiceAccountRead,
		UpdateContext: resourceServiceAccountUpdate,
		DeleteContext: resourceServiceAccountDelete,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
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
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["organization"],
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["name"],
			},
			"admin": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: descriptions["admin"],
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

func resourceServiceAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	admin := d.Get("admin").(bool)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_SERVICE_ACCOUNT: %w", err))
	}
	sa := &v1alpha1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if admin {
		sa.ObjectMeta.Annotations = map[string]string{
			"annotations.cloud.streamnative.io/service-account-role": "admin",
		}
	}
	serviceAccount, err := clientSet.CloudV1alpha1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_SERVICE_ACCOUNT: %w", err))
	}
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = serviceAccount.Status.PrivateKeyData
		_ = d.Set("name", serviceAccount.Name)
		_ = d.Set("organization", serviceAccount.Namespace)
		_ = d.Set("private_key_data", privateKeyData)
		d.SetId(fmt.Sprintf("%s/%s", serviceAccount.Namespace, serviceAccount.Name))
	}
	// Don't retry too frequently to avoid affecting the api-server.
	err = retry.RetryContext(ctx, 5*time.Second, func() *retry.RetryError {
		dia := resourceServiceAccountRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_CREATE_SERVICE_ACCOUNT: %s", dia[0].Summary))
		}
		pkd := d.Get("private_key_data")
		if pkd == "" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_CREATE_SERVICE_ACCOUNT"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_SERVICE_ACCOUNT: %w", err))
	}
	return nil
}

func resourceServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
	d.SetId(fmt.Sprintf("%s/%s", serviceAccount.Namespace, serviceAccount.Name))

	return nil
}

func resourceServiceAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_SERVICE_ACCOUNT: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	err = clientSet.CloudV1alpha1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_SERVICE_ACCOUNT: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}

func resourceServiceAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	err := resourceServiceAccountRead(ctx, d, meta)
	if err.HasError() {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT: %s", err[0].Summary))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	_ = d.Set("name", name)
	_ = d.Set("organization", namespace)
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return resourceServiceAccountRead(ctx, d, meta)
}
