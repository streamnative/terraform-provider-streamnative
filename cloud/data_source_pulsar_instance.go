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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				Description:  descriptions["instance_name"],
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
			"oauth2_audience": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["oauth2_audience"],
			},
			"oauth2_issuer_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["oauth2_issuer_url"],
			},
		},
	}
}

func dataSourcePulsarInstanceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	pulsarInstance, err := clientSet.CloudV1alpha1().PulsarInstances(namespace).Get(ctx, name, metav1.GetOptions{})
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
	if pulsarInstance.Status.Auth != nil && pulsarInstance.Status.Auth.Type == "oauth2" &&
		pulsarInstance.Status.Auth.OAuth2 != nil {
		_ = d.Set("oauth2_audience", pulsarInstance.Status.Auth.OAuth2.Audience)
		_ = d.Set("oauth2_issuer_url", pulsarInstance.Status.Auth.OAuth2.IssuerURL)
	}
	d.SetId(fmt.Sprintf("%s/%s", pulsarInstance.Namespace, pulsarInstance.Name))
	return nil
}
