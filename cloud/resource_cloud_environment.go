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

func resourceCloudEnvironment() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCloudEnvironmentCreate,
		ReadContext:   resourceCloudEnvironmentRead,
		UpdateContext: resourceCloudEnvironmentUpdate,
		DeleteContext: resourceCloudEnvironmentDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			if oldOrg.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChanges("organization") ||
				diff.HasChanges("cloud_connection_name") ||
				diff.HasChanges("region") ||
				diff.HasChanges("network_id") ||
				diff.HasChanges("network_cidr") {
				return fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
					"The cloud environment does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationInstance := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationInstance[0])
				err := resourceCloudEnvironmentRead(ctx, d, meta)
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
			"environment_type": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["cloud_environment_type"],
				ValidateFunc: validateCloudEnvionmentType,
			},
			"region": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["region"],
				ValidateFunc: validateRegion,
			},
			"cloud_connection_name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["cloud_connection_name"],
				ValidateFunc: validateNotBlank,
			},
			"network": {
				Type:        schema.TypeList,
				Required:    true,
				Description: descriptions["network"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"cidr": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validateCidrRange,
						},
					},
				},
			},
		},
	}
}

func resourceCloudEnvironmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	cloudEnvironmentType := d.Get("environment_type").(string)
	region := d.Get("region").(string)
	cloudConnectionName := d.Get("cloud_connection_name").(string)
	network := d.Get("network").([]interface{})
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CLOUD_ENVIRONMENT: %w", err))
	}

	cloudEnvironment := &cloudv1alpha1.CloudEnvironment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CloudEnvironment",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Annotations: map[string]string{
				"cloud.streamnative.io/environment-type": cloudEnvironmentType,
			},
		},
		Spec: cloudv1alpha1.CloudEnvironmentSpec{
			CloudConnectionName: cloudConnectionName,
			Region:              region,
			Network:             &cloudv1alpha1.Network{},
		},
	}

	if len(network) > 0 {
		for _, networkItem := range network {
			networkItemMap := networkItem.(map[string]interface{})
			if networkItemMap["id"] != nil {
				networkId := networkItemMap["id"].(string)
				cloudEnvironment.Spec.Network.ID = networkId
			}
			if networkItemMap["cidr"] != nil {
				networkCidr := networkItemMap["cidr"].(string)
				cloudEnvironment.Spec.Network.CIDR = networkCidr
			}
		}
	}

	if cloudEnvironment.Spec.Network.ID == "" && cloudEnvironment.Spec.Network.CIDR == "" {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_ENVIRONMENT: " + "One of network.id or network.cidr must be set"))
	}

	ce, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Create(ctx, cloudEnvironment, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_ENVIRONMENT: %w", err))
	}

	d.SetId(fmt.Sprintf("%s/%s", ce.ObjectMeta.Namespace, ce.ObjectMeta.Name))

	if ce.Status.Conditions != nil {
		ready := false
		for _, condition := range ce.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if ready {
			_ = d.Set("organization", namespace)
			return resourceCloudEnvironmentRead(ctx, d, meta)
		}
	}
	err = retry.RetryContext(ctx, 3*time.Minute, func() *retry.RetryError {
		dia := resourceCloudEnvironmentRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_CLOUD_ENVIRONMENT: %s", dia[0].Summary))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_CLOUD_ENVIRONMENT: %w", err))
	}
	return nil
}

func resourceCloudEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := strings.Split(d.Id(), "/")[1]

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	cloudEnvironment, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT: %w", err))
	}

	if cloudEnvironment.Spec.Network != nil {
		err = d.Set("network", flattenCloudEnvironmentNetwork(cloudEnvironment.Spec.Network))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT_CONFIG: %w", err))
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", cloudEnvironment.Namespace, cloudEnvironment.Name))
	return nil
}

func resourceCloudEnvironmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
		"The cloud environment does not support updates, please recreate it"))
}

func resourceCloudEnvironmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	name := strings.Split(d.Id(), "/")[1]
	namespace := d.Get("organization").(string)

	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_CLOUD_ENVIRONMENT: %w", err))
	}

	err = clientSet.CloudV1alpha1().CloudEnvironments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_CLOUD_ENVIRONMENT: %w", err))
	}
	return nil
}
