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

func TestServiceAccountBinding(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckServiceAccountBindingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceServiceAccountBinding(
					"sndev",
					clusterGeneratedName,
					"shared-gcp-prod",
					"streamnative",
					"us-central1", "rapid"),
				Check: resource.ComposeTestCheckFunc(
					testCheckServiceAccountBindingExists("streamnative_service_account_binding.test-service-account-binding"),
				),
			},
		},
	})
}

func TestServiceAccountBindingRemovedExternally(t *testing.T) {
	// This test case is to simulate the situation that the service account binding is removed externally
	// and the terraform state still has the resource
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckServiceAccountBindingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceServiceAccountBinding(
					"sndev",
					clusterGeneratedName,
					"shared-gcp-prod",
					"streamnative",
					"us-central1", "rapid"),
				Check: resource.ComposeTestCheckFunc(
					testCheckServiceAccountBindingExists("streamnative_service_account_binding.test-service-account-binding"),
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
						ServiceAccountBindings("sndev").
						Delete(context.Background(), clusterGeneratedName+".streamnative.gcp-shared-usc1-test", metav1.DeleteOptions{})
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: testResourceDataSourceServiceAccountBinding(
					"sndev",
					clusterGeneratedName,
					"shared-gcp-prod",
					"streamnative",
					"us-central1", "rapid"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testCheckServiceAccountBindingDestroy(s *terraform.State) error {
	// Add a sleep for wait the service account binding to be deleted
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_service_account_binding" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationServiceAccountBinding := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			ServiceAccountBindings(organizationServiceAccountBinding[0]).
			Get(context.Background(), organizationServiceAccountBinding[1], metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_BINDING_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckServiceAccountBindingExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_BINDING_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_BINDING_ID_NOT_SET`)
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		serviceAccountBinding, err := clientSet.CloudV1alpha1().
			ServiceAccountBindings(organizationCluster[0]).
			Get(context.Background(), organizationCluster[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		length := len(serviceAccountBinding.Status.Conditions)
		if serviceAccountBinding.Status.Conditions[length-1].Type != "Ready" {
			return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_BINDING_NOT_READY: "%s"`, rs.Primary.ID)
		}
		return nil
	}
}

func testResourceDataSourceServiceAccountBinding(organization, name, poolName, poolNamespace, location, releaseChannel string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_pulsar_instance" "test-pulsar-instance" {
	organization = "%s"
	name = "%s"
	availability_mode = "zonal"
	pool_name = "%s"
	pool_namespace = "%s"
	type = "dedicated"
}
resource "streamnative_pulsar_cluster" "test-pulsar-cluster" {
	organization = "%s"
	name = "%s"
	instance_name = "%s"
	location = "%s"
	release_channel = "%s"
	depends_on = [streamnative_pulsar_instance.test-pulsar-instance]
}

resource "streamnative_service_account" "test-service-account" {
	organization = "%s"
	name = "%s"
	admin = %t
}

resource "streamnative_service_account_binding" "test-service-account-binding" {
	organization = "%s"
	service_account_name = streamnative_service_account.test-service-account.name
    cluster_name = streamnative_pulsar_cluster.test-pulsar-cluster.name
}

`, organization, name, poolName, poolNamespace, organization, name, name, location, releaseChannel, organization, name, true)
}
