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
					"terraform-test-pulsar-cluster",
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
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_pulsar_cluster" {
			continue
		}
		meta := testAccProvider.Meta()
		apiClient := getFactoryFromMeta(meta)
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		_, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedPulsarCluster(
			context.Background(), organizationCluster[1], organizationCluster[0]).Execute()
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
			return fmt.Errorf(`ERROR_RESOURCE_PULSAR_INSTANCE_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("ERROR_RESOURCE_PULSAR_INSTANCE_ID_NOT_SET")
		}
		meta := testAccProvider.Meta()
		apiClient := getFactoryFromMeta(meta)
		organizationCluster := strings.Split(rs.Primary.ID, "/")
		pulsarCluster, _, err := apiClient.CloudStreamnativeIoV1alpha1Api.ReadNamespacedPulsarCluster(
			context.Background(), organizationCluster[1], organizationCluster[0]).Execute()
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
			allowAutoTopicCreation = "true"
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
