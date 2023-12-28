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

func TestResourcePulsarCluster(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckResourcePulsarClusterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourcePulsarCluster(
					"sndev",
					"terraform-test-pulsar-cluster",
					"terraform-test-pulsar-instance",
					"us-central1"),
				Check: resource.ComposeTestCheckFunc(
					testCheckResourcePulsarClusterExists("streamnative_pulsar_cluster.test-pulsar-cluster"),
				),
			},
		},
	})
}

func testCheckResourcePulsarClusterDestroy(s *terraform.State) error {
	time.Sleep(5 * time.Second)
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

func testCheckResourcePulsarClusterExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_PULSAR_INSTANCE_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("ERROR_RESOURCE_PULSAR_INSTANCE_ID_NOT_SET")
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

func testResourcePulsarCluster(organization, name, instanceName, location string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_pulsar_cluster" "test-pulsar-cluster" {
	organization = "%s"
	name = "%s"
  	instance_name = "%s"
  	location = "%s"
  	websocket_enabled = true
  	function_enabled = true
  	transaction_enabled = true
  	kafka = {}
  	mqtt = {}
  	audit_log = ["Management", "Describe", "Produce", "Consume"]
  	custom = {
    	"allowAutoTopicCreation": "true"
  	}
}
`, organization, name, instanceName, location)
}
