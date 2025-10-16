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
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	pulsarv1alpha1 "github.com/streamnative/sn-operator/api/pulsar/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourceCatalog() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCatalogCreate,
		ReadContext:   resourceCatalogRead,
		UpdateContext: resourceCatalogUpdate,
		DeleteContext: resourceCatalogDelete,
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  descriptions["organization"],
				ValidateFunc: validateNotBlank,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  descriptions["catalog_name"],
				ValidateFunc: validateNotBlank,
			},
			"mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "EXTERNAL",
				Description: descriptions["catalog_mode"],
			},
			"unity_catalog_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog_unity_catalog_name"],
			},
			"unity_uri": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog_unity_uri"],
			},
			"unity_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog_secret"],
			},
			"open_catalog_warehouse": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog_warehouse"],
			},
			"open_catalog_uri": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog_open_catalog_uri"],
			},
			"open_catalog_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog_secret"],
			},
			"s3_table_bucket": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "S3 table bucket ARN. Must be in format: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)",
			},
			"s3_table_region": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "AWS region extracted from S3 table bucket ARN or name",
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["catalog_ready"],
			},
		},
	}
}

func resourceCatalogCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	mode := d.Get("mode").(string)

	// Set default mode if not provided
	if mode == "" {
		mode = "EXTERNAL"
	}

	// Validate that the mode is supported
	if err := validateCatalogMode(mode); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_VALIDATE_CATALOG_MODE: %w", err))
	}

	// Validate that only one catalog type is configured
	if err := validateCatalogType(d); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_VALIDATE_CATALOG_TYPE: %w", err))
	}

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_CATALOG: %w", err))
	}

	catalog := &v1alpha1.Catalog{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Catalog",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.CatalogSpec{
			Mode: pulsarv1alpha1.TableMode(mode),
		},
	}

	// Set Unity configuration
	if d.Get("unity_uri").(string) != "" {
		catalog.Spec.Unity = &v1alpha1.Unity{
			CatalogName: d.Get("unity_catalog_name").(string),
			CatalogConnection: v1alpha1.CatalogConnection{
				URI:    d.Get("unity_uri").(string),
				Secret: d.Get("unity_secret").(string),
			},
		}
	}

	// Set OpenCatalog configuration
	if openCatalogWarehouse := d.Get("open_catalog_warehouse").(string); openCatalogWarehouse != "" || d.Get("open_catalog_uri").(string) != "" {
		catalog.Spec.OpenCatalog = &v1alpha1.Iceberg{
			Warehouse: d.Get("open_catalog_warehouse").(string),
			CatalogConnection: v1alpha1.CatalogConnection{
				URI:    d.Get("open_catalog_uri").(string),
				Secret: d.Get("open_catalog_secret").(string),
			},
		}
	}

	// Set S3Table configuration
	if s3TableBucket := d.Get("s3_table_bucket").(string); s3TableBucket != "" {
		// Generate URI from bucket name
		uri, err := generateS3TableURI(s3TableBucket)
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_GENERATE_S3_TABLE_URI: %w", err))
		}

		catalog.Spec.S3Table = &v1alpha1.Iceberg{
			Warehouse: s3TableBucket,
			CatalogConnection: v1alpha1.CatalogConnection{
				URI: uri,
			},
		}
	}

	createdCatalog, err := clientSet.CloudV1alpha1().Catalogs(namespace).Create(ctx, catalog, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_CATALOG: %w", err))
	}

	if createdCatalog.Status.Conditions != nil && len(createdCatalog.Status.Conditions) > 0 {
		ready := false
		for _, condition := range createdCatalog.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if ready {
			_ = d.Set("organization", namespace)
			_ = d.Set("name", name)
			return resourceCatalogRead(ctx, d, meta)
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceCatalogRead(ctx, d, meta)
		if dia.HasError() {
			return retry.RetryableError(fmt.Errorf("ERROR_READ_CATALOG: %s", dia[0].Summary))
		}
		ready := d.Get("ready").(string)
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_WAITING_CATALOG_READY: catalog is not ready yet"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_WAIT_CATALOG_READY: %w", err))
	}
	return nil
}

func resourceCatalogDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_CATALOG: %w", err))
	}

	err = clientSet.CloudV1alpha1().Catalogs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_DELETE_CATALOG: %w", err))
	}

	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		_, err := clientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return retry.RetryableError(fmt.Errorf("ERROR_DELETE_CATALOG: %w", err))
		}
		return retry.RetryableError(fmt.Errorf("CONTINUE_WAITING_CATALOG_DELETE: %s", "catalog is not deleted yet"))
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_WAIT_CATALOG_DELETE: %w", err))
	}

	d.SetId("")
	return nil
}

func resourceCatalogRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_CATALOG: %w", err))
	}

	_ = d.Set("ready", "False")
	catalog, err := clientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_CATALOG: %w", err))
	}

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
		if err = d.Set("unity_catalog_name", catalog.Spec.Unity.CatalogName); err != nil {
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

	d.SetId(fmt.Sprintf("%s/%s", catalog.Namespace, catalog.Name))
	if catalog.Status.Conditions != nil && len(catalog.Status.Conditions) > 0 {
		for _, condition := range catalog.Status.Conditions {
			if condition.Type == "Ready" {
				_ = d.Set("ready", condition.Status)
			}
		}
	}
	return nil
}

func resourceCatalogUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	mode := d.Get("mode").(string)

	// Set default mode if not provided
	if mode == "" {
		mode = "EXTERNAL"
	}

	// Validate that the mode is supported
	if err := validateCatalogMode(mode); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_VALIDATE_CATALOG_MODE: %w", err))
	}

	// Validate that only one catalog type is configured
	if err := validateCatalogType(d); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_VALIDATE_CATALOG_TYPE: %w", err))
	}

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_UPDATE_CATALOG: %w", err))
	}

	catalog, err := clientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_GET_CATALOG_ON_UPDATE: %w", err))
	}

	catalog.Spec.Mode = pulsarv1alpha1.TableMode(mode)

	// Update Unity configuration
	if d.Get("unity_uri").(string) != "" {
		catalog.Spec.Unity = &v1alpha1.Unity{
			CatalogName: d.Get("unity_catalog_name").(string),
			CatalogConnection: v1alpha1.CatalogConnection{
				URI:    d.Get("unity_uri").(string),
				Secret: d.Get("unity_secret").(string),
			},
		}
	} else {
		catalog.Spec.Unity = nil
	}

	// Update OpenCatalog configuration
	if openCatalogWarehouse := d.Get("open_catalog_warehouse").(string); openCatalogWarehouse != "" || d.Get("open_catalog_uri").(string) != "" {
		catalog.Spec.OpenCatalog = &v1alpha1.Iceberg{
			Warehouse: d.Get("open_catalog_warehouse").(string),
			CatalogConnection: v1alpha1.CatalogConnection{
				URI:    d.Get("open_catalog_uri").(string),
				Secret: d.Get("open_catalog_secret").(string),
			},
		}
	} else {
		catalog.Spec.OpenCatalog = nil
	}

	// Update S3Table configuration
	if s3TableBucket := d.Get("s3_table_bucket").(string); s3TableBucket != "" {
		// Generate URI from bucket name
		uri, err := generateS3TableURI(s3TableBucket)
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_GENERATE_S3_TABLE_URI: %w", err))
		}

		catalog.Spec.S3Table = &v1alpha1.Iceberg{
			Warehouse: s3TableBucket,
			CatalogConnection: v1alpha1.CatalogConnection{
				URI: uri,
			},
		}
	} else {
		catalog.Spec.S3Table = nil
	}

	_, err = clientSet.CloudV1alpha1().Catalogs(namespace).Update(ctx, catalog, metav1.UpdateOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_CATALOG: %w", err))
	}

	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceCatalogRead(ctx, d, meta)
		if dia.HasError() {
			return retry.RetryableError(fmt.Errorf("ERROR_READ_CATALOG"))
		}
		ready := d.Get("ready").(string)
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf(
				"CONTINUE_WAITING_CATALOG_READY: catalog is not ready yet"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_WAIT_CATALOG_READY: %w", err))
	}

	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

// Helper function to convert map[string]interface{} to map[string]string
func convertMapToStringMap(input map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range input {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

// validateCatalogType checks that only one catalog type is configured
func validateCatalogType(d *schema.ResourceData) error {
	catalogTypes := 0

	// Check Unity configuration
	if d.Get("unity_uri").(string) != "" {
		catalogTypes++
	}

	// Check OpenCatalog configuration
	if openCatalogWarehouse := d.Get("open_catalog_warehouse").(string); openCatalogWarehouse != "" || d.Get("open_catalog_uri").(string) != "" {
		catalogTypes++
	}

	// Check S3Table configuration
	if s3TableBucket := d.Get("s3_table_bucket").(string); s3TableBucket != "" {
		catalogTypes++
	}

	if catalogTypes > 1 {
		return fmt.Errorf("catalog can only have one type configured (Unity, OpenCatalog, or S3Table), found %d types", catalogTypes)
	}

	return nil
}

// validateCatalogMode checks that the mode is currently supported
func validateCatalogMode(mode string) error {
	if mode != "EXTERNAL" {
		return fmt.Errorf("catalog mode '%s' is not supported, currently only 'EXTERNAL' mode is supported", mode)
	}
	return nil
}

// extractS3TableRegion extracts region from S3 bucket ARN
// Only supports ARN format: arn:aws:s3tables:region:account:bucket/name
// Example: arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket
// Returns the extracted region
func extractS3TableRegion(bucket string) (string, error) {
	if bucket == "" {
		return "", fmt.Errorf("bucket name cannot be empty")
	}

	// Only support ARN format
	if !strings.HasPrefix(bucket, "arn:aws:s3tables:") {
		return "", fmt.Errorf("invalid bucket format, only ARN format is supported: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)")
	}

	// Parse ARN format: arn:aws:s3tables:region:account:bucket/name
	parts := strings.Split(bucket, ":")
	if len(parts) < 6 {
		return "", fmt.Errorf("invalid ARN format, expected: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)")
	}

	// Validate ARN structure
	if parts[0] != "arn" || parts[1] != "aws" || parts[2] != "s3tables" {
		return "", fmt.Errorf("invalid ARN format, expected: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)")
	}

	// Check if the last part contains bucket/name format
	lastPart := parts[5]
	if !strings.Contains(lastPart, "bucket/") {
		return "", fmt.Errorf("invalid ARN format, last part must contain 'bucket/' prefix: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)")
	}

	return parts[3], nil // region is the 4th part in ARN
}

// generateS3TableURI generates URI from S3 bucket ARN
// Only supports ARN format: arn:aws:s3tables:region:account:bucket/name
// Example: arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket
// Returns URI in format: https://s3tables.{region}.amazonaws.com/iceberg
func generateS3TableURI(bucket string) (string, error) {
	region, err := extractS3TableRegion(bucket)
	if err != nil {
		return "", err
	}

	// Generate URI
	uri := fmt.Sprintf("https://s3tables.%s.amazonaws.com/iceberg", region)
	return uri, nil
}
