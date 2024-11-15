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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dataSourcePool() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourcePoolRead,
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
				Description:  descriptions["pool_name"],
				ValidateFunc: validateNotBlank,
			},
			"type": {
				Type:        schema.TypeString,
				Description: descriptions["pool_type"],
				Computed:    true,
			},
		},
	}
}

func DataSourcePoolRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_POOL: %w", err))
	}
	pool, err := clientSet.CloudV1alpha1().Pools(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_POOL: %w", err))
	}
	_ = d.Set("name", pool.Name)
	_ = d.Set("organization", pool.Namespace)
	_ = d.Set("type", pool.Spec.Type)
	d.SetId(fmt.Sprintf("%s/%s", pool.Namespace, pool.Name))

	return nil
}
