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
				Description:  descriptions["name"],
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
	apiVersion := "cloud.streamnative.io/v1alpha1"
	kind := "PulsarInstance"
	instanceType := "standard"
	pulsarInstance := sncloudv1.V1alpha1PulsarInstance{
		ApiVersion: &apiVersion,
		Kind:       &kind,
		Metadata: &sncloudv1.V1ObjectMeta{
			Name:      &name,
			Namespace: &namespace,
		},
		Spec: &sncloudv1.V1alpha1PulsarInstanceSpec{
			AvailabilityMode: availabilityMode,
			Type:             &instanceType,
			PoolRef: &sncloudv1.V1alpha1PoolRef{
				Namespace: poolNamespace,
				Name:      poolName,
			},
		},
	}
	apiClient := getFactoryFromMeta(meta)
	pi, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.
		CreateNamespacedPulsarInstance(ctx, namespace).Body(pulsarInstance).Execute()
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
	return nil
}

func resourcePulsarInstanceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	apiClient := getFactoryFromMeta(meta)
	pulsarInstance, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.
		ReadNamespacedPulsarInstance(ctx, name, namespace).Execute()
	if err != nil {
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
	metadata := pulsarInstance.GetMetadata()
	d.SetId(fmt.Sprintf("%s/%s", metadata.GetNamespace(), metadata.GetName()))
	return nil
}

func resourcePulsarInstanceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_INSTANCE: " +
		"The pulsar instance does not support updates, please recreate it"))
}

func resourcePulsarInstanceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	apiClient := getFactoryFromMeta(meta)
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	_, err := apiClient.CloudStreamnativeIoV1alpha1Api.DeleteNamespacedPulsarInstance(ctx, name, namespace).Execute()
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_PULSAR_INSTANCE: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}
