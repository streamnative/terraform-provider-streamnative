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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourceServiceAccountBinding() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServiceAccountBindingCreate,
		ReadContext:   resourceServiceAccountBindingRead,
		UpdateContext: resourceServiceAccountBindingUpdate,
		DeleteContext: resourceServiceAccountBindingDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChanges("organization") ||
				diff.HasChanges("pool_member_name") ||
				diff.HasChanges("pool_member_namespace") ||
				diff.HasChanges("service_account_name") {
				return fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT_BINDING: " +
					"The service account binding does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationServiceAccount := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationServiceAccount[0])
				_ = d.Set("name", organizationServiceAccount[1])
				err := resourceServiceAccountBindingRead(ctx, d, meta)
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
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["service_account_binding_name"],
			},
			"service_account_name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["service_account_name"],
				ValidateFunc: validateNotBlank,
			},
			"cluster_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["cluster_name"],
			},
			"pool_member_name": {
				Type:        schema.TypeString,
				Description: descriptions["pool_member_name"],
				Computed:    true,
				Optional:    true,
			},
			"pool_member_namespace": {
				Type:        schema.TypeString,
				Description: descriptions["pool_member_namespace"],
				Computed:    true,
				Optional:    true,
			},
			"enable_iam_account_creation": {
				Type:        schema.TypeBool,
				Description: descriptions["enable_iam_account_creation"],
				Optional:    true,
			},
			"aws_assume_role_arns": {
				Type:        schema.TypeList,
				Description: descriptions["aws_assume_role_arns"],
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceServiceAccountBindingCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	serviceAccountName := d.Get("service_account_name").(string)
	clusterName := d.Get("cluster_name").(string)
	poolMemberName := d.Get("pool_member_name").(string)
	poolMemberNamespace := d.Get("pool_member_namespace").(string)
	if poolMemberName == "" && poolMemberNamespace == "" && clusterName == "" {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_SERVICE_ACCOUNT_BINDING: " +
			"either (pool_member_name & pool_member_namespace) or cluster_name must be provided"))
	}
	enableIAMAccountCreation := d.Get("enable_iam_account_creation").(bool)
	awsAssumeRoleARNRawList := d.Get("aws_assume_role_arns").([]interface{})
	awsAssumeRoleARNs := make([]string, len(awsAssumeRoleARNRawList))
	for i, v := range awsAssumeRoleARNRawList {
		awsAssumeRoleARNs[i] = v.(string)
	}

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_SERVICE_ACCOUNT_BINDING: %w", err))
	}

	if clusterName != "" {
		pulsarCluster, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, clusterName, metav1.GetOptions{})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER: %w", err))
		}

		poolMemberNamespace = pulsarCluster.Spec.PoolMemberRef.Namespace
		poolMemberName = pulsarCluster.Spec.PoolMemberRef.Name
	}

	name := fmt.Sprintf("%s.%s.%s", serviceAccountName, poolMemberNamespace, poolMemberName)
	sab := &v1alpha1.ServiceAccountBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ServiceAccountBindingSpec{
			ServiceAccountName: serviceAccountName,
			PoolMemberRef: v1alpha1.PoolMemberReference{
				Name:      poolMemberName,
				Namespace: poolMemberNamespace,
			},
			EnableIAMAccountCreation: enableIAMAccountCreation,
			AWSAssumeRoleARNs:        awsAssumeRoleARNs,
		},
	}
	serviceAccountBinding, err := clientSet.CloudV1alpha1().ServiceAccountBindings(namespace).Create(ctx, sab, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	_ = d.Set("name", serviceAccountBinding.Name)
	// Don't retry too frequently to avoid affecting the api-server.
	err = retry.RetryContext(ctx, 5*time.Second, func() *retry.RetryError {
		dia := resourceServiceAccountBindingRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_CREATE_SERVICE_ACCOUNT_BINDING: %s", dia[0].Summary))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	return nil
}

func resourceServiceAccountBindingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	serviceAccountBinding, err := clientSet.CloudV1alpha1().ServiceAccountBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	_ = d.Set("name", serviceAccountBinding.Name)
	_ = d.Set("organization", serviceAccountBinding.Namespace)
	_ = d.Set("service_account_name", serviceAccountBinding.Spec.ServiceAccountName)
	_ = d.Set("pool_member_name", serviceAccountBinding.Spec.PoolMemberRef.Name)
	_ = d.Set("pool_member_namespace", serviceAccountBinding.Spec.PoolMemberRef.Namespace)
	_ = d.Set("enable_iam_account_creation", serviceAccountBinding.Spec.EnableIAMAccountCreation)
	_ = d.Set("aws_assume_role_arns", flattenStringSlice(serviceAccountBinding.Spec.AWSAssumeRoleARNs))
	d.SetId(fmt.Sprintf("%s/%s", serviceAccountBinding.Namespace, serviceAccountBinding.Name))

	return nil
}

func resourceServiceAccountBindingDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	err = clientSet.CloudV1alpha1().ServiceAccountBindings(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("DELETE_SERVICE_ACCOUNT_BINDING: %w", err))
	}
	_ = d.Set("name", "")
	return nil
}

func resourceServiceAccountBindingUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.FromErr(fmt.Errorf("ERROR_UPDATE_SERVICE_ACCOUNT_BINDING: " +
		"The service account binding does not support updates, please recreate it"))
}
