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
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
)

func resourceSecret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecretCreate,
		ReadContext:   resourceSecretRead,
		UpdateContext: resourceSecretUpdate,
		DeleteContext: resourceSecretDelete,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				parts := strings.Split(d.Id(), "/")
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid import id %q, expected <organization>/<name>", d.Id())
				}
				_ = d.Set("organization", parts[0])
				_ = d.Set("name", parts[1])
				if diags := resourceSecretRead(ctx, d, meta); diags.HasError() {
					return nil, fmt.Errorf("import %q: %s", d.Id(), diags[0].Summary)
				}
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  descriptions["organization"],
				ValidateFunc: validateNotBlank,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  descriptions["secret_name"],
				ValidateFunc: validateNotBlank,
			},
			"instance_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: descriptions["instance_name"],
			},
			"location": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: descriptions["location"],
			},
			"pool_member_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: descriptions["pool_member_name"],
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: descriptions["secret_type"],
			},
			"data": {
				Type:         schema.TypeMap,
				Optional:     true,
				Computed:     true,
				Sensitive:    true,
				AtLeastOneOf: []string{"data", "string_data"},
				Description:  descriptions["secret_data"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"string_data": {
				Type:         schema.TypeMap,
				Optional:     true,
				Sensitive:    true,
				AtLeastOneOf: []string{"data", "string_data"},
				Description:  descriptions["secret_string_data"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceSecretCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_SECRET: %w", err))
	}

	secret := buildSecretFromResourceData(d)
	created, err := clientSet.CloudV1alpha1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_SECRET: %w", err))
	}

	d.SetId(fmt.Sprintf("%s/%s", created.Namespace, created.Name))
	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SECRET: %w", err))
	}

	secret, err := clientSet.CloudV1alpha1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_SECRET: %w", err))
	}

	return setSecretState(d, secret)
}

func resourceSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_UPDATE_SECRET: %w", err))
	}

	secret, err := clientSet.CloudV1alpha1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_GET_SECRET_ON_UPDATE: %w", err))
	}

	applySecretPlan(secret, d, false)
	updated, err := clientSet.CloudV1alpha1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{
		FieldManager: "terraform-update",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_SECRET: %w", err))
	}

	d.SetId(fmt.Sprintf("%s/%s", updated.Namespace, updated.Name))
	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_SECRET: %w", err))
	}

	if err := clientSet.CloudV1alpha1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_DELETE_SECRET: %w", err))
	}

	d.SetId("")
	return nil
}

func buildSecretFromResourceData(d *schema.ResourceData) *v1alpha1.Secret {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	secret := &v1alpha1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	applySecretPlan(secret, d, true)
	return secret
}

func applySecretPlan(secret *v1alpha1.Secret, d *schema.ResourceData, includeUnset bool) {
	secret.InstanceName = d.Get("instance_name").(string)
	secret.Location = d.Get("location").(string)

	poolMemberName := d.Get("pool_member_name").(string)
	if poolMemberName != "" {
		secret.PoolMemberRef = &v1alpha1.PoolMemberReference{
			Name:      poolMemberName,
			Namespace: secret.Namespace,
		}
	} else if includeUnset || d.HasChange("pool_member_name") {
		secret.PoolMemberRef = nil
	}

	if includeUnset || d.HasChange("type") {
		secretType := d.Get("type").(string)
		if secretType != "" {
			t := corev1.SecretType(secretType)
			secret.Type = &t
		} else {
			secret.Type = nil
		}
	}

	if includeUnset || d.HasChange("data") {
		if dataRaw, ok := d.GetOk("data"); ok {
			secret.Data = convertToStringMap(dataRaw.(map[string]interface{}))
		} else {
			secret.Data = nil
		}
	}

	if includeUnset || d.HasChange("string_data") {
		if stringDataRaw, ok := d.GetOk("string_data"); ok {
			secret.StringData = convertToStringMap(stringDataRaw.(map[string]interface{}))
		} else {
			secret.StringData = nil
		}
	}
}

func setSecretState(d *schema.ResourceData, secret *v1alpha1.Secret) diag.Diagnostics {
	if err := d.Set("organization", secret.Namespace); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ORGANIZATION: %w", err))
	}
	if err := d.Set("name", secret.Name); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_NAME: %w", err))
	}
	if err := d.Set("instance_name", secret.InstanceName); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_INSTANCE_NAME: %w", err))
	}
	if err := d.Set("location", secret.Location); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_LOCATION: %w", err))
	}
	if err := d.Set("data", secret.Data); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_DATA: %w", err))
	}

	if secret.PoolMemberRef != nil {
		if err := d.Set("pool_member_name", secret.PoolMemberRef.Name); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_POOL_MEMBER_NAME: %w", err))
		}
	} else {
		if err := d.Set("pool_member_name", ""); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_RESET_POOL_MEMBER_NAME: %w", err))
		}
	}

	if secret.Type != nil {
		if err := d.Set("type", string(*secret.Type)); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_TYPE: %w", err))
		}
	} else {
		if err := d.Set("type", ""); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_RESET_TYPE: %w", err))
		}
	}

	// Preserve user-supplied string_data without attempting to read it from the API server.
	if stringData, ok := d.GetOk("string_data"); ok {
		if err := d.Set("string_data", stringData); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_STRING_DATA: %w", err))
		}
	}

	d.SetId(fmt.Sprintf("%s/%s", secret.Namespace, secret.Name))
	return nil
}
