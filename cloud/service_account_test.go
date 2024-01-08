package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"testing"
	"time"
)

func TestServiceAccount(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckServiceAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceServiceAccount(
					"sndev", "terraform-test-service-account-b", true),
				Check: resource.ComposeTestCheckFunc(
					testCheckServiceAccountExists("streamnative_service_account.test-service-account"),
				),
			},
		},
	})
}

func testCheckServiceAccountDestroy(s *terraform.State) error {
	// Add a sleep for wait the service account to be deleted
	// It seems that azure connection to gcp is slow, so add a delay to wait
	// for the resource to be cleaned up and check it again
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_service_account" {
			continue
		}
		meta := testAccProvider.Meta()
		apiClient := getFactoryFromMeta(meta)
		organizationServiceAccount := strings.Split(rs.Primary.ID, "/")
		_, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedServiceAccount(
			context.Background(), organizationServiceAccount[1], organizationServiceAccount[0]).Execute()
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckServiceAccountExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_ID_NOT_SET`)
		}
		meta := testAccProvider.Meta()
		apiClient := getFactoryFromMeta(meta)
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		serviceAccount, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedServiceAccount(
			context.Background(), organizationCluster[1], organizationCluster[0]).Execute()
		if err != nil {
			return err
		}
		if serviceAccount.Status.Conditions[0].Type != "Ready" || *serviceAccount.Status.PrivateKeyData == "" {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_NOT_READY: "%s"`, rs.Primary.ID)
		}
		return nil
	}
}

func testResourceDataSourceServiceAccount(organization string, name string, admin bool) string {
	return fmt.Sprintf(`
provider "streamnative" {
}

resource "streamnative_service_account" "test-service-account" {
	organization = "%s"
	name = "%s"
	admin = %t
}

data "streamnative_service_account" "test-service-account" {
  depends_on = [streamnative_service_account.test-service-account]
  organization = streamnative_service_account.test-service-account.organization
  name = streamnative_service_account.test-service-account.name
}
`, organization, name, admin)
}
