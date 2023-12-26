package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
	"time"
)

func TestResourceServiceAccount(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckResourceServiceAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceServiceAccount("sndev", "test-service-account-b", true),
				Check: resource.ComposeTestCheckFunc(
					testCheckResourceServiceAccountExists("streamnative_service_account.test-service-account"),
				),
			},
		},
	})
}

func testCheckResourceServiceAccountDestroy(s *terraform.State) error {
	// Add a sleep for wait the service account to be deleted
	// It seems that azure connection to gcp is slow, so add a delay to wait
	// for the resource to be cleaned up and check it again
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_service_account" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationServiceAccount := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			ServiceAccounts(organizationServiceAccount[0]).
			Get(context.Background(), organizationServiceAccount[1], metav1.GetOptions{})
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

func testCheckResourceServiceAccountExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_ID_NOT_SET`)
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		serviceAccount, err := clientSet.CloudV1alpha1().
			ServiceAccounts(organizationCluster[0]).
			Get(context.Background(), organizationCluster[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if serviceAccount.Status.Conditions[0].Type != "Ready" || serviceAccount.Status.PrivateKeyData == "" {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_NOT_READY: "%s"`, rs.Primary.ID)
		}
		return nil
	}
}

func testResourceServiceAccount(organization string, name string, admin bool) string {
	return fmt.Sprintf(`
provider "streamnative" {
}

resource "streamnative_service_account" "test-service-account" {
	organization = "%s"
	name = "%s"
	admin = %t
}
`, organization, name, admin)
}