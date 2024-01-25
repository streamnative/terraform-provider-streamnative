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

func dataSourceCloudEnvironment() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceCloudEnvironmentRead,
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
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["region"],
			},
			"cloud_connection_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["cloud_connection_name"],
			},
			"network": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["network"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"cidr": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceCloudEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_CLOUD_ENVIRONMENT: %w", err))
	}
	cloudEnvironment, err := clientSet.CloudV1alpha1().CloudEnvironments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT: %w", err))
	}

	_ = d.Set("name", cloudEnvironment.Name)
	_ = d.Set("organization", cloudEnvironment.Namespace)
	_ = d.Set("region", cloudEnvironment.Spec.Region)
	_ = d.Set("cloud_connection_name", cloudEnvironment.Spec.CloudConnectionName)

	if cloudEnvironment.Spec.Network != nil {
		err = d.Set("network", flattenCloudEnvironmentNetwork(cloudEnvironment.Spec.Network))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_ENVIRONMENT_CONFIG: %w", err))
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", cloudEnvironment.Namespace, cloudEnvironment.Name))

	return nil
}
