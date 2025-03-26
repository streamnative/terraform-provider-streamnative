package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud"
	"github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func resourceVolume() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVolumeCreate,
		ReadContext:   resourceVolumeRead,
		UpdateContext: resourceVolumeUpdate,
		DeleteContext: resourceVolumeDelete,
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
				Description:  descriptions["volume_name"],
				ValidateFunc: validateNotBlank,
			},
			"bucket": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["bucket"],
				ValidateFunc: validateNotBlank,
			},
			"path": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["path"],
				ValidateFunc: validateNotBlank,
			},
			"region": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["bucket_region"],
				ValidateFunc: validateNotBlank,
			},
			"role_arn": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["role_arn"],
				ValidateFunc: validateNotBlank,
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["volume_ready"],
			},
		},
	}
}

func resourceVolumeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	bucket := d.Get("bucket").(string)
	path := d.Get("path").(string)
	region := d.Get("region").(string)
	roleArn := d.Get("role_arn").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_VOLUME: %w", err))
	}
	v := &v1alpha1.Volume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Volume",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.VolumeSpec{
			Bucket: bucket,
			Path:   path,
			Type:   "aws",
			AWS: &v1alpha1.AWSSpec{
				RoleArn: roleArn,
				Region:  region,
			},
		},
	}
	volume, err := clientSet.CloudV1alpha1().Volumes(namespace).Create(ctx, v, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_VOLUME: %w", err))
	}
	if volume.Status.Conditions != nil && len(volume.Status.Conditions) > 0 {
		ready := false
		for _, condition := range volume.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if ready {
			_ = d.Set("organization", namespace)
			_ = d.Set("name", name)
			return resourceVolumeRead(ctx, d, meta)
		}
	}
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceVolumeRead(ctx, d, meta)
		if dia.HasError() {
			return retry.RetryableError(fmt.Errorf("ERROR_READ_VOLUME: %w", dia[0].Summary))
		}
		ready := d.Get("ready").(string)
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_WAITING_VOLUME_READY: volume is not ready yet"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_WAIT_VOLUME_READY: %w", err))
	}
	return nil
}

func resourceVolumeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_VOLUME: %w", err))
	}
	err = clientSet.CloudV1alpha1().Volumes(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		_, err := clientSet.CloudV1alpha1().Volumes(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return retry.RetryableError(fmt.Errorf("ERROR_DELETE_VOLUME: %w", err))
		}
		return retry.RetryableError(fmt.Errorf("CONTINUE_WAITING_VOLUME_DELETE: %s", "volume is not deleted yet"))
	})
	d.SetId("")
	return nil
}

func resourceVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_VOLUME: %w", err))
	}
	_ = d.Set("ready", "False")
	volume, err := clientSet.CloudV1alpha1().Volumes(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_VOLUME: %w", err))
	}
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
	if volume.Status.Conditions != nil && len(volume.Status.Conditions) > 0 {
		for _, condition := range volume.Status.Conditions {
			if condition.Type == "Ready" {
				_ = d.Set("ready", condition.Status)
			}
		}
	}
	return nil
}

func resourceVolumeUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	bucket := d.Get("bucket").(string)
	path := d.Get("path").(string)
	region := d.Get("region").(string)
	roleArn := d.Get("role_arn").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_UPDATE_VOLUME: %w", err))
	}
	volume, err := clientSet.CloudV1alpha1().Volumes(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_GET_VOLUME_ON_UPDATE: %w", err))
	}
	labels := volume.GetLabels()
	if labels != nil {
		if l, ok := labels[cloud.AnnotationVolumeAttachCluster]; ok && l != "" {
			return diag.FromErr(fmt.Errorf(
				"ERROR_UPDATE_VOLUME_ATTACHED_CLUSTER: this volume has been attached one cluster, it don't support update, %w", err))
		}
	}
	volume.Spec.Bucket = bucket
	volume.Spec.Path = path
	volume.Spec.AWS.Region = region
	volume.Spec.AWS.RoleArn = roleArn
	_, err = clientSet.CloudV1alpha1().Volumes(namespace).Update(ctx, volume, metav1.UpdateOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_VOLUME: %w", err))
	}
	err = retry.RetryContext(ctx, 10*time.Minute, func() *retry.RetryError {
		dia := resourceVolumeRead(ctx, d, meta)
		if dia.HasError() {
			return retry.RetryableError(fmt.Errorf("ERROR_READ_VOLUME"))
		}
		ready := d.Get("ready").(string)
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf(
				"CONTINUE_WAITING_VOLUME_READY: volume is not ready yet"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_WAIT_VOLUME_READY: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return nil
}
