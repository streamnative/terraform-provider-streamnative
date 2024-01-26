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

func dataSourceCloudConnection() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceCloudConnectionRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationInstance := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationInstance[0])
				_ = d.Set("name", organizationInstance[1])
				err := resourceCloudConnectionRead(ctx, d, meta)
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
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["name"],
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["type"],
			},
			"aws": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["aws"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"account_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"gcp": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["gcp"],
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"project": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceCloudConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_CLOUD_CONNECTION: %w", err))
	}
	cloudConnection, err := clientSet.CloudV1alpha1().CloudConnections(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION: %w", err))
	}

	_ = d.Set("name", cloudConnection.Name)
	_ = d.Set("organization", cloudConnection.Namespace)
	_ = d.Set("type", cloudConnection.Spec.ConnectionType)

	_ = d.Set("ready", "False")
	if cloudConnection.Status.Conditions != nil {
		for _, condition := range cloudConnection.Status.Conditions {
			if condition.Type == "Ready" {
				_ = d.Set("ready", condition.Status)
			}
		}
	}

	if cloudConnection.Spec.AWS != nil {
		err = d.Set("aws", flattenCloudConnectionAws(cloudConnection.Spec.AWS))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION_CONFIG: %w", err))
		}
	}

	if cloudConnection.Spec.GCloud != nil {
		err = d.Set("gcp", flattenCloudConnectionGcloud(cloudConnection.Spec.GCloud))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_CLOUD_CONNECTION_CONFIG: %w", err))
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", cloudConnection.Namespace, cloudConnection.Name))

	return nil
}
