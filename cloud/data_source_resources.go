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
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"

	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

var (
	validResourceNames = []string{
		"pools",
		"poolmembers",
		"pulsarclusters",
		"pulsarinstances",
		"cloudconnections",
		"cloudenvironments",
		"catalogs",
	}
)

func dataSourceResources() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourceGetResources,
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["organization"],
				ValidateFunc: validateNotBlank,
			},
			"resource": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["resource_name"],
				ValidateFunc: validation.StringInSlice(validResourceNames, true),
			},
			"names": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func DataSourceGetResources(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	resource := strings.ToLower(d.Get("resource").(string))
	dynamicClient, err := getDynamicClient(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_RESOURCES: %w", err))
	}
	l, err := dynamicClient.Resource(k8sschema.GroupVersionResource{
		Group:    cloudv1alpha1.ApiVersion.GroupVersion.Group,
		Version:  cloudv1alpha1.ApiVersion.GroupVersion.Version,
		Resource: resource,
	}).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_RESOURCES: %w", err))
	}
	var names []string
	idsum := sha256.New()
	for _, item := range l.Items {
		_, err := idsum.Write([]byte(item.GetName()))
		if err != nil {
			return diag.FromErr(err)
		}
		names = append(names, item.GetName())
	}
	_ = d.Set("names", names)
	d.SetId(fmt.Sprintf("%s/%s", namespace, resource))
	return nil
}
