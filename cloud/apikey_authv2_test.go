// Copyright 2026 StreamNative, Inc.
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
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwe"
	"github.com/lestrrat-go/jwx/v2/jwk"
	cloudapi "github.com/streamnative/cloud-api-server/pkg/apis/cloud"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type apiKeyTestFactory struct {
	cmdutil.Factory
	config *rest.Config
}

func (f apiKeyTestFactory) ToRESTConfig() (*rest.Config, error) { return f.config, nil }

type apiKeyTestTransport func(*http.Request) (*http.Response, error)

func (f apiKeyTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestApiKeyCreateAuthMode(t *testing.T) {
	for _, tt := range []struct {
		name         string
		instance     string
		features     map[string]bool
		orgForbidden bool
		wantError    string
	}{
		{name: "organization key", features: map[string]bool{"AuthV2": true}},
		{name: "AuthV2 disabled", features: map[string]bool{"AuthV2": false}, wantError: "instance_name is required when AuthV2 is not enabled"},
		{name: "AuthV2 unset", wantError: "instance_name is required when AuthV2 is not enabled"},
		{name: "organization read denied", orgForbidden: true, wantError: "ERROR_READ_ORGANIZATION_ON_CREATE_API_KEY"},
		{name: "existing instance path", instance: "test-instance", orgForbidden: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			const orgPath = "/apis/cloud.streamnative.io/v1alpha1/organizations/test-org"
			const keysPath = "/apis/cloud.streamnative.io/v1alpha1/namespaces/test-org/apikeys"
			var requests []string
			var apiKey *v1alpha1.APIKey
			factory := apiKeyTestFactory{config: &rest.Config{
				Host: "https://cloud.example.test",
				Transport: apiKeyTestTransport(func(req *http.Request) (*http.Response, error) {
					requests = append(requests, req.Method+" "+req.URL.Path)
					status := http.StatusOK
					var body interface{}
					switch {
					case req.Method == http.MethodGet && req.URL.Path == orgPath:
						if tt.orgForbidden {
							status = http.StatusForbidden
							body = &metav1.Status{Status: "Failure", Reason: metav1.StatusReasonForbidden, Code: http.StatusForbidden, Message: "organization read denied"}
						} else {
							body = &v1alpha1.Organization{Spec: v1alpha1.OrganizationSpec{CloudFeature: tt.features}}
						}
					case req.Method == http.MethodPost && req.URL.Path == keysPath:
						apiKey = &v1alpha1.APIKey{}
						require.NoError(t, json.NewDecoder(req.Body).Decode(apiKey))
						assert.Equal(t, tt.instance, apiKey.Spec.InstanceName)
						assert.Equal(t, "test-sa", apiKey.Spec.ServiceAccountName)
						if tt.instance == "" {
							assert.Equal(t, "v2", apiKey.Annotations[cloudapi.AnnotationAuthMode])
						} else {
							assert.Empty(t, apiKey.Annotations)
						}
						publicKey, err := jwk.ParseKey([]byte(apiKey.Spec.EncryptionKey.PEM), jwk.WithPEM(true))
						require.NoError(t, err)
						encrypted, err := jwe.Encrypt([]byte("test-token"), jwe.WithKey(jwa.RSA_OAEP, publicKey), jwe.WithContentEncryption(jwa.A256GCM))
						require.NoError(t, err)
						jweToken, keyID := string(encrypted), "test-key-id"
						now := metav1.Now()
						apiKey.Status = v1alpha1.APIKeyStatus{
							KeyId: &keyID, IssuedAt: &now, ExpiresAt: apiKey.Spec.ExpirationTime,
							EncryptedToken: &v1alpha1.EncryptedToken{JWE: &jweToken},
							Conditions: []metav1.Condition{
								{Type: "Issued", Status: metav1.ConditionTrue},
								{Type: "Revoked", Status: metav1.ConditionFalse},
								{Type: "Expired", Status: metav1.ConditionFalse},
							},
						}
						status, body = http.StatusCreated, apiKey
					case req.Method == http.MethodGet && req.URL.Path == keysPath+"/test-key":
						require.NotNil(t, apiKey)
						body = apiKey
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					encoded, err := json.Marshal(body)
					require.NoError(t, err)
					return &http.Response{
						StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
						Body: io.NopCloser(strings.NewReader(string(encoded))), Request: req,
					}, nil
				}),
			}}
			config := map[string]interface{}{"organization": "test-org", "name": "test-key", "service_account_name": "test-sa"}
			if tt.instance != "" {
				config["instance_name"] = tt.instance
			}
			resource := resourceApiKey()
			require.False(t, resource.Validate(terraform.NewResourceConfigRaw(config)).HasError())
			data := schema.TestResourceDataRaw(t, resource.Schema, config)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			diagnostics := resourceApiKeyCreate(ctx, data, factory)
			if tt.wantError != "" {
				require.True(t, diagnostics.HasError())
				assert.Contains(t, diagnostics[0].Summary, tt.wantError)
				assert.Equal(t, []string{"GET " + orgPath}, requests)
				assert.Empty(t, data.Id())
				return
			}
			require.False(t, diagnostics.HasError(), "%+v", diagnostics)
			wantRequests := []string{"POST " + keysPath, "GET " + keysPath + "/test-key"}
			if tt.instance == "" {
				wantRequests = append([]string{"GET " + orgPath}, wantRequests...)
			}
			assert.Equal(t, wantRequests, requests)
			assert.Equal(t, "test-org/test-key", data.Id())
			assert.Equal(t, "test-token", data.Get("token"))
			assert.Equal(t, "True", data.Get("ready"))
			require.False(t, resourceApiKeyRead(ctx, data, factory).HasError())
			diff, err := resource.SimpleDiff(ctx, data.State(), terraform.NewResourceConfigRaw(config), nil)
			require.NoError(t, err)
			assert.True(t, diff.Empty(), "refresh must not introduce a diff: %+v", diff)

			sourceData := schema.TestResourceDataRaw(t, dataSourceApiKey().Schema, map[string]interface{}{
				"organization": "test-org", "name": "test-key", "private_key": data.Get("private_key"),
			})
			require.False(t, DataSourceApiKeyRead(ctx, sourceData, factory).HasError())
			assert.Equal(t, tt.instance, sourceData.Get("instance_name"))
			assert.Equal(t, "test-token", sourceData.Get("token"))
		})
	}
}

func TestApiKeyInstanceReplacement(t *testing.T) {
	for _, instance := range []string{"test-instance", "other-instance", ""} {
		t.Run("instance="+instance, func(t *testing.T) {
			resource := resourceApiKey()
			config := map[string]interface{}{
				"organization": "test-org", "name": "test-key", "service_account_name": "test-sa", "instance_name": "test-instance",
			}
			data := schema.TestResourceDataRaw(t, resource.Schema, config)
			data.SetId("test-org/test-key")
			state := data.State()
			delete(config, "instance_name")
			if instance != "" {
				config["instance_name"] = instance
			}
			diff, err := resource.SimpleDiff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
			require.NoError(t, err)
			if instance == "test-instance" {
				assert.True(t, diff.Empty())
			} else {
				require.NotNil(t, diff)
				assert.True(t, diff.RequiresNew())
				assert.True(t, diff.Attributes["instance_name"].RequiresNew)
			}
		})
	}
}
