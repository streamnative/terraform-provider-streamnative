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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretDataSourceDoesNotExposeBinaryData(t *testing.T) {
	if _, ok := dataSourceSecret().Schema["binary_data"]; ok {
		t.Fatal("data source should not expose write-only binary_data")
	}
}

func TestSecretBinaryDataSchema(t *testing.T) {
	binaryDataSchema := resourceSecret().Schema["binary_data"]
	if binaryDataSchema == nil {
		t.Fatal("binary_data schema is missing")
	}
	if binaryDataSchema.Type != schema.TypeMap {
		t.Fatalf("binary_data schema type = %v, expected %v", binaryDataSchema.Type, schema.TypeMap)
	}
	if !binaryDataSchema.Optional {
		t.Fatal("binary_data should be optional")
	}
	if !binaryDataSchema.Sensitive {
		t.Fatal("binary_data should be sensitive")
	}
	if !binaryDataSchema.ForceNew {
		t.Fatal("binary_data should force replacement")
	}
	if got, want := binaryDataSchema.AtLeastOneOf, []string{"data", "string_data", "binary_data"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("binary_data AtLeastOneOf = %v, expected %v", got, want)
	}
}

func TestValidateBase64String(t *testing.T) {
	validValues := []string{"", "Y2VydA==", "AAECAw=="}
	for _, value := range validValues {
		_, errs := validateBase64String(value, "binary_data.key")
		if len(errs) != 0 {
			t.Fatalf("validateBase64String(%q) returned errors: %v", value, errs)
		}
	}

	_, errs := validateBase64String("not base64", "binary_data.key")
	if len(errs) == 0 {
		t.Fatal("validateBase64String should reject invalid base64")
	}
}

func TestValidateSecretDataKeyUniqueness(t *testing.T) {
	resource := resourceSecret()
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"organization": "org-a",
		"name":         "secret-a",
		"string_data": map[string]interface{}{
			"shared": "plain text",
		},
		"binary_data": map[string]interface{}{
			"shared": "YmluYXJ5",
		},
	})

	_, err := resource.SimpleDiff(context.Background(), nil, config, nil)
	if err == nil {
		t.Fatal("SimpleDiff should reject duplicate secret payload keys")
	}
	if !strings.Contains(err.Error(), `secret data key "shared"`) {
		t.Fatalf("unexpected duplicate-key error: %v", err)
	}
}

func TestBuildSecretFromResourceDataWithBinaryData(t *testing.T) {
	resourceData := resourceSecret().TestResourceData()
	mustSetResourceData(t, resourceData, "organization", "org-a")
	mustSetResourceData(t, resourceData, "name", "secret-a")
	mustSetResourceData(t, resourceData, "binary_data", map[string]string{
		"cert.p12": "YmluYXJ5LWNlcnQ=",
	})

	secret := buildSecretFromResourceData(resourceData)
	if got := secret.BinaryData["cert.p12"]; got != "YmluYXJ5LWNlcnQ=" {
		t.Fatalf("secret.BinaryData[cert.p12] = %q", got)
	}
	if secret.StringData != nil {
		t.Fatalf("secret.StringData = %v, expected nil", secret.StringData)
	}
}

func TestSetSecretStatePreservesBinaryData(t *testing.T) {
	resourceData := resourceSecret().TestResourceData()
	mustSetResourceData(t, resourceData, "organization", "stale-org")
	mustSetResourceData(t, resourceData, "name", "stale-secret")
	mustSetResourceData(t, resourceData, "binary_data", map[string]string{
		"cert.p12": "YmluYXJ5LWNlcnQ=",
	})

	secret := &cloudv1alpha1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "org-a",
			Name:      "secret-a",
		},
		Data: map[string]string{
			"cert.p12": "encrypted-value",
		},
	}

	diags := setSecretState(resourceData, secret)
	if diags.HasError() {
		t.Fatalf("setSecretState returned diagnostics: %v", diags)
	}

	binaryData := resourceData.Get("binary_data").(map[string]interface{})
	if got := binaryData["cert.p12"]; got != "YmluYXJ5LWNlcnQ=" {
		t.Fatalf("binary_data[cert.p12] = %q", got)
	}
	if got := resourceData.Get("data").(map[string]interface{})["cert.p12"]; got != "encrypted-value" {
		t.Fatalf("data[cert.p12] = %q", got)
	}
}

func mustSetResourceData(t *testing.T, d *schema.ResourceData, key string, value interface{}) {
	t.Helper()
	if err := d.Set(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}
