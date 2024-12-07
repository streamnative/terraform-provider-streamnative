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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	cloudclient "github.com/streamnative/cloud-api-server/pkg/client/clientset_generated/clientset"
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

			old, new := diff.GetChange("default_gateway")
			oldGateway := convertGateway(old)
			newGateway := convertGateway(new)

			if oldGateway.Access != newGateway.Access {
				return fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
					"The cloud environment does not support updating the gateway access, please recreate it")
			}

			if diff.HasChanges("organization") ||
				diff.HasChanges("cloud_connection_name") ||
				diff.HasChanges("region") ||
				diff.HasChanges("network_id") ||
				diff.HasChanges("network_cidr") {
				return fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
					"The cloud environment does not support updates on the attributes: organization, cloud_connection_name, region, network_id, network_cidr. Please recreate it")
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
				Description:  descriptions["environment_type"],
				ValidateFunc: validateCloudEnvironmentType,
			},
			"region": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["region"],
				ValidateFunc: validateNotBlank,
			},
			"zone": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  descriptions["zone"],
				ValidateFunc: validateNotBlank,
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
				MaxItems:    1,
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
			"dns": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: descriptions["dns"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"default_gateway": {
				Type: schema.TypeList,
				//Set this as optional and computed because an empty block will still create a default on the API and in the statefile
				//As per https://github.com/hashicorp/terraform/issues/21278 Setting both allows optionally passing the config
				Optional:    true,
				Computed:    true,
				Description: descriptions["default_gateway"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"access": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: descriptions["default_gateway_access"],
						},
						"private_service": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: descriptions["default_gateway_private_service"],
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"allowed_ids": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: descriptions["default_gateway_allowed_ids"],
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
								},
							},
						},
					},
				},
			},
			"annotations": {
				Type:         schema.TypeMap,
				Description:  descriptions["annotations"],
				Optional:     true,
				Elem:         &schema.Schema{Type: schema.TypeString},
				ValidateFunc: validateAnnotations,
			},
			"wait_for_completion": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: descriptions["wait_for_completion"],
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(120 * time.Minute),
			Delete: schema.DefaultTimeout(120 * time.Minute),
		},
	}
}

func resourceCloudEnvironmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	cloudEnvironmentType := d.Get("environment_type").(string)
	region := d.Get("region").(string)
	zone := d.Get("zone").(string)
	cloudConnectionName := d.Get("cloud_connection_name").(string)
	network := d.Get("network").([]interface{})
	dns := d.Get("dns").([]interface{})
	rawAnnotations := d.Get("annotations").(map[string]interface{})
	waitForCompletion := d.Get("wait_for_completion")

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CLOUD_ENVIRONMENT: %w", err))
	}
	annotations := make(map[string]string)
	if len(rawAnnotations) > 0 {
		annotations = convertToStringMap(rawAnnotations)
	}
	annotations["cloud.streamnative.io/environment-type"] = cloudEnvironmentType

	cloudEnvironment := &cloudv1alpha1.CloudEnvironment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CloudEnvironment",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: cloudv1alpha1.CloudEnvironmentSpec{
			CloudConnectionName: cloudConnectionName,
			Region:              region,
			Network:             &cloudv1alpha1.Network{},
		},
	}
	if zone != "" {
		cloudEnvironment.Spec.Zone = &zone
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

	if cloudEnvironment.Spec.Network.ID != "" {
		cc, err := clientSet.CloudV1alpha1().CloudConnections(namespace).Get(ctx, cloudConnectionName, metav1.GetOptions{})
		if err != nil {
			return diag.FromErr(err)
		}
		if cc.Spec.ConnectionType == cloudv1alpha1.ConnectionTypeAzure {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_ENVIRONMENT: Azure doesn't support specify network id yet. Please use network cidr"))
		}
	}

	expandDns := func() error {
		for _, l := range dns {
			if l == nil {
				continue
			}
			item := l.(map[string]interface{})

			dnsId := item["id"].(string)
			dnsName := item["name"].(string)

			if (dnsId != "" && dnsName == "") || (dnsId == "" && dnsName != "") {
				return fmt.Errorf("ERROR_CREATE_CLOUD_ENVIRONMENT: DNS ID and name must specify together")
			}

			cloudEnvironment.Spec.DNS = &cloudv1alpha1.DNS{
				ID:   dnsId,
				Name: dnsName,
			}
		}
		return nil
	}
	if err := expandDns(); err != nil {
		return diag.FromErr(err)
	}

	cloudEnvironment.Spec.DefaultGateway = convertGateway(d.Get("default_gateway"))

	ce, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Create(ctx, cloudEnvironment, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CLOUD_ENVIRONMENT: %w", err))
	}

	ready := false
	d.SetId(fmt.Sprintf("%s/%s", ce.ObjectMeta.Namespace, ce.ObjectMeta.Name))

	if waitForCompletion == true {
		err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), retryUntilCloudEnvironmentIsProvisioned(ctx, clientSet, namespace, ce.GetObjectMeta().GetName()))
		if err != nil {
			return diag.FromErr(err)
		}
	} else {
		for _, condition := range ce.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
	}

	if ready {
		_ = d.Set("organization", namespace)
		return resourceCloudEnvironmentRead(ctx, d, meta)
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
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_CLOUD_ENVIRONMENT: %w", err))
	}
	cloudEnvironment, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT: %w", err))
	}

	_ = d.Set("region", cloudEnvironment.Spec.Region)
	_ = d.Set("cloud_connection_name", cloudEnvironment.Spec.CloudConnectionName)

	if cloudEnvironment.Spec.Network != nil {
		err = d.Set("network", flattenCloudEnvironmentNetwork(cloudEnvironment.Spec.Network))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT_CONFIG: %w", err))
		}
	}

	if cloudEnvironment.Spec.DefaultGateway != nil {
		_ = d.Set("default_gateway", flattenDefaultGateway(cloudEnvironment.Spec.DefaultGateway))
	}

	d.SetId(fmt.Sprintf("%s/%s", cloudEnvironment.Namespace, cloudEnvironment.Name))
	return nil
}

func resourceCloudEnvironmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	waitForCompletion := d.Get("wait_for_completion").(bool)
	name := strings.Split(d.Id(), "/")[1]

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_UPDATE_CLOUD_ENVIRONMENT: %w", err))
	}

	old, new := d.GetChange("default_gateway")
	oldGateway := convertGateway(old)
	newGateway := convertGateway(new)

	if oldGateway.Access != newGateway.Access {
		return diag.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
			"The cloud environment does not support updating the gateway access, please recreate it")
	}

	if d.HasChanges("organization") ||
		d.HasChanges("cloud_connection_name") ||
		d.HasChanges("region") ||
		d.HasChanges("network_id") ||
		d.HasChanges("network_cidr") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: " +
			"The cloud environment does not support updates on the attributes: organization, cloud_connection_name, region, network_id, network_cidr. Please recreate it"))
	}

	cloudEnvironment, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT: %w", err))
	}

	cloudEnvironment.Spec.DefaultGateway = convertGateway(d.Get("default_gateway"))

	if _, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Update(ctx, cloudEnvironment, metav1.UpdateOptions{
		FieldManager: "terraform-update",
	}); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_CLOUD_ENVIRONMENT: %w", err))
	}

	ready := false

	if waitForCompletion {
		err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), retryUntilCloudEnvironmentIsProvisioned(ctx, clientSet, namespace, cloudEnvironment.GetObjectMeta().GetName()))
		if err != nil {
			return diag.FromErr(err)
		}
	} else {
		for _, condition := range cloudEnvironment.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
	}

	if ready {
		_ = d.Set("organization", namespace)
		return resourceCloudEnvironmentRead(ctx, d, meta)
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

func resourceCloudEnvironmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	namespace := d.Get("organization").(string)
	name := strings.Split(d.Id(), "/")[1]
	waitForCompletion := d.Get("wait_for_completion")

	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_CLOUD_ENVIRONMENT: %w", err))
	}

	err = clientSet.CloudV1alpha1().CloudEnvironments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_CLOUD_ENVIRONMENT: %w", err))
	}

	if waitForCompletion == true {
		err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutDelete), retryUntilCloudEnvironmentIsDeleted(ctx, clientSet, namespace, name))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

// retryUntilCloudEnvironmentIsProvisioned checks if a given CloudEnvironment has finished provisioning
func retryUntilCloudEnvironmentIsProvisioned(ctx context.Context, clientSet *cloudclient.Clientset, ns string, name string) retry.RetryFunc {
	return func() *retry.RetryError {
		ce, err := clientSet.CloudV1alpha1().CloudEnvironments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if statusErr, ok := err.(*apierrors.StatusError); ok && apierrors.IsNotFound(statusErr) {
				return nil
			}
			return retry.NonRetryableError(err)
		}

		for _, condition := range ce.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				return nil
			}
		}

		//Sleep 10 seconds between checks so we don't overload the API
		time.Sleep(time.Second * 10)

		return retry.RetryableError(fmt.Errorf("cloudenvironment: %s/%s is not in complete state", ns, name))
	}
}

// retryUntilCloudEnvironmentIsDeleted checks if a given CloudEnvironment has finished deleting
func retryUntilCloudEnvironmentIsDeleted(ctx context.Context, clientSet *cloudclient.Clientset, ns string, name string) retry.RetryFunc {
	return func() *retry.RetryError {
		//Sleep 10 seconds between checks so we don't overload the API
		time.Sleep(time.Second * 10)

		_, err := clientSet.CloudV1alpha1().CloudEnvironments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil
			} else {
				return retry.RetryableError(fmt.Errorf("cloudenvironment: %s/%s is not in complete state", ns, name))
			}
		}

		return retry.RetryableError(fmt.Errorf("cloudenvironment: %s/%s is not in complete state", ns, name))
	}
}
