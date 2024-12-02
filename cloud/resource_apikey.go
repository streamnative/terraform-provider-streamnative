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
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"github.com/streamnative/terraform-provider-streamnative/cloud/util"
	"github.com/xhit/go-str2duration/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourceApiKey() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceApiKeyCreate,
		ReadContext:   resourceApiKeyRead,
		UpdateContext: resourceApiKeyUpdate,
		DeleteContext: resourceApiKeyDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChange("organization") ||
				diff.HasChange("instance_name") ||
				diff.HasChange("service_account_name") ||
				diff.HasChange("expiration_time") {
				return fmt.Errorf("ERROR_UPDATE_API_KEY: " +
					"The api key does not support updates organization, " +
					"name, instance_name, service_account_name and expiration_time, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationApiKey := strings.Split(d.Id(), "/")
				if err := d.Set("organization", organizationApiKey[0]); err != nil {
					return nil, fmt.Errorf("ERROR_IMPORT_ORGANIZATION: %w", err)
				}
				if err := d.Set("name", organizationApiKey[1]); err != nil {
					return nil, fmt.Errorf("ERROR_IMPORT_NAME: %w", err)
				}
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
			"instance_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["instance_name"],
			},
			"service_account_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["service_account_name"],
			},
			"expiration_time": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["expiration_time"],
			},
			"revoke": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: descriptions["revoke"],
			},
			"description": {
				Type:        schema.TypeString,
				Description: descriptions["description"],
				Optional:    true,
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
			"private_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["private_key"],
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
		},
	}
}

func resourceApiKeyCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	instanceName := d.Get("instance_name").(string)
	serviceAccountName := d.Get("service_account_name").(string)
	description := d.Get("description").(string)
	expirationTime := d.Get("expiration_time").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_API_KEY: %w", err))
	}
	ak := &v1alpha1.APIKey{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIKey",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.APIKeySpec{
			InstanceName:       instanceName,
			ServiceAccountName: serviceAccountName,
		},
	}
	r1 := regexp.MustCompile(`^(\d+.)(s|m|h|d)$`)
	t := time.Now()
	if expirationTime != "" {
		if r1.MatchString(expirationTime) {
			ago, err := str2duration.ParseDuration(expirationTime)
			if err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_PARSE_EXPIRATION_TIME: %w", err))
			}
			t = t.Add(ago)
		} else if expirationTime != "0" {
			layout := "2006-02-01T15:04:05Z"
			t, err = time.Parse(layout, expirationTime)
			if err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_PARSE_EXPIRATION_TIME: %w", err))
			}
		}
	} else {
		defaultExpireTime, err := time.ParseDuration("720h")
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_PARSE_DEFAULT_EXPIRATION_TIME: %w", err))
		}
		t = t.Add(defaultExpireTime)
	}
	if expirationTime != "0" {
		ak.Spec.ExpirationTime = &metav1.Time{Time: t}
	}
	if description != "" {
		ak.Spec.Description = description
	}
	privateKey, err := util.GenerateEncryptionKey()
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_GENERATE_RSA_PRIVATE_KEY: %w", err))
	}
	encryptionKey, err := util.ExportPublicKey(privateKey)
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_EXPORT_PUBLIC_KEY: %w", err))
	}
	ak.Spec.EncryptionKey = &v1alpha1.EncryptionKey{
		PEM: encryptionKey.PEM,
	}
	revoke := d.Get("revoke").(bool)
	ak.Spec.Revoke = revoke
	_, err = clientSet.CloudV1alpha1().APIKeys(namespace).Create(ctx, ak, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_API_KEY: %w", err))
	}

	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	if err = d.Set(
		"private_key", base64.StdEncoding.EncodeToString([]byte(util.ExportPrivateKey(privateKey)))); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_PRIVATE_KEY: %w", err))
	}
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceApiKeyRead(ctx, d, m)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_CREATE_API_KEY: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_CREATE_API_KEY"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_API_KEY: %w", err))
	}
	return nil
}

func resourceApiKeyDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_API_KEY: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	_, err = clientSet.CloudV1alpha1().APIKeys(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_API_KEY: %w", err))
	}
	err = clientSet.CloudV1alpha1().APIKeys(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_DELETE_API_KEY: %w", err))
	}
	_ = d.Set("name", "")
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

func resourceApiKeyUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_API_KEY: %w", err))
	}
	apiKey, err := clientSet.CloudV1alpha1().APIKeys(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_API_KEY: %w", err))
	}
	revoke := d.Get("revoke").(bool)
	apiKey.Spec.Revoke = revoke
	description := d.Get("description").(string)
	if description != "" {
		apiKey.Spec.Description = description
	}
	_, err = clientSet.CloudV1alpha1().APIKeys(namespace).Update(ctx, apiKey, metav1.UpdateOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_API_KEY: %w", err))
	}
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceApiKeyRead(ctx, d, m)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_UPDATE_API_KEY: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		revokedAt := d.Get("revoked_at")
		if revoke && revokedAt == nil {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_UPDATE_API_KEY"))
		} else if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_UPDATE_API_KEY"))
		}

		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_API_KEY: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", apiKey.Namespace, apiKey.Name))
	return nil
}

func resourceApiKeyRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_API_KEY: %w", err))
	}
	apiKey, err := clientSet.CloudV1alpha1().APIKeys(namespace).Get(ctx, name, metav1.GetOptions{})
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
	if err = d.Set("ready", "False"); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_READY: %w", err))
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
			}
		}
		if apiKey.Status.RevokedAt != nil {
			if err = d.Set("revoked_at", apiKey.Status.RevokedAt.String()); err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_SET_REVOKED_AT: %w", err))
			}
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", apiKey.Namespace, apiKey.Name))
	return nil
}
