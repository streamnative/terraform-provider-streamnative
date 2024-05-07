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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	cloudclient "github.com/streamnative/cloud-api-server/pkg/client/clientset_generated/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourcePulsarGateway() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePulsarGatewayCreate,
		ReadContext:   resourcePulsarGatewayRead,
		UpdateContext: resourcePulsarGatewayUpdate,
		DeleteContext: resourcePulsarGatewayDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("access") {
				return fmt.Errorf("ERROR_UPDATE_PULSAR_GATEWAY: " +
					"The pulsar gateway does not support updates name and access, please recreate it")
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
			"access": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["access"],
				ValidateFunc: validation.StringInSlice([]string{"public", "private"}, false),
			},
			"poolmember_name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["poolmember_name"],
				ValidateFunc: validateNotBlank,
			},
			"poolmember_namespace": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["poolmember_namespace"],
				ValidateFunc: validateNotBlank,
			},
			"private_service": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: descriptions["private_service"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allowed_ids": {
							Type:         schema.TypeList,
							Optional:     true,
							Description:  descriptions["allowed_ids"],
							ValidateFunc: validation.ListOfUniqueStrings,
						},
					},
				},
			},
			"wait_for_completion": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: descriptions["wait_for_completion"],
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},
	}
}

func resourcePulsarGatewayCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	access := d.Get("access").(string)
	poolMemberName := d.Get("poolmember_name").(string)
	poolMemberNamespace := d.Get("poolmember_namespace").(string)
	waitForCompletion := d.Get("wait_for_completion").(bool)

	pulsarGateway := &cloudv1alpha1.PulsarGateway{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PulsarGateway",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cloudv1alpha1.PulsarGatewaySpec{
			Access: cloudv1alpha1.AccessType(access),
			PoolMemberRef: cloudv1alpha1.PoolMemberReference{
				Namespace: poolMemberNamespace,
				Name:      poolMemberName,
			},
		},
	}
	if access == string(cloud.PrivateAccess) {
		privateService := d.Get("private_service").(map[string]interface{})
		allowedIds := privateService["allowed_ids"].([]string)
		pulsarGateway.Spec.PrivateService = &cloudv1alpha1.PrivateService{
			AllowedIds: allowedIds,
		}
	}

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_PULSAR_GATEWAY: %w", err))
	}

	pg, err := clientSet.CloudV1alpha1().PulsarGateways(namespace).Create(ctx, pulsarGateway, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_GATEWAY: %w", err))
	}
	ready := false
	d.SetId(fmt.Sprintf("%s/%s", pg.ObjectMeta.Namespace, pg.ObjectMeta.Name))
	if waitForCompletion {
		err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), retryUntilPulsarGatewayIsReady(ctx, clientSet, namespace, pg.GetObjectMeta().GetName()))
		if err != nil {
			return diag.FromErr(err)
		}
	} else {
		for _, condition := range pg.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
	}

	if ready {
		d.SetId(fmt.Sprintf("%s/%s", pg.ObjectMeta.Namespace, pg.ObjectMeta.Name))
		return resourcePulsarGatewayRead(ctx, d, meta)
	}
	err = retry.RetryContext(ctx, 3*time.Minute, func() *retry.RetryError {
		dia := resourcePulsarGatewayRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_PULSAR_GATEWAY: %s", dia[0].Summary))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_PULSAR_GATEWAY: %w", err))
	}
	return nil
}

func resourcePulsarGatewayRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := strings.Split(d.Id(), "/")[1]

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_PULSAR_GATEWAY: %w", err))
	}

	pg, err := clientSet.CloudV1alpha1().PulsarGateways(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_GATEWAY: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", pg.Namespace, pg.Name))
	return nil
}

func resourcePulsarGatewayUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := strings.Split(d.Id(), "/")[1]
	access := d.Get("access").(string)
	waitForCompletion := d.Get("wait_for_completion").(bool)

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_UPDATE_PULSAR_GATEWAY: %w", err))
	}
	pg, err := clientSet.CloudV1alpha1().PulsarGateways(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_GATEWAY: %w", err))
	}
	if access != string(cloud.PrivateAccess) || !d.HasChange("private_service") {
		return nil
	}

	privateService := d.Get("private_service").(map[string]interface{})
	allowedIds := privateService["allowed_ids"].([]string)
	pg.Spec.PrivateService = &cloudv1alpha1.PrivateService{
		AllowedIds: allowedIds,
	}
	if _, err := clientSet.CloudV1alpha1().PulsarGateways(namespace).Update(ctx, pg, metav1.UpdateOptions{
		FieldManager: "terraform-update",
	}); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_GATEWAY: %w", err))
	}

	if waitForCompletion {
		if err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutUpdate), retryUntilPulsarGatewayIsUpdated(ctx, clientSet, namespace, name)); err != nil {
			return diag.FromErr(err)
		}
		if err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutUpdate), retryUntilPulsarGatewayIsReady(ctx, clientSet, namespace, name)); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func resourcePulsarGatewayDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_PULSAR_GATEWAY: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := strings.Split(d.Id(), "/")[1]
	waitForCompletion := d.Get("wait_for_completion").(bool)

	err = clientSet.CloudV1alpha1().PulsarGateways(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("DELETE_PULSAR_GATEWAY: %w", err))
	}

	if waitForCompletion {
		err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutDelete), retryUntilPulsarGatewayIsDeleted(ctx, clientSet, namespace, name))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func retryUntilPulsarGatewayIsReady(ctx context.Context, clientSet *cloudclient.Clientset, ns string, name string) retry.RetryFunc {
	return func() *retry.RetryError {
		pg, err := clientSet.CloudV1alpha1().PulsarGateways(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if statusErr, ok := err.(*apierrors.StatusError); ok && apierrors.IsNotFound(statusErr) {
				return nil
			}
			return retry.NonRetryableError(err)
		}

		for _, condition := range pg.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				return nil
			}
		}

		//Sleep 10 seconds between checks so we don't overload the API
		time.Sleep(time.Second * 10)

		return retry.RetryableError(fmt.Errorf("pulsargateway: %s/%s is not in complete state", ns, name))
	}
}

func retryUntilPulsarGatewayIsUpdated(ctx context.Context, clientSet *cloudclient.Clientset, ns string, name string) retry.RetryFunc {
	return func() *retry.RetryError {
		pg, err := clientSet.CloudV1alpha1().PulsarGateways(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if statusErr, ok := err.(*apierrors.StatusError); ok && apierrors.IsNotFound(statusErr) {
				return nil
			}
			return retry.NonRetryableError(err)
		}
		if pg.Status.ObservedGeneration == pg.Generation {
			return nil
		}

		//Sleep 10 seconds between checks so we don't overload the API
		time.Sleep(time.Second * 10)

		return retry.RetryableError(fmt.Errorf("pulsargateway: %s/%s is not in complete state", ns, name))
	}
}

func retryUntilPulsarGatewayIsDeleted(ctx context.Context, clientSet *cloudclient.Clientset, ns string, name string) retry.RetryFunc {
	return func() *retry.RetryError {
		_, err := clientSet.CloudV1alpha1().PulsarGateways(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return retry.RetryableError(fmt.Errorf("pulsargateway: %s/%s is not deleted", ns, name))
		}

		//Sleep 10 seconds between checks so we don't overload the API
		time.Sleep(time.Second * 10)

		return retry.RetryableError(fmt.Errorf("pulsargateway: %s/%s is not deleted", ns, name))
	}
}
