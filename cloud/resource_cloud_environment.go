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
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("organization") ||
				diff.HasChanges("cloud_connection_name") ||
				diff.HasChanges("region") {
				return fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
					"The cloud environment does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationInstance := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationInstance[0])
				_ = d.Set("name", organizationInstance[1])
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
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["name"],
				ValidateFunc: validateNotBlank,
			},
			"region": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["region"],
				ValidateFunc: validateNotBlank,
			},
			"cloud_connection_name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["cloud_connection_name"],
				ValidateFunc: validateNotBlank,
			},
		},
	}
}

func resourceCloudEnvironmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	region := d.Get("region").(string)
	cloudConnectionName := d.Get("cloud_connection_name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CLOUD_ENVIRONMENT: %w", err))
	}
	//TODO grab values from tf definition
	networkRef := &cloudv1alpha1.Network{}
	CloudEnvironment := &cloudv1alpha1.CloudEnvironment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CloudEnvironment",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cloudv1alpha1.CloudEnvironmentSpec{
			CloudConnectionName: cloudConnectionName,
			Region:              region,
			Network:             networkRef,
		},
	}
	pi, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Create(ctx, CloudEnvironment, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_ENVIRONMENT: %w", err))
	}
	if pi.Status.Conditions != nil {
		ready := false
		for _, condition := range pi.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if ready {
			_ = d.Set("organization", namespace)
			_ = d.Set("name", name)
			return resourceCloudEnvironmentRead(ctx, d, meta)
		}
	}
	err = retry.RetryContext(ctx, 3*time.Minute, func() *retry.RetryError {
		dia := resourceCloudEnvironmentRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_CLOUD_ENVIRONMENT: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_READ_CLOUD_ENVIRONMENT"))
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
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	CloudEnvironment, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT: %w", err))
	}
	_ = d.Set("ready", "False")
	if CloudEnvironment.Status.Conditions != nil {
		for _, condition := range CloudEnvironment.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				_ = d.Set("ready", "True")
			}
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", CloudEnvironment.Namespace, CloudEnvironment.Name))
	return nil
}

func resourceCloudEnvironmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
		"The cloud environment does not support updates, please recreate it"))
}

func resourceCloudEnvironmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_CLOUD_ENVIRONMENT: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	err = clientSet.CloudV1alpha1().CloudEnvironments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_CLOUD_ENVIRONMENT: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}
