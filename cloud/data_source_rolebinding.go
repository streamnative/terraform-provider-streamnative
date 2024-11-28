package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func dataSourceRoleBinding() *schema.Resource {
	return &schema.Resource{
		ReadContext: DataSourceRoleBindingRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(
				ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationRoleBinding := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationRoleBinding[0])
				_ = d.Set("name", organizationRoleBinding[1])
				err := resourceRoleBindingRead(ctx, d, meta)
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
				Description:  descriptions["rolebinding_name"],
				ValidateFunc: validateNotBlank,
			},
			"ready": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: descriptions["rolebinding_ready"],
			},
			"cluster_role_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["rolebinding_cluster_role_name"],
			},
			"service_account_names": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["rolebinding_service_account_names"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func DataSourceRoleBindingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	organization := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_ROLEBINDING: %w", err))
	}
	roleBinding, err := clientSet.CloudV1alpha1().RoleBindings(organization).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_ROLEBINDING: %w", err))
	}
	if err = d.Set("organization", roleBinding.Namespace); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ORGANIZATION: %w", err))
	}
	if err = d.Set("name", roleBinding.Name); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_NAME: %w", err))
	}

	if roleBinding.Spec.RoleRef.Kind == "ClusterRole" {
		if err = d.Set("cluster_role_name", roleBinding.Spec.RoleRef.Name); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_CLUSTER_ROLE_NAME: %w", err))
		}
	}

	var serviceAccountNames []string
	for _, subject := range roleBinding.Spec.Subjects {
		if subject.Kind == "ServiceAccount" {
			serviceAccountNames = append(serviceAccountNames, subject.Name)
		}
	}
	if serviceAccountNames != nil {
		if err = d.Set("service_account_names", serviceAccountNames); err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_SET_SERVICE_ACCOUNT_NAMES: %w", err))
		}
	}

	if len(roleBinding.Status.Conditions) >= 1 {
		for _, condition := range roleBinding.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				if err = d.Set("ready", true); err != nil {
					return diag.FromErr(fmt.Errorf("ERROR_SET_READY: %w", err))
				}
			}
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", roleBinding.Namespace, roleBinding.Name))
	return nil
}
