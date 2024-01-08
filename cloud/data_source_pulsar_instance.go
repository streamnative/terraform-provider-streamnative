package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"strings"
)

func dataSourcePulsarInstance() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePulsarInstanceRead,
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
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: descriptions["availability-mode"],
			},
			"pool_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pool_name"],
			},
			"pool_namespace": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pool_namespace"],
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["instance_ready"],
			},
		},
	}
}

func dataSourcePulsarInstanceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	apiClient := getFactoryFromMeta(meta)
	pulsarInstance, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedPulsarInstance(ctx, name, namespace).Execute()
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
	if pulsarInstance.Spec.PoolRef != nil {
		_ = d.Set("pool_name", pulsarInstance.Spec.PoolRef.Name)
		_ = d.Set("pool_namespace", pulsarInstance.Spec.PoolRef.Namespace)
	}
	_ = d.Set("availability_mode", pulsarInstance.Spec.AvailabilityMode)
	metadata := pulsarInstance.GetMetadata()
	d.SetId(fmt.Sprintf("%s/%s", metadata.GetNamespace(), metadata.GetName()))
	return nil
}
