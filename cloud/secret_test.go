package cloud

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecret(t *testing.T) {
	data := map[string]string{
		"username": "tf-user",
		"password": "tf-password",
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceSecret("sndev", "terraform-test-secret", data),
				Check: resource.ComposeTestCheckFunc(
					testCheckSecretExists("streamnative_secret.test-secret", data),
				),
			},
		},
	})
}

func TestSecretRemovedExternally(t *testing.T) {
	data := map[string]string{
		"token": "removed-secret",
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceSecret("sndev", "terraform-test-secret-remove", data),
				Check: resource.ComposeTestCheckFunc(
					testCheckSecretExists("streamnative_secret.test-secret", data),
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
						Secrets("sndev").
						Delete(context.Background(), "terraform-test-secret-remove", metav1.DeleteOptions{})
					if err != nil && !apierrors.IsNotFound(err) {
						t.Fatal(err)
					}
				},
				Config:             testResourceDataSourceSecret("sndev", "terraform-test-secret-remove", data),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testCheckSecretDestroy(s *terraform.State) error {
	time.Sleep(5 * time.Second)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "streamnative_secret" {
			continue
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		parts := strings.Split(rs.Primary.ID, "/")
		_, err = clientSet.CloudV1alpha1().
			Secrets(parts[0]).
			Get(context.Background(), parts[1], metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf(`ERROR_RESOURCE_SECRET_STILL_EXISTS: "%s"`, rs.Primary.ID)
	}
	return nil
}

func testCheckSecretExists(name string, expectedData map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf(`ERROR_RESOURCE_SECRET_NOT_FOUND: "%s"`, name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf(`ERROR_RESOURCE_SECRET_ID_NOT_SET`)
		}
		meta := testAccProvider.Meta()
		clientSet, err := getClientSet(getFactoryFromMeta(meta))
		if err != nil {
			return err
		}
		parts := strings.Split(rs.Primary.ID, "/")
		secret, err := clientSet.CloudV1alpha1().
			Secrets(parts[0]).
			Get(context.Background(), parts[1], metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(secret.Data) != len(expectedData) {
			return fmt.Errorf("unexpected secret data length: got %d, expected %d", len(secret.Data), len(expectedData))
		}
		for k, v := range expectedData {
			if secret.Data[k] != v {
				return fmt.Errorf("secret data mismatch for key %q: got %q, expected %q", k, secret.Data[k], v)
			}
		}
		return nil
	}
}

func testResourceDataSourceSecret(organization string, name string, data map[string]string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(fmt.Sprintf(`    %s = "%s"`+"\n", k, data[k]))
	}
	return fmt.Sprintf(`
provider "streamnative" {
}

resource "streamnative_secret" "test-secret" {
  organization = "%s"
  name = "%s"
  data = {
%s  }
}

data "streamnative_secret" "test-secret" {
  depends_on = [streamnative_secret.test-secret]
  organization = streamnative_secret.test-secret.organization
  name = streamnative_secret.test-secret.name
}
`, organization, name, builder.String())
}
