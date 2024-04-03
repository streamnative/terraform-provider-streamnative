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
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var apiKeyGeneratedName = fmt.Sprintf("terraform-test-pulsar-cluster-%d", rand.Intn(10000))

func TestApiKey(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckApiKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceApiKey(
					"terraform-test-api-key", apiKeyGeneratedName),
				Check: resource.ComposeTestCheckFunc(
					testCheckApiKeyExists("streamnative_apikey.test-terraform-api-key"),
				),
			},
		},
	})
}

func testCheckApiKeyDestroy(s *terraform.State) error {
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_api_key" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationApiKey := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().APIKeys(organizationApiKey[0]).Get(
			context.Background(), organizationApiKey[1], metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_API_KEY_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckApiKeyExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_API_KEY_NOT_FOUND: "%s"`, resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("ERROR_RESOURCE_API_KEY_ID_NOT_SET")
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationApiKey := strings.Split(rs.Primary.ID, "/")
		apiKey, err := clientSet.CloudV1alpha1().APIKeys(organizationApiKey[0]).Get(
			context.Background(), organizationApiKey[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if apiKey.Status.Conditions != nil {
			ready := false
			for _, condition := range apiKey.Status.Conditions {
				if condition.Type == "Issued" && condition.Status == "True" {
					ready = true
				}
			}
			if !ready {
				return fmt.Errorf(`ERROR_RESOURCE_API_KEY_NOT_READY: "%s"`, rs.Primary.ID)
			}
		}
		return nil
	}
}

func testResourceDataSourceApiKey(clusterName string, apiKeyName string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_pulsar_cluster" "test-api-key-pulsar-cluster" {
	organization = "sndev"
	name = "%s"
  	instance_name = "terraform-test-api-key-pulsar-instance"
  	location = "us-central1"
	config {
		websocket_enabled = true
		function_enabled = false
		transaction_enabled = false
		protocols {
		  mqtt = {
			enabled = "true"
		  }
		  kafka = {
			enabled = "true"
		  }
		}
		custom = {
			"allowAutoTopicCreation" = "true"
			"bookkeeper.journalSyncData" = "false"
		}
	}
}
data "streamnative_pulsar_cluster" "test-api-key-pulsar-cluster" {
  depends_on = [streamnative_pulsar_cluster.test-api-key-pulsar-cluster]
  organization = streamnative_pulsar_cluster.test-api-key-pulsar-cluster.organization
  name = streamnative_pulsar_cluster.test-api-key-pulsar-cluster.name
}
resource "streamnative_apikey" "test-terraform-api-key" {
  depends_on = [streamnative_pulsar_cluster.test-api-key-pulsar-cluster]
  organization = "sndev"
  name = "%s"
  instance_name = "terraform-test-api-key-pulsar-instance"
  service_account_name = "terraform-test-api-key-service-account"
  # just for testing, please don't set it to true for avoid token revoked
  revoke = true
  description = "This is a test api key"
}

data "streamnative_apikey" "test-terraform-api-key" {
  depends_on = [streamnative_apikey.test-terraform-api-key]
  organization = streamnative_apikey.test-terraform-api-key.organization
  name = streamnative_apikey.test-terraform-api-key.name
  private_key = streamnative_apikey.test-terraform-api-key.private_key
}
`, clusterName, apiKeyName)
}
