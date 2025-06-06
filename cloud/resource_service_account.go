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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourceServiceAccount() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServiceAccountCreate,
		ReadContext:   resourceServiceAccountRead,
		UpdateContext: resourceServiceAccountUpdate,
		DeleteContext: resourceServiceAccountDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("organization") ||
				diff.HasChanges("admin") {
				return fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT: " +
					"The service account does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationServiceAccount := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationServiceAccount[0])
				_ = d.Set("name", organizationServiceAccount[1])
				err := resourceServiceAccountRead(ctx, d, meta)
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
				Description:  descriptions["service_account_name"],
				ValidateFunc: validateNotBlank,
			},
			"admin": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: descriptions["admin"],
			},
			"private_key_data": {
				Type:        schema.TypeString,
				Description: descriptions["private_key_data"],
				Computed:    true,
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},
	}
}

func resourceServiceAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	admin := d.Get("admin").(bool)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_SERVICE_ACCOUNT: %w", err))
	}
	sa := &v1alpha1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if admin {
		sa.ObjectMeta.Annotations = map[string]string{
			ServiceAccountAdminAnnotation: "admin",
		}
	}
	serviceAccount, err := clientSet.CloudV1alpha1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_SERVICE_ACCOUNT: %w", err))
	}

	if admin {
		_, err := clientSet.CloudV1alpha1().RoleBindings(namespace).Create(ctx, &v1alpha1.RoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "RoleBinding",
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v1alpha1.SchemeGroupVersion.String(),
						Kind:       "ServiceAccount",
						Name:       serviceAccount.Name,
						UID:        serviceAccount.UID,
					},
				},
			},
			Spec: v1alpha1.RoleBindingSpec{
				RoleRef: v1alpha1.RoleRef{
					APIGroup: "cloud.streamnative.io",
					Kind:     "Role",
					Name:     "admin",
				},
				Subjects: []v1alpha1.Subject{
					{
						Kind:     "ServiceAccount",
						APIGroup: "cloud.streamnative.io",
						Name:     name,
					},
				},
			},
		}, metav1.CreateOptions{
			FieldManager: "terraform-create",
		})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_ROLE_BINDING: %w", err))
		}
	}
	privateKeyData := ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = serviceAccount.Status.PrivateKeyData
		_ = d.Set("name", serviceAccount.Name)
		_ = d.Set("organization", serviceAccount.Namespace)
		_ = d.Set("private_key_data", privateKeyData)
		d.SetId(fmt.Sprintf("%s/%s", serviceAccount.Namespace, serviceAccount.Name))
	}

	err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *retry.RetryError {
		//Sleep 20 seconds between checks so we don't overload the API
		time.Sleep(time.Second * 20)

		dia := resourceServiceAccountRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_CREATE_SERVICE_ACCOUNT: %s", dia[0].Summary))
		}
		pkd := d.Get("private_key_data")
		if pkd == "" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_CREATE_SERVICE_ACCOUNT"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_SERVICE_ACCOUNT: %w", err))
	}
	return nil
}

func resourceServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT: %w", err))
	}
	serviceAccount, err := clientSet.CloudV1alpha1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT: %w", err))
	}
	_ = d.Set("name", serviceAccount.Name)
	_ = d.Set("organization", serviceAccount.Namespace)
	var privateKeyData = ""
	if len(serviceAccount.Status.Conditions) > 0 && serviceAccount.Status.Conditions[0].Type == "Ready" {
		privateKeyData = serviceAccount.Status.PrivateKeyData
	}
	_ = d.Set("private_key_data", privateKeyData)
	d.SetId(fmt.Sprintf("%s/%s", serviceAccount.Namespace, serviceAccount.Name))

	return nil
}

func resourceServiceAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_SERVICE_ACCOUNT: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	foreground := metav1.DeletePropagationForeground
	err = clientSet.CloudV1alpha1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &foreground,
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_SERVICE_ACCOUNT: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}

func resourceServiceAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT: " +
		"The service account does not support updates, please recreate it"))
}
