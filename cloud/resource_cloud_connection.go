// Copyright 2024 StreamNative, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourceCloudConnection() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCloudConnectionCreate,
		ReadContext:   resourceCloudConnectionRead,
		UpdateContext: resourceCloudConnectionUpdate,
		DeleteContext: resourceCloudConnectionDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("organization") ||
				diff.HasChanges("name") ||
				diff.HasChanges("type") {
				return fmt.Errorf("ERROR_UPDATE_CLOUD_CONNECTION: " +
					"The cloud connection does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationInstance := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationInstance[0])
				_ = d.Set("name", organizationInstance[1])
				err := resourceCloudConnectionRead(ctx, d, meta)
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
				Required:     true,
				Description:  descriptions["cloud_connection_name"],
				ValidateFunc: validateNotBlank,
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["type"],
				ValidateFunc: validateNotBlank,
			},
			"aws": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: descriptions["aws"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"account_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"gcp": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: descriptions["gcp"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"project_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"azure": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: descriptions["azure"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subscription_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"tenant_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"client_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"support_client_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func resourceCloudConnectionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	connectionType := d.Get("type").(string)
	aws := d.Get("aws").([]interface{})
	gcp := d.Get("gcp").([]interface{})
	azure := d.Get("azure").([]interface{})
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CLOUD_CONNECTION: %w", err))
	}

	cloudConnection := &cloudv1alpha1.CloudConnection{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CloudConnection",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},

		Spec: cloudv1alpha1.CloudConnectionSpec{
			ConnectionType: cloudv1alpha1.ConnectionType(connectionType),
			AWS:            nil,
			GCP:            nil,
			Azure:          nil,
		},
	}

	if len(aws) > 0 {
		cloudConnection.Spec.AWS = &cloudv1alpha1.AWSCloudConnection{}
		for _, awsItem := range aws {
			awsMapItem := awsItem.(map[string]interface{})
			if awsMapItem["account_id"] != nil {
				accountId := awsMapItem["account_id"].(string)
				cloudConnection.Spec.AWS.AccountId = accountId
			}
		}
	}

	if len(gcp) > 0 {
		cloudConnection.Spec.GCP = &cloudv1alpha1.GCPCloudConnection{}
		for _, gcpItem := range gcp {
			gcpItemMap := gcpItem.(map[string]interface{})
			if gcpItemMap["project_id"] != nil {
				projectId := gcpItemMap["project_id"].(string)
				cloudConnection.Spec.GCP.ProjectId = projectId
			}
		}
	}

	if len(azure) > 0 {
		cloudConnection.Spec.Azure = &cloudv1alpha1.AzureConnection{}
		for _, azureItem := range azure {
			azureItemMap := azureItem.(map[string]interface{})
			if azureItemMap["subscription_id"] != nil {
				cloudConnection.Spec.Azure.SubscriptionId = azureItemMap["subscription_id"].(string)
			}
			if azureItemMap["tenant_id"] != nil {
				cloudConnection.Spec.Azure.TenantId = azureItemMap["tenant_id"].(string)
			}
			if azureItemMap["client_id"] != nil {
				cloudConnection.Spec.Azure.ClientId = azureItemMap["client_id"].(string)
			}
			if azureItemMap["support_client_id"] != nil {
				cloudConnection.Spec.Azure.SupportClientId = azureItemMap["support_client_id"].(string)
			}
		}
	}

	if cloudConnection.Spec.AWS == nil && cloudConnection.Spec.GCP == nil && cloudConnection.Spec.Azure == nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_CONNECTION: " + "One of aws.account_id, gcp.project_id or azure block must be set"))
	}

	cc, err := clientSet.CloudV1alpha1().CloudConnections(namespace).Create(ctx, cloudConnection, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_CONNECTION: %w", err))
	}
	if cc.Status.Conditions != nil {
		ready := false
		for _, condition := range cc.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if ready {
			_ = d.Set("organization", namespace)
			_ = d.Set("name", name)
			return resourceCloudConnectionRead(ctx, d, meta)
		}
	}
	err = retry.RetryContext(ctx, 3*time.Minute, func() *retry.RetryError {
		dia := resourceCloudConnectionRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_CLOUD_CONNECTION: %s", dia[0].Summary))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_CLOUD_CONNECTION: %w", err))
	}
	return nil
}

func resourceCloudConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	cloudConnection, err := clientSet.CloudV1alpha1().CloudConnections(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION: %w", err))
	}

	if cloudConnection.Spec.AWS != nil {
		err = d.Set("aws", flattenCloudConnectionAws(cloudConnection.Spec.AWS))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION_AWS: %w", err))
		}
	}

	if cloudConnection.Spec.GCP != nil {
		err = d.Set("gcp", flattenCloudConnectionGCP(cloudConnection.Spec.GCP))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION_GCP: %w", err))
		}
	}

	if cloudConnection.Spec.Azure != nil {
		err = d.Set("azure", flattenCloudConnectionAzure(cloudConnection.Spec.Azure))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION_AZURE: %w", err))
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", cloudConnection.Namespace, cloudConnection.Name))
	return nil
}

func resourceCloudConnectionUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_CLOUD_CONNECTION: " +
		"The cloud connection does not support updates, please recreate it"))
}

func resourceCloudConnectionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_CLOUD_CONNECTION: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	err = clientSet.CloudV1alpha1().CloudConnections(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_CLOUD_CONNECTION: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}
