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
	"math"
	"strings"
	"time"

	"k8s.io/utils/pointer"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	cloudclient "github.com/streamnative/cloud-api-server/pkg/client/clientset_generated/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func resourcePulsarCluster() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePulsarClusterCreate,
		ReadContext:   resourcePulsarClusterRead,
		UpdateContext: resourcePulsarClusterUpdate,
		DeleteContext: resourcePulsarClusterDelete,
		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, i interface{}) error {
			oldOrg, _ := diff.GetChange("organization")
			oldName, newName := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				// But we still need to check if bookie_replicas should be suppressed for serverless/ursa
				suppressBookieForServerlessOrUrsa(ctx, diff, i)
				// For serverless clusters, make lakehouse_storage_enabled computed
				makeLakehouseStorageComputedForServerless(ctx, diff, i)
				return nil
			}
			if oldName != "" && newName == "" {
				// Auto generate the name, so we don't need to check the diff.
				return nil
			}
			if diff.HasChanges([]string{"organization", "name", "instance_name", "location", "pool_member_name", "release_channel"}...) {
				return fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
					"The pulsar cluster organization, name, instance_name, location, pool_member_name does not support updates, please recreate it")
			}
			// Suppress bookie_replicas changes for serverless or ursa clusters
			suppressBookieForServerlessOrUrsa(ctx, diff, i)
			// For serverless clusters, make lakehouse_storage_enabled computed
			makeLakehouseStorageComputedForServerless(ctx, diff, i)
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				err := resourcePulsarClusterRead(ctx, d, meta)
				if err.HasError() {
					return nil, fmt.Errorf("import %q: %s", d.Id(), err[0].Summary)
				}
				return []*schema.ResourceData{d}, nil
			},
		},
		Timeouts: &schema.ResourceTimeout{
			// Pulsar clusters can take time to tear down; allow 30m to avoid spurious test failures.
			Delete: schema.DefaultTimeout(30 * time.Minute),
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
				Optional:    true,
				Description: descriptions["cluster_name"],
			},
			"display_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["cluster_display_name"],
			},
			"instance_name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["instance_name"],
				ValidateFunc: validateNotBlank,
			},
			"location": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  descriptions["location"],
				ValidateFunc: validateNotBlank,
			},
			"pool_member_name": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  descriptions["pool_member_name"],
				ValidateFunc: validateNotBlank,
			},
			"release_channel": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "rapid",
				Description:  descriptions["release_channel"],
				ValidateFunc: validateReleaseChannel,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"bookie_replicas": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      3,
				Description:  descriptions["bookie_replicas"],
				ValidateFunc: validateBookieReplicas,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"broker_replicas": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      2,
				Description:  descriptions["broker_replicas"],
				ValidateFunc: validateBrokerReplicas,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"compute_unit": {
				Deprecated:   "Deprecated. Please use compute_unit_per_broker instead.",
				Type:         schema.TypeFloat,
				Optional:     true,
				Default:      0.5,
				Description:  descriptions["compute_unit_per_broker"],
				ValidateFunc: validateCUSU,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"compute_unit_per_broker": {
				Type:         schema.TypeFloat,
				Optional:     true,
				Default:      0.5,
				Description:  descriptions["compute_unit_per_broker"],
				ValidateFunc: validateCUSU,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"storage_unit": {
				Deprecated:   "Deprecated. Please use storage_unit_per_bookie instead.",
				Type:         schema.TypeFloat,
				Optional:     true,
				Default:      0.5,
				Description:  descriptions["storage_unit_per_bookie"],
				ValidateFunc: validateCUSU,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"storage_unit_per_bookie": {
				Type:         schema.TypeFloat,
				Optional:     true,
				Default:      0.5,
				Description:  descriptions["storage_unit_per_bookie"],
				ValidateFunc: validateCUSU,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
			},
			"volume": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["volume_name"],
			},
			"config": {
				Type:     schema.TypeList,
				Optional: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("type") == string(cloudv1alpha1.PulsarInstanceTypeServerless)
				},
				MinItems: 0,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"websocket_enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
						"function_enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: descriptions["function_enabled"],
						},
						"transaction_enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: descriptions["transaction_enabled"],
						},
						"protocols": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: descriptions["protocols"],
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"kafka": {
										Type:        schema.TypeMap,
										Default:     map[string]interface{}{},
										Optional:    true,
										Description: descriptions["kafka"],
									},
									"mqtt": {
										Type:        schema.TypeMap,
										Optional:    true,
										Default:     map[string]interface{}{},
										Description: descriptions["mqtt"],
									},
								},
							},
						},
						"audit_log": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: descriptions["audit_log"],
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"categories": {
										Type:        schema.TypeList,
										Optional:    true,
										MinItems:    1,
										Description: descriptions["categories"],
										Elem: &schema.Schema{
											Type:         schema.TypeString,
											ValidateFunc: validateAuditLog,
										},
									},
								},
							},
						},
						"custom": {
							Type:        schema.TypeMap,
							Optional:    true,
							Description: descriptions["custom"],
						},
					},
				},
			},
			"endpoint_access": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"gateway": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "default",
						},
					},
				},
			},
			"ready": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["cluster_ready"],
			},
			"http_tls_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["http_tls_service_url"],
			},
			"http_tls_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["http_tls_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"pulsar_tls_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pulsar_tls_service_url"],
			},
			"pulsar_tls_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["pulsar_tls_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"kafka_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["kafka_service_url"],
			},
			"kafka_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["kafka_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"mqtt_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["mqtt_service_url"],
			},
			"mqtt_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["mqtt_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"websocket_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["websocket_service_url"],
			},
			"websocket_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["websocket_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"pulsar_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pulsar_version"],
			},
			"bookkeeper_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["bookkeeper_version"],
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["instance_type"],
			},
			"catalog": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["catalog"],
			},
			"lakehouse_storage_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: descriptions["lakehouse_storage"],
			},
			"apply_lakehouse_to_all_topics": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: descriptions["apply_lakehouse_to_all_topics"],
			},
			"iam_policy": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["iam_policy"],
			},
			"maintenance_window": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Description: "Maintenance window configuration for the pulsar cluster",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"window": {
							Type:        schema.TypeList,
							Optional:    true,
							Computed:    true,
							Description: "Maintenance execution window",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"start_time": {
										Type:        schema.TypeString,
										Optional:    true,
										Computed:    true,
										Description: "Start time of the maintenance window",
									},
									"duration": {
										Type:         schema.TypeString,
										Optional:     true,
										Computed:     true,
										Description:  "Duration of the maintenance window in Go duration format (e.g., \"2h0m0s\", \"30m0s\", \"1h30m0s\")",
										ValidateFunc: validateDuration,
									},
								},
							},
						},
						"recurrence": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Recurrence pattern for maintenance (0-6 for Monday to Sunday)",
						},
					},
				},
			},
		},
	}
}

func resourcePulsarClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	displayName := d.Get("display_name").(string)
	instanceName := d.Get("instance_name").(string)
	pool_member_name := d.Get("pool_member_name").(string)
	location := d.Get("location").(string)
	if pool_member_name == "" && location == "" {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
			"either pool_member_name or location must be provided"))
	}
	releaseChannel := d.Get("release_channel").(string)
	bookieReplicas := int32(d.Get("bookie_replicas").(int))
	brokerReplicas := int32(d.Get("broker_replicas").(int))
	computeUnit := getComputeUnit(d)
	storageUnit := getStorageUnit(d)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_CREATE_PULSAR_CLUSTER: %w", err))
	}
	pulsarInstance, err := clientSet.CloudV1alpha1().
		PulsarInstances(namespace).
		Get(ctx, instanceName, metav1.GetOptions{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PulsarInstance",
				APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
			},
		})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_GET_PULSAR_INSTANCE_ON_CREATE_PULSAR_CLUSTER: %w", err))
	}
	ursaEngine, ok := pulsarInstance.Annotations[UrsaEngineAnnotation]
	ursaEnabled := ok && ursaEngine == UrsaEngineValue
	bookieCPU := resource.NewMilliQuantity(int64(storageUnit*2*1000), resource.DecimalSI)
	brokerCPU := resource.NewMilliQuantity(int64(computeUnit*2*1000), resource.DecimalSI)
	brokerMem := resource.NewQuantity(int64(computeUnit*8*1024*1024*1024), resource.DecimalSI)
	bookieMem := resource.NewQuantity(int64(storageUnit*8*1024*1024*1024), resource.DecimalSI)

	if pool_member_name != "" {
		// only allow BYOC user to select specific pool member
		poolMember, err := clientSet.CloudV1alpha1().
			PoolMembers(namespace).
			Get(ctx, pool_member_name, metav1.GetOptions{})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_GET_POOL_MEMBER_ON_CREATE_PULSAR_CLUSTER: %w", err))
		}
		if poolMember.Spec.PoolName != pulsarInstance.Spec.PoolRef.Name {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
				"the pool member does not belong to the pool which pulsar instance is attached"))
		}
	}

	pulsarCluster := &cloudv1alpha1.PulsarCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PulsarCluster",
			APIVersion: cloudv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName:   instanceName,
			Location:       location,
			ReleaseChannel: releaseChannel,
			Broker: cloudv1alpha1.Broker{
				Replicas: &brokerReplicas,
				Resources: &cloudv1alpha1.DefaultNodeResource{
					Cpu:    brokerCPU,
					Memory: brokerMem,
				},
			},
		},
	}
	bookkeeper := &cloudv1alpha1.BookKeeper{
		Replicas: &bookieReplicas,
		Resources: &cloudv1alpha1.BookkeeperNodeResource{
			DefaultNodeResource: cloudv1alpha1.DefaultNodeResource{
				Cpu:    bookieCPU,
				Memory: bookieMem,
			},
		},
	}
	if name != "" {
		pulsarCluster.ObjectMeta.Name = name
	}
	if displayName != "" {
		pulsarCluster.Spec.DisplayName = displayName
	}
	if pulsarInstance.IsServerless() {
		if computeUnit != 0.5 {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
				"compute_unit must be 0.5 for serverless instance"))
		}
		if brokerReplicas != 2 {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
				"broker_replicas must be 2 for serverless instance"))
		}
		pulsarCluster.Annotations = map[string]string{
			"cloud.streamnative.io/type": "serverless",
		}
	}
	if ursaEnabled {
		if pulsarCluster.Annotations == nil {
			pulsarCluster.Annotations = map[string]string{
				UrsaEngineAnnotation: UrsaEngineValue,
			}
		} else {
			pulsarCluster.Annotations[UrsaEngineAnnotation] = UrsaEngineValue
		}
		volumeName := d.Get("volume").(string)
		if volumeName != "" {
			_, err := clientSet.CloudV1alpha1().Volumes(namespace).Get(ctx, volumeName, metav1.GetOptions{})
			if err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_GET_VOLUME_ON_CREATE_PULSAR_CLUSTER: %w", err))
			}
			pulsarCluster.Spec.Volume = &cloudv1alpha1.VolumeReference{
				Name: volumeName,
			}
		}
	}
	if ursaEnabled || pulsarInstance.IsServerless() {
		if pulsarCluster.Spec.ReleaseChannel != "" && pulsarCluster.Spec.ReleaseChannel != "rapid" {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
				"release_channel must be rapid for ursa engine or serverless instance"))
		}
	}
	if !ursaEnabled && !pulsarInstance.IsServerless() {
		pulsarCluster.Spec.BookKeeper = bookkeeper
	}
	if pool_member_name != "" {
		pulsarCluster.Spec.PoolMemberRef = cloudv1alpha1.PoolMemberReference{
			Name:      pool_member_name,
			Namespace: namespace,
		}
	} else {
		pulsarCluster.Spec.Location = location
	}
	pulsarCluster.Spec.EndpointAccess = convertEndpointAccess(d.Get("endpoint_access"))
	for _, endpoint := range pulsarCluster.Spec.EndpointAccess {
		if endpoint.Gateway != "default" {
			_, err := clientSet.CloudV1alpha1().PulsarGateways(namespace).Get(ctx, endpoint.Gateway, metav1.GetOptions{})
			if err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_GET_PULSAR_GATEWAY_ON_CREATE_PULSAR_CLUSTER: %w", err))
			}
		}
	}
	if pulsarInstance.Spec.Type != cloudv1alpha1.PulsarInstanceTypeServerless && !pulsarInstance.IsUsingUrsaEngine() {
		getPulsarClusterChanged(ctx, pulsarCluster, d)
	}

	// Handle lakehouse_storage_enabled
	if pulsarInstance.IsServerless() {
		// For serverless clusters, automatically enable lakehouse storage
		if pulsarCluster.Spec.Config == nil {
			pulsarCluster.Spec.Config = &cloudv1alpha1.Config{}
		}
		pulsarCluster.Spec.Config.LakehouseStorage = &cloudv1alpha1.LakehouseStorageConfig{
			Enabled: pointer.Bool(true),
		}
	} else {
		// For non-serverless clusters, check user input
		if d.Get("lakehouse_storage_enabled").(bool) {
			if ursaEnabled {
				return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
					"you don't set this option for ursa engine cluster"))
			}
			if pulsarCluster.Spec.Config == nil {
				pulsarCluster.Spec.Config = &cloudv1alpha1.Config{}
			}
			pulsarCluster.Spec.Config.LakehouseStorage = &cloudv1alpha1.LakehouseStorageConfig{
				Enabled: pointer.Bool(true),
			}
		}
	}

	// Handle catalog configuration
	catalogName := d.Get("catalog").(string)
	var catalog *cloudv1alpha1.Catalog
	if catalogName != "" {
		// Get catalog information
		catalog, err = clientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, catalogName, metav1.GetOptions{})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_GET_CATALOG: %w", err))
		}

		// Check if it's an S3Table catalog
		if catalog.Spec.S3Table != nil {
			// Validate region match
			if err := validateCatalogRegionMatch(ctx, clientSet, namespace, catalogName, location); err != nil {
				return diag.FromErr(err)
			}
		}

		// Add catalog to the cluster
		pulsarCluster.Spec.Catalogs = []string{catalogName}

		// Determine table format based on catalog and lakehouse storage
		lakehouseStorageEnabled := false
		if pulsarCluster.Spec.Config != nil &&
			pulsarCluster.Spec.Config.LakehouseStorage != nil &&
			pulsarCluster.Spec.Config.LakehouseStorage.Enabled != nil &&
			*pulsarCluster.Spec.Config.LakehouseStorage.Enabled {
			lakehouseStorageEnabled = true
		}

		if (lakehouseStorageEnabled || ursaEnabled) && catalog != nil {
			tableFormat, err := determineTableFormat(ctx, clientSet, namespace, catalogName)
			if err != nil {
				return diag.FromErr(fmt.Errorf("ERROR_DETERMINE_TABLE_FORMAT: %w", err))
			}
			pulsarCluster.Spec.TableFormat = tableFormat
		}
	}

	// Handle SDT annotation based on apply_lakehouse_to_all_topics
	if shouldApplyLakehouseToAllTopics(d) {
		if ursaEnabled {
			return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: " +
				"you don't set this apply_lakehouse_to_all_topics option for ursa engine cluster"))
		}
		if pulsarCluster.Annotations == nil {
			pulsarCluster.Annotations = make(map[string]string)
		}
		pulsarCluster.Annotations["cloud.streamnative.io/sdt-enabled"] = "true"
	}

	pc, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Create(ctx, pulsarCluster, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", pc.Namespace, pc.Name))

	// Generate and set IAM policy if catalog is configured
	if catalog != nil && catalog.Spec.S3Table != nil {
		// Try to get account ID from pool options using instance pool information
		var accountID string
		if pool_member_name != "" || location != "" {
			accountIDFromPool, err := getAccountIDFromPoolOptions(
				ctx, clientSet,
				pulsarCluster.Namespace,
				fmt.Sprintf("%s-%s", pulsarInstance.Spec.PoolRef.Namespace, pulsarInstance.Spec.PoolRef.Name),
				location,
				pool_member_name)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Failed to get account ID from pool options: %v", err))
				tflog.Warn(ctx, "Using placeholder account ID in IAM policy")
			} else {
				accountID = accountIDFromPool
				tflog.Info(ctx, fmt.Sprintf("Retrieved account ID from pool options: %s", accountID))
			}
		}

		// Get S3Table warehouse
		s3TableWarehouse := ""
		if catalog.Spec.S3Table != nil {
			s3TableWarehouse = catalog.Spec.S3Table.Warehouse
		}

		iamPolicy := generateIAMPolicy(namespace, name, catalogName, accountID, s3TableWarehouse)
		_ = d.Set("iam_policy", iamPolicy)

		// Log IAM policy information for user reference
		tflog.Info(ctx, "🎉 Pulsar cluster created successfully with S3Table catalog!")
		tflog.Info(ctx, fmt.Sprintf("Cluster: %s", name))
		tflog.Info(ctx, fmt.Sprintf("Organization: %s", namespace))
		tflog.Info(ctx, fmt.Sprintf("Catalog: %s", catalogName))
		if accountID != "" {
			tflog.Info(ctx, fmt.Sprintf("Account ID: %s", accountID))
		}
		tflog.Info(ctx, "IAM Policy has been generated and is available in the 'iam_policy' output.")
		tflog.Info(ctx, "Please apply this IAM policy to your AWS IAM role to enable S3Table access.")
	}
	if pc.Status.Conditions != nil {
		ready := false
		for _, condition := range pc.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if ready {
			return resourcePulsarClusterRead(ctx, d, meta)
		}
	}
	err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *retry.RetryError {
		dia := resourcePulsarClusterRead(ctx, d, meta)
		if dia.HasError() {
			return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_PULSAR_CLUSTER: %s", dia[0].Summary))
		}
		ready := d.Get("ready")
		if ready == "False" {
			return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_READ_PULSAR_CLUSTER"))
		}
		return nil
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_PULSAR_CLUSTER: %w", err))
	}
	return nil
}

func resourcePulsarClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_PULSAR_CLUSTER: %w", err))
	}
	var pulsarCluster *cloudv1alpha1.PulsarCluster
	if name == "" {
		organizationCluster := strings.Split(d.Id(), "/")
		name = organizationCluster[1]
		namespace = organizationCluster[0]
	}
	pulsarCluster, err = clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER: %w", err))
	}
	_ = d.Set("ready", "False")
	if pulsarCluster.Status.Conditions != nil {
		for _, condition := range pulsarCluster.Status.Conditions {
			if condition.Type == "Ready" {
				_ = d.Set("ready", condition.Status)
			}
		}
	}
	pulsarInstance, err := clientSet.CloudV1alpha1().PulsarInstances(namespace).Get(ctx, pulsarCluster.Spec.InstanceName, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_INSTANCE: %w", err))
	}
	istioEnabledVal, ok := pulsarInstance.Annotations[IstioEnabledAnnotation]
	istioEnabled := ok && istioEnabledVal == "true"

	var httpTlsServiceUrls []string
	var pulsarTlsServiceUrls []string
	var websocketServiceUrls []string
	var kafkaServiceUrls []string
	var mqttServiceUrls []string
	for _, endpoint := range pulsarCluster.Spec.ServiceEndpoints {
		if endpoint.Type == "service" {
			httpTlsServiceUrls = append(httpTlsServiceUrls, fmt.Sprintf("https://%s", endpoint.DnsName))
			pulsarTlsServiceUrls = append(pulsarTlsServiceUrls, fmt.Sprintf("pulsar+ssl://%s:6651", endpoint.DnsName))
			if pulsarCluster.Spec.Config != nil {
				if pulsarCluster.Spec.Config.WebsocketEnabled != nil && *pulsarCluster.Spec.Config.WebsocketEnabled {
					if istioEnabled {
						websocketServiceUrls = append(websocketServiceUrls, fmt.Sprintf("wss://%s", endpoint.DnsName))
					} else {
						websocketServiceUrls = append(websocketServiceUrls, fmt.Sprintf("ws://%s:9443", endpoint.DnsName))
					}
				}
				if pulsarCluster.Spec.Config.Protocols != nil {
					if pulsarCluster.Spec.Config.Protocols.Kafka != nil && istioEnabled {
						kafkaServiceUrls = append(kafkaServiceUrls, fmt.Sprintf("%s:9093", endpoint.DnsName))
					}
					if pulsarCluster.Spec.Config.Protocols.Mqtt != nil {
						mqttServiceUrls = append(mqttServiceUrls, fmt.Sprintf("mqtts://%s:8883", endpoint.DnsName))
					}
				}
			}
		}
	}
	_ = d.Set("http_tls_service_urls", flattenStringSlice(httpTlsServiceUrls))
	_ = d.Set("pulsar_tls_service_urls", flattenStringSlice(pulsarTlsServiceUrls))
	_ = d.Set("websocket_service_urls", flattenStringSlice(websocketServiceUrls))
	_ = d.Set("kafka_service_urls", flattenStringSlice(kafkaServiceUrls))
	_ = d.Set("mqtt_service_urls", flattenStringSlice(mqttServiceUrls))
	if len(httpTlsServiceUrls) > 0 {
		_ = d.Set("http_tls_service_url", httpTlsServiceUrls[0])
	}
	if len(pulsarTlsServiceUrls) > 0 {
		_ = d.Set("pulsar_tls_service_url", pulsarTlsServiceUrls[0])
	}
	if len(websocketServiceUrls) > 0 {
		_ = d.Set("websocket_service_url", websocketServiceUrls[0])
	}
	if len(kafkaServiceUrls) > 0 {
		_ = d.Set("kafka_service_url", kafkaServiceUrls[0])
	}
	if len(mqttServiceUrls) > 0 {
		_ = d.Set("mqtt_service_url", mqttServiceUrls[0])
	} else {
		_ = d.Set("mqtt_service_url", "")
	}
	if pulsarCluster.Spec.Config != nil {
		tflog.Debug(ctx, "pulsar cluster config: ", map[string]interface{}{
			"config": pulsarCluster.Spec.Config,
		})
		err = d.Set("config", flattenPulsarClusterConfig(pulsarCluster.Spec.Config))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER_CONFIG: %w", err))
		}
	}

	// Set maintenance window if configured
	if pulsarCluster.Spec.MaintenanceWindow != nil {
		err = d.Set("maintenance_window", flattenMaintenanceWindow(pulsarCluster.Spec.MaintenanceWindow))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER_MAINTENANCE_WINDOW: %w", err))
		}
	} else {
		_ = d.Set("maintenance_window", []interface{}{})
	}
	if pulsarInstance.Spec.Type != cloudv1alpha1.PulsarInstanceTypeServerless && !pulsarCluster.IsUsingUrsaEngine() {
		bookkeeperImage := strings.Split(pulsarCluster.Spec.BookKeeper.Image, ":")
		if len(bookkeeperImage) > 1 {
			_ = d.Set("bookkeeper_version", bookkeeperImage[1])
		}
	}
	brokerImage := strings.Split(pulsarCluster.Spec.Broker.Image, ":")
	if len(brokerImage) > 1 {
		_ = d.Set("pulsar_version", brokerImage[1])
	}
	releaseChannel := pulsarCluster.Spec.ReleaseChannel
	if releaseChannel != "" {
		_ = d.Set("release_channel", releaseChannel)
	}
	_ = d.Set("type", pulsarInstance.Spec.Type)
	computeUnit := convertCpuAndMemoryToComputeUnit(pulsarCluster)
	storageUnit := convertCpuAndMemoryToStorageUnit(pulsarCluster)
	_ = d.Set("compute_unit_per_broker", computeUnit)
	_ = d.Set("storage_unit_per_bookie", storageUnit)

	// Set lakehouse_storage_enabled
	if pulsarInstance.Spec.Type == cloudv1alpha1.PulsarInstanceTypeServerless {
		// For serverless clusters, always set to true (computed)
		_ = d.Set("lakehouse_storage_enabled", true)
	} else {
		// For non-serverless clusters, use the actual value
		if pulsarCluster.Spec.Config != nil && pulsarCluster.Spec.Config.LakehouseStorage != nil && pulsarCluster.Spec.Config.LakehouseStorage.Enabled != nil {
			_ = d.Set("lakehouse_storage_enabled", *pulsarCluster.Spec.Config.LakehouseStorage.Enabled)
		} else {
			_ = d.Set("lakehouse_storage_enabled", false)
		}
	}

	// Set catalog information
	if len(pulsarCluster.Spec.Catalogs) > 0 {
		catalogName := pulsarCluster.Spec.Catalogs[0]
		_ = d.Set("catalog", catalogName)

		// Get catalog information
		catalog, err := clientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, catalogName, metav1.GetOptions{})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_GET_CATALOG: %w", err))
		}

		var accountID string
		// Check if it's an S3Table catalog
		if catalog.Spec.S3Table != nil {
			// Validate region match
			if err := validateCatalogRegionMatch(
				ctx, clientSet, namespace, catalogName, pulsarCluster.Spec.Location); err != nil {
				return diag.FromErr(err)
			}
			// Try to get account ID from pool options using instance pool information
			if pulsarCluster.Spec.PoolMemberRef.Name != "" || pulsarCluster.Spec.Location != "" {
				accountIDFromPool, err := getAccountIDFromPoolOptions(
					ctx, clientSet,
					pulsarCluster.Namespace,
					fmt.Sprintf("%s-%s", pulsarInstance.Spec.PoolRef.Namespace, pulsarInstance.Spec.PoolRef.Name),
					pulsarCluster.Spec.Location,
					pulsarCluster.Spec.PoolMemberRef.Name)
				if err != nil {
					tflog.Warn(ctx, fmt.Sprintf("Failed to get account ID from pool options: %v", err))
				} else {
					accountID = accountIDFromPool
				}
			}

			// Get S3Table warehouse
			s3TableWarehouse, err := getS3TableWarehouse(ctx, clientSet, pulsarCluster.Namespace, catalogName)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Failed to get S3Table warehouse: %v", err))
			}

			// Generate and set IAM policy for S3Table catalog
			iamPolicy := generateIAMPolicy(pulsarCluster.Namespace, pulsarCluster.Name, catalogName, accountID, s3TableWarehouse)
			_ = d.Set("iam_policy", iamPolicy)
		}
	} else {
		_ = d.Set("catalog", "")
		_ = d.Set("iam_policy", "")
	}

	d.SetId(fmt.Sprintf("%s/%s", pulsarCluster.Namespace, pulsarCluster.Name))
	return nil
}

func resourcePulsarClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	serverless := d.Get("type")
	displayNameChanged := d.HasChange("display_name")
	lakehouseStorageChanged := d.HasChange("lakehouse_storage_enabled")

	// For serverless clusters, lakehouse_storage_enabled is computed and cannot be changed
	if serverless == string(cloudv1alpha1.PulsarInstanceTypeServerless) {
		if lakehouseStorageChanged {
			return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
				"lakehouse_storage_enabled cannot be set for serverless pulsar cluster, it is automatically computed"))
		}
		// Always set to true for serverless clusters
		_ = d.Set("lakehouse_storage_enabled", true)
	}
	if d.HasChange("organization") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"The pulsar cluster organization does not support updates"))
	}
	if d.HasChange("name") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"The pulsar cluster name does not support updates"))
	}
	if d.HasChange("instance_name") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"The pulsar cluster instance_name does not support updates"))
	}
	if d.HasChange("location") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"The pulsar cluster location does not support updates"))
	}
	if d.HasChange("release_channel") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"The pulsar cluster release channel does not support updates"))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	if d.Get("type") == cloudv1alpha1.PulsarInstanceTypeServerless {
		organizationCluster := strings.Split(d.Id(), "/")
		namespace = organizationCluster[0]
		name = organizationCluster[1]
	}
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_PULSAR_CLUSTER: %w", err))
	}
	if name == "" {
		organizationCluster := strings.Split(d.Id(), "/")
		name = organizationCluster[1]
		namespace = organizationCluster[0]
	}
	pulsarCluster, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER: %w", err))
	}

	// Validate lakehouse_storage_enabled update: once enabled, cannot be disabled
	// For serverless clusters, skip validation as it's computed
	if serverless != string(cloudv1alpha1.PulsarInstanceTypeServerless) {
		if diagErr := validateLakehouseStorageUpdate(d, pulsarCluster); diagErr != nil {
			return diagErr
		}
	} else {
		// For serverless clusters, ensure lakehouse storage is enabled
		if pulsarCluster.Spec.Config == nil {
			pulsarCluster.Spec.Config = &cloudv1alpha1.Config{}
		}
		pulsarCluster.Spec.Config.LakehouseStorage = &cloudv1alpha1.LakehouseStorageConfig{
			Enabled: pointer.Bool(true),
		}
	}
	if d.HasChange("bookie_replicas") {
		bookieReplicas := int32(d.Get("bookie_replicas").(int))
		pulsarCluster.Spec.BookKeeper.Replicas = &bookieReplicas
	}
	if d.HasChange("broker_replicas") {
		brokerReplicas := int32(d.Get("broker_replicas").(int))
		pulsarCluster.Spec.Broker.Replicas = &brokerReplicas
	}
	if d.HasChange("compute_unit") || d.HasChange("compute_unit_per_broker") {
		computeUnit := getComputeUnit(d)
		pulsarCluster.Spec.Broker.Resources.Cpu = resource.NewMilliQuantity(
			int64(computeUnit*2*1000), resource.DecimalSI)
		pulsarCluster.Spec.Broker.Resources.Memory = resource.NewQuantity(
			int64(computeUnit*8*1024*1024*1024), resource.DecimalSI)
	}
	if d.HasChange("storage_unit") || d.HasChange("storage_unit_per_bookie") {
		storageUnit := getStorageUnit(d)
		pulsarCluster.Spec.BookKeeper.Resources.Cpu = resource.NewMilliQuantity(
			int64(storageUnit*2*1000), resource.DecimalSI)
		pulsarCluster.Spec.BookKeeper.Resources.Memory = resource.NewQuantity(
			int64(storageUnit*8*1024*1024*1024), resource.DecimalSI)
	}
	changed := getPulsarClusterChanged(ctx, pulsarCluster, d)
	if displayNameChanged {
		displayName := d.Get("display_name").(string)
		pulsarCluster.Spec.DisplayName = displayName
	}

	// Handle catalog configuration changes
	if d.HasChange("catalog") {
		catalogName := d.Get("catalog").(string)
		if catalogName != "" {
			// Validate catalog configuration
			if err := validateCatalogConfiguration(ctx, clientSet, namespace, catalogName, pulsarCluster.Spec.Location); err != nil {
				return diag.FromErr(err)
			}
			// Add catalog to the cluster
			pulsarCluster.Spec.Catalogs = []string{catalogName}
		} else {
			// Remove catalog
			pulsarCluster.Spec.Catalogs = nil
		}
		changed = true
	}

	// Handle table format determination when catalog or lakehouse storage changes
	if (pulsarCluster.Spec.TableFormat == "" || pulsarCluster.Spec.TableFormat == "none") &&
		(d.HasChange("catalog") || d.HasChange("lakehouse_storage_enabled") || pulsarCluster.IsUsingUrsaEngine()) {
		catalogName := d.Get("catalog").(string)
		// For serverless clusters, lakehouse storage is always enabled
		// Determine table format based on catalog (lakehouse storage is always enabled for serverless)
		tableFormat, err := determineTableFormat(ctx, clientSet, namespace, catalogName)
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_DETERMINE_TABLE_FORMAT: %w", err))
		}
		pulsarCluster.Spec.TableFormat = tableFormat
		changed = true
	}

	if d.Get("apply_lakehouse_to_all_topics").(bool) && pulsarCluster.IsUsingUrsaEngine() {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"you don't set this apply_lakehouse_to_all_topics option for ursa engine cluster"))
	}
	// Handle SDT annotation based on apply_lakehouse_to_all_topics
	if d.HasChange("apply_lakehouse_to_all_topics") || d.HasChange("catalog") || d.HasChange("lakehouse_storage_enabled") {
		if shouldApplyLakehouseToAllTopics(d) {
			if pulsarCluster.Annotations == nil {
				pulsarCluster.Annotations = make(map[string]string)
			}
			pulsarCluster.Annotations["cloud.streamnative.io/sdt-enabled"] = "true"
		} else {
			// Remove the annotation if conditions are not met
			if pulsarCluster.Annotations != nil {
				delete(pulsarCluster.Annotations, "cloud.streamnative.io/sdt-enabled")
			}
		}
		changed = true
	}

	// Update IAM policy if catalog changes
	if d.HasChange("catalog") {
		catalogName := d.Get("catalog").(string)
		if catalogName != "" {
			// Try to get account ID from pool options using instance pool information
			var accountID string
			// Get pulsar instance to access pool information
			pulsarInstance, err := clientSet.CloudV1alpha1().PulsarInstances(namespace).Get(ctx, pulsarCluster.Spec.InstanceName, metav1.GetOptions{})
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Failed to get pulsar instance: %v", err))
			} else if pulsarCluster.Spec.PoolMemberRef.Name != "" || pulsarCluster.Spec.Location != "" {
				accountIDFromPool, err := getAccountIDFromPoolOptions(ctx,
					clientSet,
					pulsarCluster.Namespace,
					fmt.Sprintf("%s-%s", pulsarInstance.Spec.PoolRef.Namespace, pulsarInstance.Spec.PoolRef.Name),
					pulsarCluster.Spec.Location,
					pulsarCluster.Spec.PoolMemberRef.Name)
				if err != nil {
					tflog.Warn(ctx, fmt.Sprintf("Failed to get account ID from pool options: %v", err))
				} else {
					accountID = accountIDFromPool
				}
			}

			// Get S3Table warehouse
			s3TableWarehouse, err := getS3TableWarehouse(ctx, clientSet, namespace, catalogName)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Failed to get S3Table warehouse: %v", err))
			}

			iamPolicy := generateIAMPolicy(namespace, name, catalogName, accountID, s3TableWarehouse)
			_ = d.Set("iam_policy", iamPolicy)
		} else {
			_ = d.Set("iam_policy", "")
		}
	}

	if d.HasChange("bookie_replicas") ||
		d.HasChange("broker_replicas") ||
		d.HasChange("compute_unit") ||
		d.HasChange("storage_unit") ||
		d.HasChange("compute_unit_per_broker") ||
		d.HasChange("storage_unit_per_bookie") || changed || displayNameChanged {
		_, err = clientSet.CloudV1alpha1().PulsarClusters(namespace).Update(ctx, pulsarCluster, metav1.UpdateOptions{
			FieldManager: "terraform-update",
		})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: %w", err))
		}
		// Delay 10 seconds to wait for api server start reconcile.
		time.Sleep(10 * time.Second)
		err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutUpdate), func() *retry.RetryError {
			dia := resourcePulsarClusterRead(ctx, d, meta)
			if dia.HasError() {
				return retry.NonRetryableError(fmt.Errorf("ERROR_RETRY_READ_PULSAR_CLUSTER: %s", dia[0].Summary))
			}
			ready := d.Get("ready")
			if ready == "False" {
				return retry.RetryableError(fmt.Errorf("CONTINUE_RETRY_READ_PULSAR_CLUSTER"))
			}
			return nil
		})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_PULSAR_CLUSTER: %w", err))
		}
	}
	return nil
}

func resourcePulsarClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_DELETE_PULSAR_CLUSTER: %w", err))
	}
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	if name == "" {
		organizationCluster := strings.Split(d.Id(), "/")
		name = organizationCluster[1]
		namespace = organizationCluster[0]
	}
	err = clientSet.CloudV1alpha1().PulsarClusters(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_DELETE_PULSAR_CLUSTER: %w", err))
	}
	err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutDelete), func() *retry.RetryError {
		_, err = clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if statusErr, ok := err.(*errors.StatusError); ok && errors.IsNotFound(statusErr) {
				return nil
			}
			return retry.NonRetryableError(err)
		}

		e := fmt.Errorf("pulsarcluster (%s) still exists", d.Id())
		return retry.RetryableError(e)
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_RETRY_READ_PULSAR_CLUSTER: %w", err))
	}

	d.SetId("")
	return nil
}

func getPulsarClusterChanged(ctx context.Context, pulsarCluster *cloudv1alpha1.PulsarCluster, d *schema.ResourceData) bool {
	changed := false
	if pulsarCluster.Spec.Config == nil {
		pulsarCluster.Spec.Config = &cloudv1alpha1.Config{}
	}

	// Handle lakehouse_storage_enabled at the top level
	// Note: For serverless clusters, this should not be changed by user, but we handle it here for completeness
	if d.HasChange("lakehouse_storage_enabled") {
		enabledBool := d.Get("lakehouse_storage_enabled").(bool)
		if enabledBool {
			pulsarCluster.Spec.Config.LakehouseStorage = &cloudv1alpha1.LakehouseStorageConfig{
				Enabled: &enabledBool,
			}
		} else {
			pulsarCluster.Spec.Config.LakehouseStorage = nil
		}
		changed = true
	}
	config := d.Get("config").([]interface{})
	if len(config) > 0 {
		for _, configItem := range config {
			configItemMap := configItem.(map[string]interface{})
			tflog.Debug(ctx, "configItemMap: %v", configItemMap)
			if configItemMap["websocket_enabled"] != nil {
				webSocketEnabled := configItemMap["websocket_enabled"].(bool)
				pulsarCluster.Spec.Config.WebsocketEnabled = &webSocketEnabled
				changed = true
			}
			if configItemMap["function_enabled"] != nil {
				functionEnabled := configItemMap["function_enabled"].(bool)
				pulsarCluster.Spec.Config.FunctionEnabled = &functionEnabled
				changed = true
			}
			if configItemMap["transaction_enabled"] != nil {
				transactionEnabled := configItemMap["transaction_enabled"].(bool)
				pulsarCluster.Spec.Config.TransactionEnabled = &transactionEnabled
				changed = true
			}
			kafkaEnabled := true
			mqttEnabled := true
			if configItemMap["protocols"] != nil {
				if pulsarCluster.Spec.Config.Protocols == nil {
					pulsarCluster.Spec.Config.Protocols = &cloudv1alpha1.ProtocolsConfig{}
				}
				protocols := configItemMap["protocols"].([]interface{})
				if len(protocols) > 0 {
					for _, protocolItem := range protocols {
						protocolItemMap := protocolItem.(map[string]interface{})
						kafka, ok := protocolItemMap["kafka"]
						if ok {
							if kafka != nil {
								kafkaMap := kafka.(map[string]interface{})
								if enabled, ok := kafkaMap["enabled"]; ok {
									flag := enabled.(string)
									if flag == "false" {
										kafkaEnabled = false
									}
								}
							}
						}
						mqtt, ok := protocolItemMap["mqtt"]
						if ok {
							if mqtt != nil {
								mqttMap := mqtt.(map[string]interface{})
								if enabled, ok := mqttMap["enabled"]; ok {
									flag := enabled.(string)
									if flag == "false" {
										mqttEnabled = false
									}
								}
							}
						}
					}
				}
			}
			if kafkaEnabled {
				pulsarCluster.Spec.Config.Protocols.Kafka = &cloudv1alpha1.KafkaConfig{}
			} else {
				pulsarCluster.Spec.Config.Protocols.Kafka = nil
			}
			if mqttEnabled {
				pulsarCluster.Spec.Config.Protocols.Mqtt = &cloudv1alpha1.MqttConfig{}
			} else {
				pulsarCluster.Spec.Config.Protocols.Mqtt = nil
			}
			if d.HasChanges("protocols") {
				changed = true
			}
			auditLogEnabled := false
			var categories []string
			if configItemMap["audit_log"] != nil {
				auditLog := configItemMap["audit_log"].([]interface{})
				if len(auditLog) > 0 {
					for _, category := range auditLog {
						c := category.(map[string]interface{})
						if _, ok := c["categories"]; ok {
							categoriesSchema := c["categories"].([]interface{})
							if len(categoriesSchema) > 0 {
								auditLogEnabled = true
								for _, categoryItem := range categoriesSchema {
									categories = append(categories, categoryItem.(string))
								}
							}
						}
					}
				}
			}
			if auditLogEnabled {
				pulsarCluster.Spec.Config.AuditLog = &cloudv1alpha1.AuditLog{
					Categories: categories,
				}
			} else {
				pulsarCluster.Spec.Config.AuditLog = nil
			}
			if d.HasChanges("audit_log") {
				changed = true
			}
			if configItemMap["custom"] != nil {
				custom := configItemMap["custom"].(map[string]interface{})
				if len(custom) > 0 {
					result := map[string]string{}
					for k := range custom {
						if v, ok := custom[k].(string); ok {
							result[k] = v
						}
					}
					pulsarCluster.Spec.Config.Custom = result
					changed = true
				}
			}
		}
	}

	// Handle maintenance_window configuration
	if d.HasChange("maintenance_window") {
		maintenanceWindow := d.Get("maintenance_window").([]interface{})
		if len(maintenanceWindow) > 0 {
			for _, mwItem := range maintenanceWindow {
				mwItemMap := mwItem.(map[string]interface{})

				if pulsarCluster.Spec.MaintenanceWindow == nil {
					pulsarCluster.Spec.MaintenanceWindow = &cloudv1alpha1.MaintenanceWindow{}
				}

				// Handle recurrence
				if recurrence, ok := mwItemMap["recurrence"]; ok && recurrence != "" {
					pulsarCluster.Spec.MaintenanceWindow.Recurrence = recurrence.(string)
				}

				// Handle window configuration
				if window, ok := mwItemMap["window"].([]interface{}); ok && len(window) > 0 {
					for _, windowItem := range window {
						windowItemMap := windowItem.(map[string]interface{})

						if pulsarCluster.Spec.MaintenanceWindow.Window == nil {
							pulsarCluster.Spec.MaintenanceWindow.Window = &cloudv1alpha1.Window{}
						}

						// Handle start_time
						if startTime, ok := windowItemMap["start_time"]; ok && startTime != "" {
							pulsarCluster.Spec.MaintenanceWindow.Window.StartTime = startTime.(string)
						}

						// Handle duration
						if durationStr, ok := windowItemMap["duration"]; ok && durationStr != "" {
							duration, err := time.ParseDuration(durationStr.(string))
							if err != nil {
								tflog.Warn(ctx, fmt.Sprintf("Failed to parse maintenance window duration: %v", err))
							} else {
								pulsarCluster.Spec.MaintenanceWindow.Window.Duration = &metav1.Duration{Duration: duration}
							}
						}
					}
				}
			}
		} else {
			// If maintenance_window is empty, clear the maintenance window configuration
			pulsarCluster.Spec.MaintenanceWindow = nil
		}
		changed = true
	}

	tflog.Debug(ctx, "get pulsarcluster changed: %v", map[string]interface{}{
		"pulsarcluster": *pulsarCluster.Spec.Config,
	})
	return changed
}

func getComputeUnit(d *schema.ResourceData) float64 {
	computeUnit := d.Get("compute_unit").(float64)
	if newComputeUnit, exist := d.GetOk("compute_unit_per_broker"); exist {
		computeUnit = newComputeUnit.(float64)
	}
	return computeUnit
}

func getStorageUnit(d *schema.ResourceData) float64 {
	storageUnit := d.Get("storage_unit").(float64)
	if newStorageUnit, exist := d.GetOk("storage_unit_per_bookie"); exist {
		storageUnit = newStorageUnit.(float64)
	}
	return storageUnit
}

func convertCpuAndMemoryToComputeUnit(pc *cloudv1alpha1.PulsarCluster) float64 {
	if pc != nil && pc.Spec.Broker.Resources != nil {
		cpu := pc.Spec.Broker.Resources.Cpu.MilliValue()
		memory := pc.Spec.Broker.Resources.Memory.Value()
		return math.Max(float64(cpu)/2/1000, float64(memory)/(8*1024*1024*1024))
	}
	return 0.5 // default value
}

func convertCpuAndMemoryToStorageUnit(pc *cloudv1alpha1.PulsarCluster) float64 {
	if pc != nil && pc.Spec.BookKeeper != nil && pc.Spec.BookKeeper.Resources != nil {
		cpu := pc.Spec.BookKeeper.Resources.Cpu.MilliValue()
		memory := pc.Spec.BookKeeper.Resources.Memory.Value()
		return math.Max(float64(cpu)/2/1000, float64(memory)/(8*1024*1024*1024))
	}
	return 0.5 // default value
}

// suppressBookieForServerlessOrUrsa suppresses bookie_replicas and storage_unit_per_bookie
// changes for serverless or ursa clusters, and hides them in plan output
func suppressBookieForServerlessOrUrsa(ctx context.Context, diff *schema.ResourceDiff, meta interface{}) {
	isServerless := false
	isUrsa := false

	// Get instance information to check type and ursa status
	instanceName := diff.Get("instance_name").(string)
	namespace := diff.Get("organization").(string)
	if instanceName == "" || namespace == "" {
		return
	}

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		// If we can't get client, skip suppression
		return
	}

	pulsarInstance, err := clientSet.CloudV1alpha1().
		PulsarInstances(namespace).
		Get(ctx, instanceName, metav1.GetOptions{})
	if err != nil {
		// If we can't get instance, skip suppression
		return
	}

	// Check if instance is serverless
	if pulsarInstance.Spec.Type == cloudv1alpha1.PulsarInstanceTypeServerless {
		isServerless = true
	}

	// Check if instance is ursa
	ursaEngine, ok := pulsarInstance.Annotations[UrsaEngineAnnotation]
	if ok && ursaEngine == UrsaEngineValue {
		isUrsa = true
	}

	// If serverless or ursa, suppress and hide bookie-related fields
	if isServerless || isUrsa {
		// Clear changes if any
		if diff.HasChange("bookie_replicas") {
			diff.Clear("bookie_replicas")
		}
		if diff.HasChange("storage_unit_per_bookie") {
			diff.Clear("storage_unit_per_bookie")
		}
		if diff.HasChange("storage_unit") {
			diff.Clear("storage_unit")
		}

		// Hide fields in plan by removing them from the diff for new resources
		// This prevents them from showing up in terraform plan output
		if diff.Id() == "" {
			// This is a create operation, remove fields from diff to hide them
			// For fields with default values, we need to check if they were explicitly set
			// If not explicitly set, we can try to remove them from the diff
			// However, for TypeInt and TypeFloat, we can't set to nil, so we use a workaround:
			// Set them to their default values and rely on DiffSuppressFunc to suppress them
			// But since DiffSuppressFunc already handles this, we just need to ensure
			// the fields are not shown in the plan. The best way is to use SetNewComputed
			// which marks them as "known after apply", but that still shows in plan.
			// Instead, we'll use a different approach: set them to a sentinel value and suppress
			// But actually, the DiffSuppressFunc should already handle this.
			// The issue is that default values still show in plan even with DiffSuppressFunc.
			// Let's try using SetNew to set them to nil (which may not work for TypeInt/Float)
			// or use SetNewComputed which marks them as computed.
			// Actually, the best approach is to use SetNewComputed which should work.
			diff.SetNewComputed("bookie_replicas")
			diff.SetNewComputed("storage_unit_per_bookie")
			diff.SetNewComputed("storage_unit")
		}
	}
}

// isServerlessOrUrsa checks if the cluster type is serverless or if the instance is ursa
// This is used in DiffSuppressFunc where we can only access schema.ResourceData
func isServerlessOrUrsa(d *schema.ResourceData) bool {
	// Check if type is serverless
	clusterType := d.Get("type")
	if clusterType == string(cloudv1alpha1.PulsarInstanceTypeServerless) {
		return true
	}
	// Note: We cannot check for ursa in DiffSuppressFunc because we don't have access to the instance
	// Ursa checking is handled in CustomizeDiff via suppressBookieForServerlessOrUrsa
	return false
}

// makeLakehouseStorageComputedForServerless makes lakehouse_storage_enabled computed for serverless clusters
func makeLakehouseStorageComputedForServerless(ctx context.Context, diff *schema.ResourceDiff, meta interface{}) {
	// Get instance information to check type
	instanceName := diff.Get("instance_name").(string)
	namespace := diff.Get("organization").(string)
	if instanceName == "" || namespace == "" {
		return
	}

	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		// If we can't get client, skip
		return
	}

	pulsarInstance, err := clientSet.CloudV1alpha1().
		PulsarInstances(namespace).
		Get(ctx, instanceName, metav1.GetOptions{})
	if err != nil {
		// If we can't get instance, skip
		return
	}

	// Check if instance is serverless
	if pulsarInstance.Spec.Type == cloudv1alpha1.PulsarInstanceTypeServerless {
		// For serverless clusters, always set lakehouse_storage_enabled to computed
		// and set its value to true
		if diff.HasChange("lakehouse_storage_enabled") {
			// If user tries to set it, clear the change and set as computed with value true
			diff.Clear("lakehouse_storage_enabled")
		}
		// Always set as computed with value true for serverless
		diff.SetNewComputed("lakehouse_storage_enabled")
		// Set the value to true for serverless clusters
		diff.SetNew("lakehouse_storage_enabled", true)
	}
}

// determineTableFormat determines the table format based on catalog type and configuration
func determineTableFormat(ctx context.Context, cloudClientSet *cloudclient.Clientset, namespace, catalogName string) (string, error) {

	// If no catalog is specified, return "none"
	if catalogName == "" {
		return "none", nil
	}

	// Get catalog information
	catalog, err := cloudClientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, catalogName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("ERROR_GET_CATALOG: %w", err)
	}

	// Check catalog type and set table format accordingly
	if catalog.Spec.Unity != nil {
		// For Unity catalog, check URI to determine format
		if strings.Contains(catalog.Spec.Unity.URI, "/api/2.1/unity-catalog/iceberg-rest") {
			return "iceberg", nil
		}
		return "delta", nil
	}

	if catalog.Spec.OpenCatalog != nil || catalog.Spec.S3Table != nil {
		// For OpenCatalog and S3Table, always use iceberg
		return "iceberg", nil
	}

	// Default to "none" if catalog type is not recognized
	return "none", nil
}

// shouldApplyLakehouseToAllTopics checks if the SDT annotation should be added
func shouldApplyLakehouseToAllTopics(d *schema.ResourceData) bool {
	// Check if lakehouse storage is enabled
	lakehouseStorageEnabled := d.Get("lakehouse_storage_enabled").(bool)
	if lakehouseStorageEnabled {
		// Check if catalog is set
		catalogName := d.Get("catalog").(string)
		if catalogName == "" {
			return false
		}

		// Check if apply_lakehouse_to_all_topics is enabled
		applyToAllTopics := d.Get("apply_lakehouse_to_all_topics").(bool)
		return applyToAllTopics
	}
	return false
}

// validateCatalogConfiguration validates catalog configuration for the cluster
func validateCatalogConfiguration(ctx context.Context, cloudClientSet *cloudclient.Clientset, namespace, catalogName, clusterLocation string) error {
	// Get catalog information
	catalog, err := cloudClientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, catalogName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ERROR_GET_CATALOG: %w", err)
	}

	// Check if it's an S3Table catalog
	if catalog.Spec.S3Table != nil {
		// Validate region match
		if err := validateCatalogRegionMatch(ctx, cloudClientSet, namespace, catalogName, clusterLocation); err != nil {
			return err
		}
	}

	return nil
}

// validateCatalogRegionMatch validates that S3Table catalog region matches cluster location
func validateCatalogRegionMatch(ctx context.Context, cloudClientSet *cloudclient.Clientset, namespace, catalogName, clusterLocation string) error {
	// Get catalog information
	catalog, err := cloudClientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, catalogName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ERROR_GET_CATALOG: %w", err)
	}

	// Check if it's an S3Table catalog
	if catalog.Spec.S3Table == nil {
		return nil // Not an S3Table catalog, no validation needed
	}

	// Extract region from S3Table warehouse ARN
	catalogRegion, err := extractS3TableRegion(catalog.Spec.S3Table.Warehouse)
	if err != nil {
		return fmt.Errorf("ERROR_EXTRACT_CATALOG_REGION: %w", err)
	}

	// Check if regions match
	if catalogRegion != clusterLocation {
		return fmt.Errorf("You can only select a catalog in the same region (%s) as this cluster", clusterLocation)
	}

	return nil
}

// getAccountIDFromPoolOptions retrieves the account ID from PoolOptions API
func getAccountIDFromPoolOptions(ctx context.Context,
	cloudClientSet *cloudclient.Clientset,
	namespace, poolName,
	location, poolmemberName string) (string, error) {

	// Get pool options
	poolOption, err := cloudClientSet.CloudV1alpha1().PoolOptions(namespace).Get(ctx, poolName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("ERROR_GET_POOL_OPTIONS: %w", err)
	}
	if len(poolOption.Status.Environments) > 0 {
		for _, environment := range poolOption.Status.Environments {
			if environment.Name == poolmemberName {
				return environment.AwsAccountId, nil
			}
			if environment.Region == location {
				return environment.AwsAccountId, nil
			}
		}
	}
	return "", fmt.Errorf("ERROR_POOL_OPTIONS_STRUCTURE: PoolOptions.Status.Environments field needs to be added to the API structure")
}

// generateIAMPolicy generates IAM policy JSON for S3Table catalog access
func generateIAMPolicy(organization, clusterName, catalogName string, accountID string, s3TableWarehouse string) string {
	// Use the provided account ID or fallback to placeholder
	actualAccountID := accountID
	if actualAccountID == "" {
		actualAccountID = "YOUR_ACCOUNT_ID"
	}

	// Use the provided warehouse or fallback to placeholder
	actualWarehouse := s3TableWarehouse
	if actualWarehouse == "" {
		actualWarehouse = "YOUR_S3_TABLE_BUCKET_ARN"
	}

	policy := fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "S3ListTableBucket",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::%s:role/StreamNative/sncloud-role/authorization.streamnative.io/iamaccounts/IamAccount-%s-%s-broker"
      },
      "Action": [
        "s3tables:ListTableBuckets"
      ],
      "Resource": [
        "*"
      ]
    },
    {
      "Sid": "DataAccessPermissionsForS3TableBucket",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::%s:role/StreamNative/sncloud-role/authorization.streamnative.io/iamaccounts/IamAccount-%s-%s-broker"
      },
      "Action": [
        "s3tables:GetTableBucket",
        "s3tables:CreateNamespace",
        "s3tables:GetNamespace",
        "s3tables:ListNamespaces",
        "s3tables:CreateTable",
        "s3tables:GetTable",
        "s3tables:ListTables",
        "s3tables:UpdateTableMetadataLocation",
        "s3tables:GetTableMetadataLocation",
        "s3tables:GetTableData",
        "s3tables:PutTableData"
      ],
      "Resource": [
        "%s",
        "%s/*"
      ]
    }
  ]
}`, actualAccountID, organization, clusterName, actualAccountID, organization, clusterName, actualWarehouse, actualWarehouse)

	return policy
}

// getS3TableWarehouse retrieves the warehouse field from S3Table catalog
func getS3TableWarehouse(ctx context.Context, cloudClientSet *cloudclient.Clientset, namespace, catalogName string) (string, error) {
	if catalogName == "" {
		return "", nil
	}

	// Get catalog information
	catalog, err := cloudClientSet.CloudV1alpha1().Catalogs(namespace).Get(ctx, catalogName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("ERROR_GET_CATALOG: %w", err)
	}

	// Check if it's an S3Table catalog and get warehouse
	if catalog.Spec.S3Table != nil && catalog.Spec.S3Table.Warehouse != "" {
		return catalog.Spec.S3Table.Warehouse, nil
	}

	return "", nil
}

// validateLakehouseStorageUpdate validates that lakehouse_storage_enabled cannot be disabled once enabled
func validateLakehouseStorageUpdate(d *schema.ResourceData, pulsarCluster *cloudv1alpha1.PulsarCluster) diag.Diagnostics {
	if d.HasChange("lakehouse_storage_enabled") {
		newEnabled := d.Get("lakehouse_storage_enabled").(bool)
		// Check if lakehouse storage was previously enabled
		if pulsarCluster.Spec.Config != nil &&
			pulsarCluster.Spec.Config.LakehouseStorage != nil &&
			pulsarCluster.Spec.Config.LakehouseStorage.Enabled != nil &&
			*pulsarCluster.Spec.Config.LakehouseStorage.Enabled {
			// If it was enabled and trying to set to false, reject the update
			if !newEnabled {
				return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
					"lakehouse_storage_enabled cannot be disabled once it has been enabled"))
			}
		}
	}
	return nil
}
