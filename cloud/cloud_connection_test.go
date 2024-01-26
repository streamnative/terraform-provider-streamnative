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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCloudConnection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckcloudConnectionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceCloudConnection(
					"sndev",
					"terraform-test-cloud-connection-b",
					"us-west1"),
				Check: resource.ComposeTestCheckFunc(
					testCheckcloudConnectionExists("streamnative_cloud_connection.test-cloud-connection"),
				),
			},
		},
	})
}

func testCheckcloudConnectionDestroy(s *terraform.State) error {
	// Add a sleep for wait the service account to be deleted
	// It seems that azure connection to gcp is slow, so add a delay to wait
	// for the resource to be cleaned up and check it again
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_cloud_connection" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationInstance := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			CloudConnections(organizationInstance[0]).
			Get(context.Background(), organizationInstance[1], metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_CLOUD_CONNECTION_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckcloudConnectionExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_CLOUD_CONNECTION_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("ERROR_RESOURCE_CLOUD_CONNECTION_ID_NOT_SET")
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationInstance := strings.Split(rs.Primary.ID, "/")
		cloudConnection, err := clientSet.CloudV1alpha1().
			CloudConnections(organizationInstance[0]).
			Get(context.Background(), organizationInstance[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cloudConnection.Status.Conditions != nil {
			ready := false
			for _, condition := range cloudConnection.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					ready = true
				}
			}
			if !ready {
				return fmt.Errorf(`ERROR_RESOURCE_CLOUD_CONNECTION_NOT_READY: "%s"`, rs.Primary.ID)
			}
		}
		return nil
	}
}

func testResourceDataSourceCloudConnection(
	organization string, name string, cloudType string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_cloud_connection" "test-cloud-connection" {
	organization = "%s"
	name = "%s"
	type = "%s"
	aws {
		account_id = "test-account"
	}
}
data "streamnative_cloud_connection" "test-cloud-connection" {
  depends_on = [streamnative_cloud_connection.test-cloud-connection]
  name = streamnative_cloud_connection.test-cloud-connection.name
  organization = streamnative_cloud_connection.test-cloud-connection.organization
}
`, organization, name, cloudType)
}
