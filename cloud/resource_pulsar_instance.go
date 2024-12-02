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

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourcePulsarInstance() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePulsarInstanceCreate,
		ReadContext:   resourcePulsarInstanceRead,
		UpdateContext: resourcePulsarInstanceUpdate,
		DeleteContext: resourcePulsarInstanceDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("organization") ||
				diff.HasChanges("availability_mode") ||
				diff.HasChanges("pool_name") ||
				diff.HasChanges("pool_namespace") {
				return fmt.Errorf("ERROR_UPDATE_PULSAR_INSTANCE: " +
					"The pulsar instance does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationInstance := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationInstance[0])
				_ = d.Set("name", organizationInstance[1])
				err := resourcePulsarInstanceRead(ctx, d, meta)
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
				Description:  descriptions["instance_name"],
				ValidateFunc: validateNotBlank,
			},
			"availability_mode": {
				Type:     schema.TypeString,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: descriptions["availability-mode"],
			},
			"pool_name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["pool_name"],
				ValidateFunc: validateNotBlank,
			},
			"pool_namespace": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["pool_namespace"],
				ValidateFunc: validateNotBlank,
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  descriptions["instance_type"],
				ValidateFunc: validateInstanceType,
			},
			"engine": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  descriptions["instance_engine"],
				ValidateFunc: validateEngine,
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["instance_ready"],
			},
		},
	}
}

func resourcePulsarInstanceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	availabilityMode := d.Get("availability_mode").(string)
	poolName := d.Get("pool_name").(string)
	poolNamespace := d.Get("pool_namespace").(string)
	instanceType := d.Get("type").(string)
	instanceEngine := d.Get("engine").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_PULSAR_INSTANCE: %w", err))
	}
	poolRef := &cloudv1alpha1.PoolRef{
		Namespace: poolNamespace,
		Name:      poolName,
	}
	poolOption, err := clientSet.CloudV1alpha1().
		PoolOptions(namespace).
		Get(ctx, fmt.Sprintf("%s-%s", poolNamespace, poolName), metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_GET_POOL_OPTION: %w", err))
	}
	if instanceType == "" {
		if poolOption.Spec.DeploymentType == cloudv1alpha1.PoolDeploymentTypeHosted {
			instanceType = "serverless"
		}
		if poolOption.Spec.DeploymentType == cloudv1alpha1.PoolDeploymentTypeManaged {
			instanceType = "byoc"
		}
		if poolOption.Spec.DeploymentType == cloudv1alpha1.PoolDeploymentTypeManagedPro {
			instanceType = "byoc-pro"
		}
	}
	pulsarInstance := &cloudv1alpha1.PulsarInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PulsarInstance",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cloudv1alpha1.PulsarInstanceSpec{
			AvailabilityMode: cloudv1alpha1.InstanceAvailabilityMode(availabilityMode),
			Type:             cloudv1alpha1.PulsarInstanceType(instanceType),
			PoolRef:          poolRef,
		},
	}
	if instanceEngine == UrsaEngineValue {
		pulsarInstance.Annotations = map[string]string{
			UrsaEngineAnnotation: UrsaEngineValue,
		}
	}
	pi, err := clientSet.CloudV1alpha1().PulsarInstances(namespace).Create(ctx, pulsarInstance, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_INSTANCE: %w", err))
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
			return resourcePulsarInstanceRead(ctx, d, meta)
		}
	}
	err = retry.RetryContext(ctx, 3*time.Minute, func() *retry.RetryError {
		dia := resourcePulsarInstanceRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_PULSAR_INSTANCE: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_READ_PULSAR_INSTANCE"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_PULSAR_INSTANCE: %w", err))
	}
	return nil
}

func resourcePulsarInstanceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	pulsarInstance, err := clientSet.CloudV1alpha1().PulsarInstances(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_INSTANCE: %w", err))
	}
	_ = d.Set("ready", "False")
	if pulsarInstance.Status.Conditions != nil {
		for _, condition := range pulsarInstance.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				_ = d.Set("ready", "True")
			}
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", pulsarInstance.Namespace, pulsarInstance.Name))
	return nil
}

func resourcePulsarInstanceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_INSTANCE: " +
		"The pulsar instance does not support updates, please recreate it"))
}

func resourcePulsarInstanceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_PULSAR_INSTANCE: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	err = clientSet.CloudV1alpha1().PulsarInstances(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_PULSAR_INSTANCE: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}
