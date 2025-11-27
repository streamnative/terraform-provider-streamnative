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

func TestSecretStringData(t *testing.T) {
	stringData := map[string]string{
		"username": "tf-user-string",
		"password": "tf-password-string",
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceSecretWithStringData("sndev", "terraform-test-secret-stringdata", stringData),
				Check: resource.ComposeTestCheckFunc(
					testCheckSecretExists("streamnative_secret.test-secret", stringData),
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

func TestSecretUpdate(t *testing.T) {
	initialData := map[string]string{
		"username": "tf-user-update",
		"password": "tf-password-update",
	}
	updatedStringData := map[string]string{
		"username": "tf-user-updated",
		"password": "tf-password-updated",
	}
	initialType := "Opaque"
	updatedType := "kubernetes.io/basic-auth"
	initialInstance := "pulsar-instance-a"
	updatedInstance := "pulsar-instance-b"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testCheckSecretDestroy,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceSecretWithParams("sndev", "terraform-test-secret-update", initialData, nil, initialType, initialInstance),
				Check: resource.ComposeTestCheckFunc(
					testCheckSecretState("streamnative_secret.test-secret", initialData, &initialType, &initialInstance),
				),
			},
			{
				Config: testResourceDataSourceSecretWithParams("sndev", "terraform-test-secret-update", nil, updatedStringData, updatedType, updatedInstance),
				Check: resource.ComposeTestCheckFunc(
					testCheckSecretState("streamnative_secret.test-secret", updatedStringData, &updatedType, &updatedInstance),
				),
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
	return testResourceDataSourceSecretWithParams(organization, name, data, nil, "", "")
}

func testResourceDataSourceSecretWithStringData(organization string, name string, stringData map[string]string) string {
	return testResourceDataSourceSecretWithParams(organization, name, nil, stringData, "", "")
}

func testResourceDataSourceSecretWithParams(
	organization string,
	name string,
	data map[string]string,
	stringData map[string]string,
	secretType string,
	instanceName string,
) string {
	var resourceBuilder strings.Builder
	resourceBuilder.WriteString(fmt.Sprintf(`resource "streamnative_secret" "test-secret" {
  organization = "%s"
  name = "%s"
`, organization, name))
	if instanceName != "" {
		resourceBuilder.WriteString(fmt.Sprintf(`  instance_name = "%s"
`, instanceName))
	}
	if secretType != "" {
		resourceBuilder.WriteString(fmt.Sprintf(`  type = "%s"
`, secretType))
	}
	if len(data) > 0 {
		resourceBuilder.WriteString("  data = {\n")
		resourceBuilder.WriteString(buildHCLMap(data))
		resourceBuilder.WriteString("  }\n")
	}
	if len(stringData) > 0 {
		resourceBuilder.WriteString("  string_data = {\n")
		resourceBuilder.WriteString(buildHCLMap(stringData))
		resourceBuilder.WriteString("  }\n")
	}
	resourceBuilder.WriteString("}\n")

	return fmt.Sprintf(`
provider "streamnative" {
}

%s
data "streamnative_secret" "test-secret" {
  depends_on = [streamnative_secret.test-secret]
  organization = streamnative_secret.test-secret.organization
  name = streamnative_secret.test-secret.name
}
`, resourceBuilder.String())
}

func buildHCLMap(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(fmt.Sprintf(`    %s = "%s"`+"\n", k, values[k]))
	}
	return builder.String()
}

func testCheckSecretState(name string, expectedData map[string]string, expectedType *string, expectedInstanceName *string) resource.TestCheckFunc {
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
		if expectedData != nil {
			if len(secret.Data) != len(expectedData) {
				return fmt.Errorf("unexpected secret data length: got %d, expected %d", len(secret.Data), len(expectedData))
			}
			for k, v := range expectedData {
				if secret.Data[k] != v {
					return fmt.Errorf("secret data mismatch for key %q: got %q, expected %q", k, secret.Data[k], v)
				}
			}
		}
		if expectedType != nil {
			if secret.Type == nil {
				return fmt.Errorf("secret type is nil, expected %q", *expectedType)
			}
			if string(*secret.Type) != *expectedType {
				return fmt.Errorf("secret type mismatch: got %q, expected %q", string(*secret.Type), *expectedType)
			}
		}
		if expectedInstanceName != nil {
			if secret.InstanceName != *expectedInstanceName {
				return fmt.Errorf("secret instance_name mismatch: got %q, expected %q", secret.InstanceName, *expectedInstanceName)
			}
		}
		return nil
	}
}
