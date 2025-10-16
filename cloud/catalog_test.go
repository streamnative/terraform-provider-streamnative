package cloud

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCatalogS3Table(t *testing.T) {
	catalogName, err := uuid.NewRandom()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			time.Sleep(5 * time.Second)
			for _, rs := range state.RootModule().Resources {
				if rs.Type != "streamnative_catalog" {
					continue
				}
				meta := testAccProvider.Meta()
				clientSet, err := getClientSet(getFactoryFromMeta(meta))
				if err != nil {
					return err
				}
				organizationCatalog := strings.Split(rs.Primary.ID, "/")
				_, err = clientSet.CloudV1alpha1().
					Catalogs(organizationCatalog[0]).
					Get(context.Background(), organizationCatalog[1], metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf(`ERROR_RESOURCE_CATALOG_STILL_EXISTS: "%s"`, rs.Primary.ID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "streamnative" {
}

resource "streamnative_catalog" "s3_table_catalog" {
  organization = "max"
  name         = "%s"
  # mode is optional and defaults to "EXTERNAL"

  # S3Table configuration with ARN format (required)
  s3_table_bucket = "arn:aws:s3tables:ap-northeast-1:598203581484:bucket/s3-table-test"
}

data "streamnative_catalog" "s3_table_catalog" {
  depends_on = [streamnative_catalog.s3_table_catalog]
  organization = streamnative_catalog.s3_table_catalog.organization
  name         = streamnative_catalog.s3_table_catalog.name
}
`, catalogName),
				Check: func(state *terraform.State) error {
					rs, ok := state.RootModule().Resources["streamnative_catalog.s3_table_catalog"]
					if !ok {
						return fmt.Errorf(`ERROR_RESOURCE_CATALOG_NOT_FOUND: s3_table_catalog`)
					}
					if rs.Primary.ID == "" {
						return fmt.Errorf(`ERROR_RESOURCE_CATALOG_ID_NOT_SET`)
					}
					meta := testAccProvider.Meta()
					clientSet, err := getClientSet(getFactoryFromMeta(meta))
					if err != nil {
						return err
					}
					organizationCatalog := strings.Split(rs.Primary.ID, "/")
					catalog, err := clientSet.CloudV1alpha1().
						Catalogs(organizationCatalog[0]).
						Get(context.Background(), organizationCatalog[1], metav1.GetOptions{})
					if err != nil {
						return err
					}
					if catalog.Status.Conditions != nil && len(catalog.Status.Conditions) > 0 {
						for _, condition := range catalog.Status.Conditions {
							if condition.Type == "Ready" && condition.Status == "True" {
								return nil
							}
						}
					}
					return fmt.Errorf(`ERROR_RESOURCE_CATALOG_NOT_READY: "%s"`, rs.Primary.ID)
				},
			},
		},
	})
}

func TestCatalogS3TableRegionExtraction(t *testing.T) {
	// Test the region extraction function
	region, err := extractS3TableRegion("arn:aws:s3tables:ap-northeast-1:598203581484:bucket/s3-table-test")
	assert.NoError(t, err)
	assert.Equal(t, "ap-northeast-1", region)

	// Test invalid ARN format
	_, err = extractS3TableRegion("invalid-arn")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bucket format")

	// Test empty bucket
	_, err = extractS3TableRegion("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name cannot be empty")
}

func TestCatalogS3TableURIGeneration(t *testing.T) {
	// Test URI generation
	uri, err := generateS3TableURI("arn:aws:s3tables:us-east-2:598203581484:bucket/test-bucket")
	assert.NoError(t, err)
	assert.Equal(t, "https://s3tables.us-east-2.amazonaws.com/iceberg", uri)

	// Test with different region
	uri, err = generateS3TableURI("arn:aws:s3tables:eu-west-1:598203581484:bucket/test-bucket")
	assert.NoError(t, err)
	assert.Equal(t, "https://s3tables.eu-west-1.amazonaws.com/iceberg", uri)
}

func TestCatalogModeValidation(t *testing.T) {
	// Test valid mode
	err := validateCatalogMode("EXTERNAL")
	assert.NoError(t, err)

	// Test invalid mode
	err = validateCatalogMode("INVALID")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestCatalogTypeValidation(t *testing.T) {
	// Create a mock ResourceData for testing
	resourceData := resourceCatalog().TestResourceData()

	// Test with no catalog types configured
	err := validateCatalogType(resourceData)
	assert.NoError(t, err)

	// Test with Unity configuration
	resourceData.Set("unity_uri", "https://test.com")
	err = validateCatalogType(resourceData)
	assert.NoError(t, err)

	// Test with multiple catalog types (should fail)
	resourceData.Set("open_catalog_uri", "https://open.com")
	err = validateCatalogType(resourceData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "can only have one type configured")
}
