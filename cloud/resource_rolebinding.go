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
	"github.com/streamnative/terraform-provider-streamnative/cloud/rbac"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				diff.HasChange("cluster_role_name") {
				return fmt.Errorf("ERROR_UPDATE_: " +
					"The rolebinding does not support updates organization, " +
					"name, cluster_role_name, please recreate it")
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
			"cluster_role_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["rolebinding_cluster_role_name"],
			},
			"service_account_names": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: descriptions["rolebinding_service_account_names"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"user_names": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: descriptions["rolebinding_user_names"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"resource_name_restriction": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: rbac.GenerateResourceRoleBinding(),
				},
			},
			"condition_resource_names": {
				ConflictsWith: []string{"condition_cel"},
				Type:          schema.TypeList,
				Optional:      true,
				Description:   descriptions["rolebinding_condition_resource_names"],
				Deprecated:    "condition_resource_names has deprecated, please use resource_name_restriction instead.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"organization": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_organization"],
						},
						"instance": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_instance"],
						},
						"cluster": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_cluster"],
						},
						"tenant": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_tenant"],
						},
						"namespace": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_namespace"],
						},
						"topic_domain": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_topic_domain"],
						},
						"topic_name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_topic_name"],
						},
						"subscription": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_subscription"],
						},
						"service_account": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_service_account"],
						},
						"secret": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: descriptions["rolebinding_condition_resource_names_secret"],
						},
					},
				},
			},
			"condition_cel": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   descriptions["rolebinding_condition_cel"],
				ConflictsWith: []string{"condition_resource_names"},
			},
		},
	}
}

func resourceRoleBindingCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)

	predefinedRoleName := d.Get("cluster_role_name").(string)
	serviceAccountNames := d.Get("service_account_names").([]interface{})
	userNames := d.Get("user_names").([]interface{})
	resourceNameRestriction := d.Get("resource_name_restriction").([]interface{})

	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_ROLEBINDING: %w", err))
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
			APIGroup: "cloud.streamnative.io",
			Kind:     "ClusterRole",
			Name:     predefinedRoleName,
		}
	}
	if serviceAccountNames != nil {
		for _, serviceAccountName := range serviceAccountNames {
			rb.Spec.Subjects = append(rb.Spec.Subjects, v1alpha1.Subject{
				APIGroup: "cloud.streamnative.io",
				Name:     serviceAccountName.(string),
				Kind:     "ServiceAccount",
			})
		}
	}

	if userNames != nil {
		for _, userName := range userNames {
			rb.Spec.Subjects = append(rb.Spec.Subjects, v1alpha1.Subject{
				APIGroup: "cloud.streamnative.io",
				Name:     userName.(string),
				Kind:     "User",
			})
		}
	}

	if resourceNameRestriction != nil && len(resourceNameRestriction) > 0 {
		if restriction, updated := rbac.ParseToResourceNameRestriction(resourceNameRestriction[0].(map[string]interface{})); updated {
			rb.Spec.ResourceNameRestriction = restriction
		}
	}

	conditionSet(namespace, d, rb)

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
	userNames := d.Get("user_names").([]interface{})
	clientSet, err := getClientSet(getFactoryFromMeta(m))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_ROLEBINDING: %w", err))
	}
	roleBinding, err := clientSet.CloudV1alpha1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_ROLEBINDING: %w", err))
	}

	serviceAccountNames := d.Get("service_account_names").([]interface{})

	roleBinding.Spec.Subjects = []v1alpha1.Subject{}

	if serviceAccountNames != nil {
		for _, serviceAccountName := range serviceAccountNames {
			roleBinding.Spec.Subjects = append(roleBinding.Spec.Subjects, v1alpha1.Subject{
				APIGroup: "cloud.streamnative.io",
				Name:     serviceAccountName.(string),
				Kind:     "ServiceAccount",
			})
		}
	}
	if userNames != nil {
		for _, userName := range userNames {
			roleBinding.Spec.Subjects = append(roleBinding.Spec.Subjects, v1alpha1.Subject{
				APIGroup: "cloud.streamnative.io",
				Name:     userName.(string),
				Kind:     "User",
			})
		}
	}

	conditionSet(namespace, d, roleBinding)
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

func conditionSet(organization string, d *schema.ResourceData, binding *v1alpha1.RoleBinding) {
	cel, exist := d.GetOk("condition_cel")
	if exist {
		celExpression := cel.(string)
		binding.Spec.CEL = &celExpression
	}

	resourceNames := d.Get("condition_resource_names")
	if resourceNames != nil {
		var bindingResourceNames []v1alpha1.ResourceName
		resourceNamesEntity := resourceNames.([]interface{})
		for idx := range resourceNamesEntity {
			resourceName := resourceNamesEntity[idx]
			resourceElements := resourceName.(map[string]interface{})
			bindingResourceNames = append(bindingResourceNames, v1alpha1.ResourceName{
				Organization:   organization,
				Instance:       resourceElements["instance"].(string),
				Cluster:        resourceElements["cluster"].(string),
				Tenant:         resourceElements["tenant"].(string),
				Namespace:      resourceElements["namespace"].(string),
				TopicDomain:    resourceElements["topic_domain"].(string),
				TopicName:      resourceElements["topic_name"].(string),
				Subscription:   resourceElements["subscription"].(string),
				ServiceAccount: resourceElements["service_account"].(string),
				Secret:         resourceElements["secret"].(string),
			})
		}
		binding.Spec.ResourceNames = bindingResourceNames
	}
}
