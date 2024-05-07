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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dataSourcePulsarGateway() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePulsarGatewayRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationInstance := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationInstance[0])
				_ = d.Set("name", organizationInstance[1])
				err := resourcePulsarGatewayRead(ctx, d, meta)
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
				Computed:    true,
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
			"private_service_ids": {
				Type:         schema.TypeList,
				Optional:     true,
				Computed:     true,
				Description:  descriptions["private_service_ids"],
				ValidateFunc: validation.ListOfUniqueStrings,
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
	d.Set("poolmember_name", pg.Spec.PoolMemberRef.Name)
	d.Set("poolmember_namespace", pg.Spec.PoolMemberRef.Namespace)

	d.Set("ready", "False")
	if pg.Status.Conditions != nil {
		for _, condition := range pg.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				_ = d.Set("ready", "True")
			}
		}
	}

	if pg.Spec.Access == v1alpha1.AccessType(cloud.PrivateAccess) {
		privateService := make(map[string]interface{})
		if pg.Spec.PrivateService != nil {
			privateService["allowed_ids"] = pg.Spec.PrivateService.AllowedIds
			d.Set("private_service", privateService)
		}
		if pg.Spec.PrivateService != nil {
			d.Set("private_service_ids", pg.Status.PrivateServiceIds)
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return nil
}
