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

func TestResourcePulsarInstance(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckResourcePulsarInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourcePulsarInstance(
					"sndev",
					"terraform-test-pulsar-instance-b",
					"zonal",
					"shared-gcp",
					"streamnative"),
				Check: resource.ComposeTestCheckFunc(
					testCheckResourcePulsarInstanceExists("streamnative_pulsar_instance.test-pulsar-instance"),
				),
			},
		},
	})
}

func testCheckResourcePulsarInstanceDestroy(s *terraform.State) error {
	// Add a sleep for wait the service account to be deleted
	// It seems that azure connection to gcp is slow, so add a delay to wait
	// for the resource to be cleaned up and check it again
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_pulsar_instance" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		organizationInstance := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			PulsarInstances(organizationInstance[0]).
			Get(context.Background(), organizationInstance[1], metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_PULSAR_INSTANCE_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckResourcePulsarInstanceExists(name string) resource.TestCheckFunc {
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
		organizationInstance := strings.Split(name, "/")
		pulsarInstance, err := clientSet.CloudV1alpha1().
			PulsarInstances(organizationInstance[0]).
			Get(context.Background(), organizationInstance[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pulsarInstance.Status.Conditions != nil {
			ready := false
			for _, condition := range pulsarInstance.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					ready = true
				}
			}
			if !ready {
				return fmt.Errorf(`ERROR_RESOURCE_PULSAR_INSTANCE_NOT_READY: "%s"`, rs.Primary.ID)
			}
		}
		return nil
	}
}

func testResourcePulsarInstance(
	organization string, name string, availabilityMode string, poolName string, poolNamespace string) string {
	return fmt.Sprintf(`
provider "streamnative" {
}
resource "streamnative_pulsar_instance" "test-pulsar-instance" {
	organization = "%s"
	name = "%s"
	availability_mode = "%s"
	pool_name = "%s"
	pool_namespace = "%s"
}
`, organization, name, availabilityMode, poolName, poolNamespace)
}
