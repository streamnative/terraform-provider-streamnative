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

func dataSourceCloudEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	serviceAccount, err := clientSet.CloudV1alpha1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT: %w", err))
	}
	_ = d.Set("name", serviceAccount.Name)
	_ = d.Set("organization", serviceAccount.Namespace)
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = serviceAccount.Status.PrivateKeyData
	}
	_ = d.Set("private_key_data", privateKeyData)
	if serviceAccount.Annotations != nil && serviceAccount.Annotations[ServiceAccountAdminAnnotation] == "admin" {
		_ = d.Set("admin", true)
	} else {
		_ = d.Set("admin", false)
	}
	d.SetId(fmt.Sprintf("%s/%s", serviceAccount.Namespace, serviceAccount.Name))

	return nil
}
