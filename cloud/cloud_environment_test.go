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

func TestCloudEnvironment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckcloudEnvironmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourcecloudEnvironment(
					"sndev",
					"terraform-test-cloud-environment-b",
					"us-west1",
					"sn-cloud-connection"),
				Check: resource.ComposeTestCheckFunc(
					testCheckcloudEnvironmentExists("streamnative_cloud_environment.test-cloud-environment"),
				),
			},
		},
	})
}

func testCheckcloudEnvironmentDestroy(s *terraform.State) error {
	// Add a sleep for wait the service account to be deleted
	// It seems that azure connection to gcp is slow, so add a delay to wait
	// for the resource to be cleaned up and check it again
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_cloud_environment" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationInstance := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			CloudEnvironments(organizationInstance[0]).
			Get(context.Background(), organizationInstance[1], metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_CLOUD_ENVIRONMENT_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckcloudEnvironmentExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_CLOUD_ENVIRONMENT_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("ERROR_RESOURCE_CLOUD_ENVIRONMENT_ID_NOT_SET")
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationInstance := strings.Split(rs.Primary.ID, "/")
		cloudEnvironment, err := clientSet.CloudV1alpha1().
			CloudEnvironments(organizationInstance[0]).
			Get(context.Background(), organizationInstance[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cloudEnvironment.Status.Conditions != nil {
			ready := false
			for _, condition := range cloudEnvironment.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					ready = true
				}
			}
			if !ready {
				return fmt.Errorf(`ERROR_RESOURCE_CLOUD_ENVIRONMENT_NOT_READY: "%s"`, rs.Primary.ID)
			}
		}
		return nil
	}
}

func testResourceDataSourcecloudEnvironment(
	organization string, name string, region string, cloudConnectionName string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_cloud_environment" "test-cloud-environment" {
	organization = "%s"
	name = "%s"
	region = "%s"
	cloud_connection_name = "%s"
	network {
		cidr = "10.0.16.0/20"
	}
}
data "streamnative_cloud_environment" "test-cloud-environment" {
  depends_on = [streamnative_cloud_environment.test-cloud-environment]
  name = streamnative_cloud_environment.test-cloud-environment.name
  organization = streamnative_cloud_environment.test-cloud-environment.organization
}
`, organization, name, region, cloudConnectionName)
}
