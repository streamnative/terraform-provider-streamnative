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
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceAccount(t *testing.T) {
	serviceAccountName := randomServiceAccountName("terraform-test-service-account")
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckServiceAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceServiceAccount(
					"sndev", serviceAccountName, true),
				Check: resource.ComposeTestCheckFunc(
					testCheckServiceAccountExists("streamnative_service_account.test-service-account"),
				),
			},
		},
	})
}

func TestServiceAccountRemovedExternally(t *testing.T) {
	serviceAccountName := randomServiceAccountName("terraform-test-service-account-remove")
	// This test case is to simulate the situation that the service account is removed externally
	// and the terraform state still has the resource
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckServiceAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceServiceAccount(
					"sndev", serviceAccountName, true),
				Check: resource.ComposeTestCheckFunc(
					testCheckServiceAccountExists("streamnative_service_account.test-service-account"),
				),
			},
			{
				PreConfig: func() {
					meta := testAccProvider.Meta()
					clientSet, err := getClientSet(getFactoryFromMeta(meta))
					if err != nil {
						t.Fatal(err)
					}
					err = clientSet.CloudV1alpha1().
						ServiceAccounts("sndev").
						Delete(context.Background(), serviceAccountName, metav1.DeleteOptions{})
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: testResourceDataSourceServiceAccount(
					"sndev", serviceAccountName, true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
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

func randomServiceAccountName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
}
