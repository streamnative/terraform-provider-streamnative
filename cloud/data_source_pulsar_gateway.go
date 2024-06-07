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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/streamnative/cloud-api-server/pkg/apis/cloud"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func dataSourcePulsarGateway() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePulsarGatewayRead,
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
				Description:  descriptions["gateway_name"],
				ValidateFunc: validateNotBlank,
			},
			"access": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["gateway_access"],
			},
			"pool_member_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pool_member_name"],
			},
			"pool_member_namespace": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pool_member_namespace"],
			},
			"private_service": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["gateway_private_service"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allowed_ids": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: descriptions["gateway_allowed_ids"],
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
			"private_service_ids": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["gateway_private_service_ids"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["gateway_ready"],
			},
		},
	}
}

func dataSourcePulsarGatewayRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_PULSAR_GATEWAY: %w", err))
	}
	pg, err := clientSet.CloudV1alpha1().PulsarGateways(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_GATEWAY: %w", err))
	}
	d.Set("access", pg.Spec.Access)
	d.Set("pool_member_name", pg.Spec.PoolMemberRef.Name)
	d.Set("pool_member_namespace", pg.Spec.PoolMemberRef.Namespace)

	d.Set("ready", "False")
	if pg.Status.Conditions != nil {
		for _, condition := range pg.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				_ = d.Set("ready", "True")
			}
		}
	}

	if pg.Spec.Access == cloudv1alpha1.AccessType(cloud.PrivateAccess) {
		if pg.Spec.PrivateService != nil {
			d.Set("private_service", flattenPrivateService(pg.Spec.PrivateService))
			d.Set("private_service_ids", flattenPrivateServiceIds(pg.Status.PrivateServiceIds))
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

func flattenPrivateService(in *cloudv1alpha1.PrivateService) []interface{} {
	att := make(map[string]interface{})
	if in.AllowedIds != nil {
		att["allowedIds"] = flattenStringSlice(in.AllowedIds)
	}
	return []interface{}{att}
}
func flattenPrivateServiceIds(in []cloudv1alpha1.PrivateServiceId) []interface{} {
	ids := make([]string, 0, len(in))
	for _, v := range in {
		ids = append(ids, v.Id)
	}
	return flattenStringSlice(ids)
}

func flattenStringSlice(in []string) []interface{} {
	ids := make([]interface{}, 0, len(in))
	for _, v := range in {
		ids = append(ids, v)
	}
	return ids
}
