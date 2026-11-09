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
	"encoding/json"
	"testing"

	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// secretResourceData plans config against state the way the SDK does during
// apply, including the raw config that write-only values are read from.
func secretResourceData(
	t *testing.T, state *terraform.InstanceState, config map[string]interface{},
) *schema.ResourceData {
	t.Helper()
	r := resourceSecret()
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)
	rawConfig, err := ctyjson.Unmarshal(configJSON, r.CoreConfigSchema().ImpliedType())
	require.NoError(t, err)

	sm := schema.InternalMap(r.Schema)
	resourceConfig := terraform.NewResourceConfigShimmed(rawConfig, r.CoreConfigSchema())
	diff, err := sm.Diff(context.Background(), state, resourceConfig, nil, nil, true)
	require.NoError(t, err)
	if diff == nil {
		diff = &terraform.InstanceDiff{}
	}
	diff.RawConfig = rawConfig
	d, err := sm.Data(state, diff)
	require.NoError(t, err)
	return d
}

func TestSecretWriteOnlySchema(t *testing.T) {
	s := resourceSecret().Schema
	for _, key := range []string{"data_wo", "string_data_wo"} {
		assert.True(t, s[key].WriteOnly, key)
		assert.True(t, s[key].Sensitive, key)
		assert.True(t, s[key].Optional, key)
		assert.False(t, s[key].Computed, key)
		assert.ElementsMatch(t, []string{"data", "string_data"}, s[key].ConflictsWith, key)
		assert.Equal(t, []string{key + "_version"}, s[key].RequiredWith, key)
		assert.Equal(t, []string{key}, s[key+"_version"].RequiredWith, key)
		assert.False(t, s[key+"_version"].WriteOnly, key)
	}
	for _, key := range []string{"data", "string_data"} {
		assert.False(t, s[key].WriteOnly, key)
		assert.True(t, s[key].ForceNew, key)
		assert.ElementsMatch(t, []string{"data", "string_data", "data_wo", "string_data_wo"}, s[key].AtLeastOneOf, key)
	}
}

func TestSecretWriteOnlyValidation(t *testing.T) {
	type cfg = map[string]interface{}
	kv := cfg{"k": "v"}
	both := cfg{"data_wo": kv, "data_wo_version": 1, "string_data_wo": kv, "string_data_wo_version": 1}
	cases := map[string]struct {
		config  cfg
		wantErr bool
	}{
		"data":                  {cfg{"data": kv}, false},
		"string_data":           {cfg{"string_data": kv}, false},
		"string_data_wo":        {cfg{"string_data_wo": kv, "string_data_wo_version": 1}, false},
		"data_wo":               {cfg{"data_wo": kv, "data_wo_version": 1}, false},
		"both write-only":       {both, false},
		"nothing":               {cfg{}, true},
		"data and data_wo":      {cfg{"data": kv, "data_wo": kv, "data_wo_version": 1}, true},
		"string_data and wo":    {cfg{"string_data": kv, "string_data_wo": kv, "string_data_wo_version": 1}, true},
		"data and string wo":    {cfg{"data": kv, "string_data_wo": kv, "string_data_wo_version": 1}, true},
		"wo without version":    {cfg{"string_data_wo": kv}, true},
		"version without wo":    {cfg{"data": kv, "data_wo_version": 1}, true},
		"version below minimum": {cfg{"data_wo": kv, "data_wo_version": 0}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.config["organization"], tc.config["name"] = "org", "secret"
			diags := resourceSecret().Validate(terraform.NewResourceConfigRaw(tc.config))
			assert.Equal(t, tc.wantErr, diags.HasError(), "%v", diags)
		})
	}
}

func TestBuildSecretWriteOnlyData(t *testing.T) {
	d := secretResourceData(t, nil, map[string]interface{}{
		"organization":           "org",
		"name":                   "secret",
		"data_wo":                map[string]interface{}{"cert": "pem"},
		"data_wo_version":        1,
		"string_data_wo":         map[string]interface{}{"password": "hunter2"},
		"string_data_wo_version": 1,
	})

	secret, diags := buildSecretFromResourceData(d)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, map[string]string{"cert": "pem"}, secret.Data)
	assert.Equal(t, map[string]string{"password": "hunter2"}, secret.StringData)
}

func TestBuildSecretRejectsNullWriteOnlyValue(t *testing.T) {
	d := secretResourceData(t, nil, map[string]interface{}{
		"organization":           "org",
		"name":                   "secret",
		"string_data_wo":         map[string]interface{}{"password": nil},
		"string_data_wo_version": 1,
	})

	_, diags := buildSecretFromResourceData(d)
	assert.True(t, diags.HasError())
}

func TestBuildSecretWithoutWriteOnlyData(t *testing.T) {
	d := secretResourceData(t, nil, map[string]interface{}{
		"organization": "org",
		"name":         "secret",
		"data":         map[string]interface{}{"username": "user"},
		"string_data":  map[string]interface{}{"password": "hunter2"},
	})

	secret, diags := buildSecretFromResourceData(d)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, map[string]string{"username": "user"}, secret.Data)
	assert.Equal(t, map[string]string{"password": "hunter2"}, secret.StringData)
}

func TestApplySecretPlanResendsWriteOnlyDataOnVersionChange(t *testing.T) {
	state := &terraform.InstanceState{ID: "org/secret", Attributes: map[string]string{
		"id":                     "org/secret",
		"organization":           "org",
		"name":                   "secret",
		"string_data_wo_version": "1",
	}}
	config := map[string]interface{}{
		"organization":           "org",
		"name":                   "secret",
		"string_data_wo":         map[string]interface{}{"password": "rotated"},
		"string_data_wo_version": 2,
	}
	existing := func() *cloudv1alpha1.Secret {
		secret := &cloudv1alpha1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "org"}}
		secret.Data = map[string]string{"password": "ciphertext-of-old-value"}
		return secret
	}

	secret := existing()
	diags := applySecretPlan(secret, secretResourceData(t, state, config), false)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Nil(t, secret.Data)
	assert.Equal(t, map[string]string{"password": "rotated"}, secret.StringData)

	config["string_data_wo_version"] = 1
	secret = existing()
	diags = applySecretPlan(secret, secretResourceData(t, state, config), false)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, existing().Data, secret.Data)
	assert.Nil(t, secret.StringData)
}

func TestSetSecretStateKeepsWriteOnlyDataOutOfState(t *testing.T) {
	apiSecret := &cloudv1alpha1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "org"}}
	apiSecret.Data = map[string]string{"password": "plaintext"}

	writeOnly := resourceSecret().Data(&terraform.InstanceState{ID: "org/secret", Attributes: map[string]string{
		"id":              "org/secret",
		"data_wo_version": "1",
	}})
	require.False(t, setSecretState(writeOnly, apiSecret).HasError())
	assert.Empty(t, writeOnly.Get("data"))
	for k, v := range writeOnly.State().Attributes {
		assert.NotContains(t, v, "plaintext", k)
	}

	plain := resourceSecret().Data(&terraform.InstanceState{ID: "org/secret", Attributes: map[string]string{
		"id": "org/secret",
	}})
	require.False(t, setSecretState(plain, apiSecret).HasError())
	assert.Equal(t, map[string]interface{}{"password": "plaintext"}, plain.Get("data"))

	dataSource := dataSourceSecret().TestResourceData()
	require.False(t, setSecretState(dataSource, apiSecret).HasError())
	assert.Equal(t, map[string]interface{}{"password": "plaintext"}, dataSource.Get("data"))
}
