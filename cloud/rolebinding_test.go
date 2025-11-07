package cloud

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"strings"
	"testing"
	"time"
)

func TestPredefinedBinding(t *testing.T) {
	serviceAccount, err := uuid.NewRandom()
	assert.NoError(t, err)
	rolebindingName, err := uuid.NewRandom()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			time.Sleep(5 * time.Second)
			for _, rs := range state.RootModule().Resources {
				if rs.Type != "streamnative_rolebinding" {
					continue
				}
				meta := testAccProvider.Meta()
				clientSet, err := getClientSet(getFactoryFromMeta(meta))
				if err != nil {
					return err
				}
				organizationRolebinding := strings.Split(rs.Primary.ID, "/")
				_, err = clientSet.CloudV1alpha1().
					RoleBindings(organizationRolebinding[0]).
					Get(context.Background(), organizationRolebinding[1], metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf(`ERROR_RESOURCE_ROLEBINDING_STILL_EXISTS: "%s"`, rs.Primary.ID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "streamnative" {
}

resource "streamnative_service_account" "test-service-account" {
	organization = "sndev"
	name = "%s"
}

resource "streamnative_rolebinding" "rolebinding_demo" {
  depends_on = [streamnative_service_account.test-service-account]
  organization = "sndev"
  name         = "%s"
  cluster_role_name = "metrics-viewer"
  service_account_names = ["%s"]
}

data "streamnative_rolebinding" "rolebinding_demo" {
  depends_on = [streamnative_rolebinding.rolebinding_demo]
  organization = streamnative_rolebinding.rolebinding_demo.organization
  name         = streamnative_rolebinding.rolebinding_demo.name
}
`, serviceAccount, rolebindingName, serviceAccount),
				Check: func(state *terraform.State) error {
					rs, ok := state.RootModule().Resources["streamnative_rolebinding.rolebinding_demo"]
					if !ok {
						return fmt.Errorf(`ERROR_RESOURCE_SERVICE_ACCOUNT_NOT_FOUND: rolebinding_demo`)
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
					rolebinding, err := clientSet.CloudV1alpha1().
						RoleBindings(organizationCluster[0]).
						Get(context.Background(), organizationCluster[1], metav1.GetOptions{})
					if err != nil {
						return err
					}
					if rolebinding.Status.Conditions[0].Type != "Ready" || rolebinding.Status.Conditions[0].Status != "True" {
						return fmt.Errorf(`ERROR_RESOURCE_ROLEBINDING_NOT_READY: "%s"`, rs.Primary.ID)
					}
					return nil
				},
			},
		},
	})
}

func TestRoleBinding_ConditionParse(t *testing.T) {
	orgName := "test"
	requestResourceData := dataSourceRoleBinding().TestResourceData()
	requestBinding := &v1alpha1.RoleBinding{
		Spec: v1alpha1.RoleBindingSpec{
			CEL: pointer.String("srn.instance == 'a'"),
		},
	}
	err := conditionParse(orgName, requestBinding, requestResourceData)
	assert.NoError(t, err)
	expectRoleBinding := &v1alpha1.RoleBinding{}
	conditionSet(orgName, requestResourceData, expectRoleBinding)
	assert.Equal(t, expectRoleBinding.Spec, requestBinding.Spec)

	requestResourceData = dataSourceRoleBinding().TestResourceData()
	requestBinding = &v1alpha1.RoleBinding{
		Spec: v1alpha1.RoleBindingSpec{
			ResourceNames: []v1alpha1.ResourceName{
				{
					Organization: orgName,
					Instance:     "ins-1",
					Cluster:      "cluster-1",
					Tenant:       "tenant-1",
				},
				{
					Organization: orgName,
					Instance:     "ins-2",
					Cluster:      "cluster-2",
					Tenant:       "tenant-2",
				},
			}},
	}
	err = conditionParse(orgName, requestBinding, requestResourceData)
	assert.NoError(t, err)
	expectRoleBinding = &v1alpha1.RoleBinding{}
	conditionSet(orgName, requestResourceData, expectRoleBinding)
	assert.Equal(t, expectRoleBinding.Spec, requestBinding.Spec)

}
