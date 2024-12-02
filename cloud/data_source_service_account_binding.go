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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dataSourceServiceAccountBinding() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourceServiceAccountBindingRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(
				ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationServiceAccount := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationServiceAccount[0])
				_ = d.Set("name", organizationServiceAccount[1])
				err := resourceServiceAccountBindingRead(ctx, d, meta)
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
				Description:  descriptions["service_account_binding_name"],
				ValidateFunc: validateNotBlank,
			},
			"service_account_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["service_account_name"],
			},
			"pool_member_name": {
				Type:        schema.TypeString,
				Description: descriptions["pool_member_name"],
				Computed:    true,
			},
			"pool_member_namespace": {
				Type:        schema.TypeString,
				Description: descriptions["pool_member_namespace"],
				Computed:    true,
			},
		},
	}
}

func DataSourceServiceAccountBindingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	serviceAccountBinding, err := clientSet.CloudV1alpha1().ServiceAccountBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	_ = d.Set("name", serviceAccountBinding.Name)
	_ = d.Set("organization", serviceAccountBinding.Namespace)
	_ = d.Set("service_account_name", serviceAccountBinding.Spec.ServiceAccountName)
	_ = d.Set("pool_member_name", serviceAccountBinding.Spec.PoolMemberRef.Name)
	_ = d.Set("pool_member_namespace", serviceAccountBinding.Spec.PoolMemberRef.Namespace)
	d.SetId(fmt.Sprintf("%s/%s", serviceAccountBinding.Namespace, serviceAccountBinding.Name))

	return nil
}
