package cloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dataSourceSecret() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceSecretRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(
				ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				parts := strings.Split(d.Id(), "/")
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid import id %q, expected <organization>/<name>", d.Id())
				}
				_ = d.Set("organization", parts[0])
				_ = d.Set("name", parts[1])
				if diags := dataSourceSecretRead(ctx, d, meta); diags.HasError() {
					return nil, fmt.Errorf("import %q: %s", d.Id(), diags[0].Summary)
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
				Description:  descriptions["secret_name"],
				ValidateFunc: validateNotBlank,
			},
			"instance_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["instance_name"],
			},
			"location": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["location"],
			},
			"pool_member_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pool_member_name"],
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["secret_type"],
			},
			"data": {
				Type:        schema.TypeMap,
				Computed:    true,
				Sensitive:   true,
				Description: descriptions["secret_data"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
