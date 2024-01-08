package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sncloudv1 "github.com/tuteng/sncloud-go-sdk"
	"strings"
	"time"
)

func resourceServiceAccount() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServiceAccountCreate,
		ReadContext:   resourceServiceAccountRead,
		UpdateContext: resourceServiceAccountUpdate,
		DeleteContext: resourceServiceAccountDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("organization") ||
				diff.HasChanges("admin") {
				return fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT: " +
					"The service account does not support updates, please recreate it")
			}
			return nil
		},
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
	apiVersion := "cloud.streamnative.io/v1alpha1"
	kind := "ServiceAccount"
	sa := sncloudv1.V1alpha1ServiceAccount{
		ApiVersion: &apiVersion,
		Kind:       &kind,
		Metadata: &sncloudv1.V1ObjectMeta{
			Name:      &name,
			Namespace: &namespace,
		},
	}
	if admin {
		sa.Metadata.Annotations = &map[string]string{
			ServiceAccountAdminAnnotation: "admin",
		}
	}
	apiClient := getFactoryFromMeta(meta)
	serviceAccount, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.
		CreateNamespacedServiceAccount(ctx, namespace).Body(sa).Execute()
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_SERVICE_ACCOUNT: %w", err))
	}
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		metadata := serviceAccount.GetMetadata()
		privateKeyData = *serviceAccount.Status.PrivateKeyData
		_ = d.Set("name", metadata.GetName())
		_ = d.Set("organization", metadata.GetNamespace())
		_ = d.Set("private_key_data", privateKeyData)
		d.SetId(fmt.Sprintf("%s/%s", metadata.GetNamespace(), metadata.GetName()))
	}
	// Don't retry too frequently to avoid affecting the api-server.
	err = retry.RetryContext(ctx, 5*time.Minute, func() *retry.RetryError {
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
	apiClient := getFactoryFromMeta(meta)
	saRequest := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedServiceAccount(ctx, name, namespace)
	serviceAccount, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedServiceAccountExecute(saRequest)
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT: %w", err))
	}
	metadata := serviceAccount.GetMetadata()
	_ = d.Set("name", metadata.GetName())
	_ = d.Set("organization", metadata.GetNamespace())
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = *serviceAccount.Status.PrivateKeyData
	}
	_ = d.Set("private_key_data", privateKeyData)
	d.SetId(fmt.Sprintf("%s/%s", metadata.GetNamespace(), metadata.GetName()))

	return nil
}

func resourceServiceAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	apiClient := getFactoryFromMeta(meta)
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	saRequest := apiClient.CloudStreamnativeIoV1alpha1Api.DeleteNamespacedServiceAccount(ctx, name, namespace)
	_, err := apiClient.CloudStreamnativeIoV1alpha1Api.DeleteNamespacedServiceAccountExecute(saRequest)
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_SERVICE_ACCOUNT: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}

func resourceServiceAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT: " +
		"The service account does not support updates, please recreate it"))
}
