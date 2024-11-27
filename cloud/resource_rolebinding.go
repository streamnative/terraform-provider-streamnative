package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

func resourceRoleBinding() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRoleBindingCreate,
		ReadContext:   resourceRoleBindingRead,
		UpdateContext: resourceRoleBindingUpdate,
		DeleteContext: resourceRoleBindingDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChange("name") ||
				diff.HasChange("organization") ||
				diff.HasChange("predefined_role_name") {
				return fmt.Errorf("ERROR_UPDATE_: " +
					"The api key does not support updates organization, " +
					"name, role, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationRoleBinding := strings.Split(d.Id(), "/")
				if err := d.Set("organization", organizationRoleBinding[0]); err != nil {
					return nil, fmt.Errorf("ERROR_IMPORT_ORGANIZATION: %w", err)
				}
				if err := d.Set("name", organizationRoleBinding[1]); err != nil {
					return nil, fmt.Errorf("ERROR_IMPORT_NAME: %w", err)
				}
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
			"predefined_role_name": {
				Type:         schema.TypeString,
				Required:     false,
				Description:  descriptions["rolebinding_predefined_role_name"],
				ValidateFunc: validateNotBlank,
			},
			"service_account_names": {
				Type:         schema.TypeList,
				Required:     false,
				Description:  descriptions["rolebinding_service_account_names"],
				ValidateFunc: validateNotBlank,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					Required:     true,
					Description:  descriptions["rolebinding_service_account_name"],
					ValidateFunc: validateNotBlank,
				},
			},
		},
	}
}

func resourceRoleBindingCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)

	predefinedRoleName := d.Get("predefined_role_name").(string)
	serviceAccountNames := d.Get("service_account_names").([]string)

	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_API_KEY: %w", err))
	}
	rb := &v1alpha1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.RoleBindingSpec{
			Subjects: []v1alpha1.Subject{},
		},
	}

	if predefinedRoleName != "" {
		rb.Spec.RoleRef = v1alpha1.RoleRef{
			Kind: "ClusterRole",
			Name: predefinedRoleName,
		}
	}
	if serviceAccountNames != nil {
		for _, serviceAccountName := range serviceAccountNames {
			rb.Spec.Subjects = append(rb.Spec.Subjects, v1alpha1.Subject{
				Name: serviceAccountName,
				Kind: "ServiceAccount",
			})
		}
	}

	if _, err := clientSet.CloudV1alpha1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{
		FieldManager: "terraform-create",
	}); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_ROLEBINDING: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceRoleBindingRead(ctx, d, m)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_CREATE_ROLEBINDING: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		if ready == false {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_CREATE_ROLEBINDING"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_CREATE_ROLEBINDING: %w", err))
	}
	return nil
}

func resourceRoleBindingDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_ROLEBINDING: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	_, err = clientSet.CloudV1alpha1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_ROLEBINDING: %w", err))
	}
	err = clientSet.CloudV1alpha1().RoleBindings(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_DELETE_ROLEBINDING: %w", err))
	}
	_ = d.Set("name", "")
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

func resourceRoleBindingUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_ROLEBINDING: %w", err))
	}
	roleBinding, err := clientSet.CloudV1alpha1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_ROLEBINDING: %w", err))
	}

	serviceAccountNames := d.Get("service_account_names").([]string)

	if serviceAccountNames != nil {
		for _, serviceAccountName := range serviceAccountNames {
			roleBinding.Spec.Subjects = append(roleBinding.Spec.Subjects, v1alpha1.Subject{
				Name: serviceAccountName,
				Kind: "ServiceAccount",
			})
		}
	}
	_, err = clientSet.CloudV1alpha1().RoleBindings(namespace).Update(ctx, roleBinding, metav1.UpdateOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_ROLEBINDING: %w", err))
	}
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceRoleBindingRead(ctx, d, m)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_UPDATE_ROLEBINDING: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		if ready == false {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_CREATE_ROLEBINDING"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_CREATE_ROLEBINDING: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", roleBinding.Namespace, roleBinding.Name))
	return nil
}

func resourceRoleBindingRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_ROLEBINDING: %w", err))
	}

	roleBinding, err := clientSet.CloudV1alpha1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_ROLEBINDING: %w", err))
	}
	if err = d.Set("organization", namespace); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ORGANIZATION: %w", err))
	}
	if err = d.Set("name", roleBinding.Name); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_NAME: %w", err))
	}
	if err = d.Set("ready", false); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_READY: %w", err))
	}
	if len(roleBinding.Status.Conditions) >= 1 {
		for _, condition := range roleBinding.Status.Conditions {
			if condition.Type == "ready" && condition.Status == "True" {
				if err = d.Set("ready", true); err != nil {
					return diag.FromErr(fmt.Errorf("ERROR_SET_READY: %w", err))
				}
			}
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", roleBinding.Namespace, roleBinding.Name))
	return nil
}
