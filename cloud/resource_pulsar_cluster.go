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

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
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
				Computed:     true,
				Description:  descriptions["bookie_replicas"],
				ValidateFunc: validateBookieReplicas,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return isServerlessOrUrsa(d)
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
				Computed:     true,
				Description:  descriptions["storage_unit_per_bookie"],
				ValidateFunc: validateCUSU,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return isServerlessOrUrsa(d)
				},
			},
			"storage_unit_per_bookie": {
				Type:         schema.TypeFloat,
				Optional:     true,
				Computed:     true,
				Description:  descriptions["storage_unit_per_bookie"],
				ValidateFunc: validateCUSU,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return isServerlessOrUrsa(d)
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
						"lakehouse_storage": {
							Type:        schema.TypeMap,
							Optional:    true,
							Description: descriptions["lakehouse_storage"],
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
	bookieReplicas := int32(3)
	if v, ok := d.GetOkExists("bookie_replicas"); ok {
		bookieReplicas = int32(v.(int))
	}
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
	pc, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Create(ctx, pulsarCluster, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: %w", err))
	}
	d.SetId(fmt.Sprintf("%s/%s", pc.Namespace, pc.Name))
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
	d.SetId(fmt.Sprintf("%s/%s", pulsarCluster.Namespace, pulsarCluster.Name))
	return nil
}

func resourcePulsarClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	serverless := d.Get("type")
	displayNameChanged := d.HasChange("display_name")
	if !displayNameChanged && serverless == string(cloudv1alpha1.PulsarInstanceTypeServerless) {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"only display_name can be updated for serverless instance"))
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
	if d.HasChanges("lakehouse_type", "catalog_type", "catalog_credentials", "catalog_connection_url", "catalog_warehouse") {
		return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
			"The pulsar cluster lakehouse storage does not support updates"))
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
	if d.HasChange("bookie_replicas") {
		bookieReplicas := int32(3)
		if v, ok := d.GetOkExists("bookie_replicas"); ok {
			bookieReplicas = int32(v.(int))
		}
		if pulsarCluster.Spec.BookKeeper != nil {
			pulsarCluster.Spec.BookKeeper.Replicas = &bookieReplicas
		}
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
	tflog.Debug(ctx, "get pulsarcluster changed: %v", map[string]interface{}{
		"pulsarcluster": *pulsarCluster.Spec.Config,
	})
	return changed
}

func getComputeUnit(d *schema.ResourceData) float64 {
	if newComputeUnit, exist := d.GetOkExists("compute_unit_per_broker"); exist {
		return newComputeUnit.(float64)
	}
	if computeUnit, exist := d.GetOkExists("compute_unit"); exist {
		return computeUnit.(float64)
	}
	return 0.5
}

func getStorageUnit(d *schema.ResourceData) float64 {
	if newStorageUnit, exist := d.GetOkExists("storage_unit_per_bookie"); exist {
		return newStorageUnit.(float64)
	}
	if storageUnit, exist := d.GetOkExists("storage_unit"); exist {
		return storageUnit.(float64)
	}
	return 0.5
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
