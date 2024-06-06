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

	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func dataSourcePoolMember() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourcePoolMemberRead,
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
				Description:  descriptions["pool_member_name"],
				ValidateFunc: validateNotBlank,
			},
			"type": {
				Type:        schema.TypeString,
				Description: descriptions["pool_member_type"],
				Computed:    true,
			},
			"pool_name": {
				Type:        schema.TypeString,
				Description: descriptions["pool_name"],
				Computed:    true,
			},
			"location": {
				Type:        schema.TypeString,
				Description: descriptions["location"],
				Computed:    true,
			},
		},
	}
}

func DataSourcePoolMemberRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_POOL_MEMBER: %w", err))
	}
	poolMember, err := clientSet.CloudV1alpha1().PoolMembers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_POOL_MEMBER: %w", err))
	}
	_ = d.Set("name", poolMember.Name)
	_ = d.Set("organization", poolMember.Namespace)
	_ = d.Set("type", poolMember.Spec.Type)
	_ = d.Set("pool_name", poolMember.Spec.PoolName)
	switch poolMember.Spec.Type {
	case cloudv1alpha1.PoolMemberTypeAws:
		_ = d.Set("location", poolMember.Spec.AWS.Region)
	case cloudv1alpha1.PoolMemberTypeGCloud:
		_ = d.Set("location", poolMember.Spec.GCloud.Location)
	case cloudv1alpha1.PoolMemberTypeAzure:
		_ = d.Set("location", poolMember.Spec.AZURE.Location)
	}
	d.SetId(fmt.Sprintf("%s/%s", poolMember.Namespace, poolMember.Name))

	return nil
}
