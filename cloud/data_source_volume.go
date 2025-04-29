package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dataSourceVolume() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceVolumeRead,
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
				Description:  descriptions["volume_name"],
				ValidateFunc: validateNotBlank,
			},
			"bucket": {
				Type:        schema.TypeString,
				Description: descriptions["bucket"],
				Computed:    true,
			},
			"path": {
				Type:        schema.TypeString,
				Description: descriptions["path"],
				Computed:    true,
			},
			"region": {
				Type:        schema.TypeString,
				Description: descriptions["bucket_region"],
				Computed:    true,
			},
			"role_arn": {
				Type:        schema.TypeString,
				Description: descriptions["role_arn"],
				Computed:    true,
			},
		},
	}
}

func dataSourceVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_VOLUME: %w", err))
	}
	volume, err := clientSet.CloudV1alpha1().Volumes(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_VOLUME: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	if err = d.Set("organization", volume.Namespace); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ORGANIZATION: %w", err))
	}
	if err = d.Set("name", volume.Name); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_NAME: %w", err))
	}
	if err = d.Set("bucket", volume.Spec.Bucket); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_BUCKET: %w", err))
	}
	if err = d.Set("path", volume.Spec.Path); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_PATH: %w", err))
	}
	if err = d.Set("region", volume.Spec.AWS.Region); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_REGION: %w", err))
	}
	if err = d.Set("role_arn", volume.Spec.AWS.RoleArn); err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_SET_ROLE_ARN: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", volume.Namespace, volume.Name))
	return nil
}
