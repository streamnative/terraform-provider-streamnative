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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var clusterGeneratedName = fmt.Sprintf("terraform-test-pulsar-cluster-%d", rand.Intn(10000))

func TestPulsarCluster(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckPulsarClusterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourcePulsarCluster(
					"sndev",
					clusterGeneratedName,
					"terraform-test-pulsar-instance",
					"us-central1"),
				Check: resource.ComposeTestCheckFunc(
					testCheckPulsarClusterExists("streamnative_pulsar_cluster.test-pulsar-cluster"),
				),
			},
		},
	})
}

func testCheckPulsarClusterDestroy(s *terraform.State) error {
	//time.Sleep(10 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_pulsar_cluster" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			PulsarClusters(organizationCluster[0]).
			Get(context.Background(), organizationCluster[1], metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_PULSAR_CLUSTER_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckPulsarClusterExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_PULSAR_CLUSTER_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("ERROR_RESOURCE_PULSAR_CLUSTER_ID_NOT_SET")
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		pulsarCluster, err := clientSet.CloudV1alpha1().
			PulsarClusters(organizationCluster[0]).
			Get(context.Background(), organizationCluster[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pulsarCluster.Status.Conditions != nil {
			ready := false
			for _, condition := range pulsarCluster.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					ready = true
				}
			}
			if !ready {
				return fmt.Errorf(`ERROR_RESOURCE_PULSAR_CLUSTER_NOT_READY: "%s"`, rs.Primary.ID)
			}
		}
		return nil
	}
}

func testResourceDataSourcePulsarCluster(organization, name, instanceName, location string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_pulsar_cluster" "test-pulsar-cluster" {
	organization = "%s"
	name = "%s"
  	instance_name = "%s"
  	location = "%s"
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
data "streamnative_pulsar_cluster" "test-pulsar-cluster" {
  depends_on = [streamnative_pulsar_cluster.test-pulsar-cluster]
  organization = streamnative_pulsar_cluster.test-pulsar-cluster.organization
  name = streamnative_pulsar_cluster.test-pulsar-cluster.name
}
`, organization, name, instanceName, location)
}
