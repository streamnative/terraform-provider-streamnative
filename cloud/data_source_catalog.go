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

func dataSourceCatalog() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceCatalogRead,
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
				Description:  descriptions["catalog_name"],
				ValidateFunc: validateNotBlank,
			},
			"mode": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_mode"],
				Computed:    true,
			},
			"unity_catalog_name": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_unity_catalog_name"],
				Computed:    true,
			},
			"unity_uri": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_unity_uri"],
				Computed:    true,
			},
			"unity_secret": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_secret"],
				Computed:    true,
			},
			"open_catalog_warehouse": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_warehouse"],
				Computed:    true,
			},
			"open_catalog_uri": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_open_catalog_uri"],
				Computed:    true,
			},
			"open_catalog_secret": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_secret"],
				Computed:    true,
			},
			"s3_table_bucket": {
				Type:        schema.TypeString,
				Description: "S3 table bucket ARN. Must be in format: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)",
				Computed:    true,
			},
			"s3_table_region": {
				Type:        schema.TypeString,
				Description: "AWS region extracted from S3 table bucket ARN or name",
				Computed:    true,
			},
			"ready": {
				Type:        schema.TypeString,
				Description: descriptions["catalog_ready"],
				Computed:    true,
			},
		},
	}
}

func dataSourceCatalogRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_CATALOG: %w", err))
	}

	catalog, err := clientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_CATALOG: %w", err))
	}

	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	if err = d.Set("organization", catalog.Namespace); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ORGANIZATION: %w", err))
	}
	if err = d.Set("name", catalog.Name); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_NAME: %w", err))
	}
	if err = d.Set("mode", string(catalog.Spec.Mode)); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_MODE: %w", err))
	}

	// Set Unity configuration
	if catalog.Spec.Unity != nil {
		// Set unity_catalog_name with fallback to Name if CatalogName is empty
		unityCatalogName := catalog.Spec.Unity.CatalogName
		if unityCatalogName == "" {
			unityCatalogName = catalog.Spec.Unity.Name
		}
		if err = d.Set("unity_catalog_name", unityCatalogName); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_UNITY_CATALOG_NAME: %w", err))
		}
		if err = d.Set("unity_uri", catalog.Spec.Unity.URI); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_UNITY_URI: %w", err))
		}
		if err = d.Set("unity_secret", catalog.Spec.Unity.Secret); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_UNITY_SECRET: %w", err))
		}
	}

	// Set OpenCatalog configuration
	if catalog.Spec.OpenCatalog != nil {
		if err = d.Set("open_catalog_warehouse", catalog.Spec.OpenCatalog.Warehouse); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_OPEN_CATALOG_WAREHOUSE: %w", err))
		}
		if err = d.Set("open_catalog_uri", catalog.Spec.OpenCatalog.URI); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_OPEN_CATALOG_URI: %w", err))
		}
		if err = d.Set("open_catalog_secret", catalog.Spec.OpenCatalog.Secret); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_OPEN_CATALOG_SECRET: %w", err))
		}
	}

	// Set S3Table configuration
	if catalog.Spec.S3Table != nil {
		if err = d.Set("s3_table_bucket", catalog.Spec.S3Table.Warehouse); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_S3_TABLE_BUCKET: %w", err))
		}

		// Extract and set region from bucket
		region, err := extractS3TableRegion(catalog.Spec.S3Table.Warehouse)
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_EXTRACT_S3_TABLE_REGION: %w", err))
		}
		if err = d.Set("s3_table_region", region); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_S3_TABLE_REGION: %w", err))
		}
	}

	// Set ready status
	_ = d.Set("ready", "False")
	if catalog.Status.Conditions != nil && len(catalog.Status.Conditions) > 0 {
		for _, condition := range catalog.Status.Conditions {
			if condition.Type == "Ready" {
				_ = d.Set("ready", condition.Status)
			}
		}
	}

	return nil
}
