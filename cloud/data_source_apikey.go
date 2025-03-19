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
	"encoding/base64"
	"fmt"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"net/url"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwe"
	"github.com/streamnative/terraform-provider-streamnative/cloud/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dataSourceApiKey() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourceApiKeyRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(
				ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationApiKey := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationApiKey[0])
				_ = d.Set("name", organizationApiKey[1])
				err := resourceApiKeyRead(ctx, d, meta)
				if err.HasError() {
					return nil, fmt.Errorf("import %q: %s", d.Id(), err[0].Summary)
				}
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["organization"],
				ValidateFunc: validateNotBlank,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["apikey_name"],
				ValidateFunc: validateNotBlank,
			},
			"private_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: descriptions["private_key"],
			},
			"instance_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["instance_name"],
			},
			"service_account_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["service_account_name"],
			},
			"description": {
				Type:        schema.TypeString,
				Description: descriptions["description"],
				Computed:    true,
			},
			"token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: descriptions["token"],
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["apikey_ready"],
			},
			"issued_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["issued_at"],
			},
			"expires_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["expires_at"],
			},
			"key_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["key_id"],
			},
			"revoked_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["revoked_at"],
			},
			"principal_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["principal_name"],
			},
		},
	}
}

func DataSourceApiKeyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	organization := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_API_KEY: %w", err))
	}
	apiKey, err := clientSet.CloudV1alpha1().APIKeys(organization).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_API_KEY: %w", err))
	}
	if err = d.Set("organization", apiKey.Namespace); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ORGANIZATION: %w", err))
	}
	if err = d.Set("name", apiKey.Name); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_NAME: %w", err))
	}
	if err = d.Set("description", apiKey.Spec.Description); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_DESCRIPTION: %w", err))
	}
	if err = d.Set("instance_name", apiKey.Spec.InstanceName); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_INSTANCE_NAME: %w", err))
	}
	if err = d.Set("service_account_name", apiKey.Spec.ServiceAccountName); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_SERVICE_ACCOUNT_NAME: %w", err))
	}
	if len(apiKey.Status.Conditions) == 3 {
		for _, condition := range apiKey.Status.Conditions {
			if condition.Type == "Issued" && condition.Status == "True" {
				if err = d.Set("issued_at", apiKey.Status.IssuedAt.String()); err != nil {
					return diag.FromErr(fmt.Errorf("ERROR_SET_ISSUED_AT: %w", err))
				}
				if err = d.Set("expires_at", apiKey.Status.ExpiresAt.String()); err != nil {
					return diag.FromErr(fmt.Errorf("ERROR_SET_EXPIRES_AT: %w", err))
				}
				if err = d.Set("key_id", apiKey.Status.KeyId); err != nil {
					return diag.FromErr(fmt.Errorf("ERROR_SET_KEY_ID: %w", err))
				}
				if err = d.Set("ready", "True"); err != nil {
					return diag.FromErr(fmt.Errorf("ERROR_SET_READY: %w", err))
				}
				privateKey := d.Get("private_key")
				if apiKey.Status.EncryptedToken.JWE != nil && privateKey != nil {
					data, err := base64.StdEncoding.DecodeString(d.Get("private_key").(string))
					if err != nil {
						return diag.FromErr(fmt.Errorf("ERROR_DECODE_PRIVATE_KEY: %w", err))
					}
					privateKey, err := util.ImportPrivateKey(string(data))
					if err != nil {
						return diag.FromErr(fmt.Errorf("ERROR_IMPORT_PRIVATE_KEY: %w", err))
					}
					token, err := jwe.Decrypt([]byte(*apiKey.Status.EncryptedToken.JWE), jwe.WithKey(jwa.RSA_OAEP, privateKey))
					if err != nil {
						return diag.FromErr(fmt.Errorf("ERROR_DECRYPT_API_KEY: %w", err))
					}
					if err = d.Set("token", string(token)); err != nil {
						return diag.FromErr(fmt.Errorf("ERROR_SET_TOKEN: %w", err))
					}
				}
			}
		}
		if apiKey.Status.RevokedAt != nil {
			if err = d.Set("revoked_at", apiKey.Status.RevokedAt.String()); err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_SET_REVOKED_AT: %w", err))
			}
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", apiKey.Namespace, apiKey.Name))
	return setPrincipalName(apiKey, d)
}

func setPrincipalName(apiKey *v1alpha1.APIKey, d *schema.ResourceData) diag.Diagnostics {
	defaultIssuer := os.Getenv("GLOBAL_DEFAULT_ISSUER")
	if defaultIssuer == "" {
		defaultIssuer = GlobalDefaultIssuer
	}
	u, err := url.Parse(defaultIssuer)
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_PARSE_DEFAULT_ISSUER: %w", err))
	}
	err = d.Set("principal_name", fmt.Sprintf(
		"%s@%s.%s", apiKey.Spec.ServiceAccountName, apiKey.Namespace, u.Host))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_PRINCIPAL_NAME: %w", err))
	}
	return nil
}
