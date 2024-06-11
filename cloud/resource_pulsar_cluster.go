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
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
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
			oldName, _ := diff.GetChange("name")
			if oldOrg.(string) == "" && oldName.(string) == "" {
				// This is create event, so we don't need to check the diff.
				return nil
			}
			if diff.HasChanges([]string{"organization", "name", "instance_name", "location", "pool_member_name"}...) {
				return fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
					"The pulsar cluster organization, name, instance_name, location, pool_member_name does not support updates, please recreate it")
			}
			return nil
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				organizationCluster := strings.Split(d.Id(), "/")
				_ = d.Set("organization", organizationCluster[0])
				_ = d.Set("name", organizationCluster[1])
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
				Type:         schema.TypeString,
				Required:     true,
				Description:  descriptions["cluster_name"],
				ValidateFunc: validateNotBlank,
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
			},
			"bookie_replicas": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      3,
				Description:  descriptions["bookie_replicas"],
				ValidateFunc: validateBookieReplicas,
			},
			"broker_replicas": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      2,
				Description:  descriptions["broker_replicas"],
				ValidateFunc: validateBrokerReplicas,
			},
			"compute_unit": {
				Type:         schema.TypeFloat,
				Optional:     true,
				Default:      0.5,
				Description:  descriptions["compute_unit"],
				ValidateFunc: validateCUSU,
			},
			"storage_unit": {
				Type:         schema.TypeFloat,
				Optional:     true,
				Default:      0.5,
				Description:  descriptions["storage_unit"],
				ValidateFunc: validateCUSU,
			},
			"config": {
				Type:     schema.TypeList,
				Optional: true,
				MinItems: 0,
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
							Default:     false,
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
			"http_tls_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["http_tls_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"pulsar_tls_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["pulsar_tls_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"kafka_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["kafka_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"mqtt_service_urls": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: descriptions["mqtt_service_urls"],
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
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
		},
	}
}

func resourcePulsarClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
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
	computeUnit := d.Get("compute_unit").(float64)
	storageUnit := d.Get("storage_unit").(float64)
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
	if pulsarInstance.Spec.Plan == string(cloudv1alpha1.PulsarInstanceTypeFree) {
		return diag.FromErr(fmt.Errorf(
			"ERROR_CREATE_PULSAR_CLUSTER: "+
				"creating a cluster under instance of type '%s' is no longer allowed",
			cloudv1alpha1.PulsarInstanceTypeFree))
	}
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
			Name:      name,
			Namespace: namespace,
		},
		Spec: cloudv1alpha1.PulsarClusterSpec{
			InstanceName:   instanceName,
			Location:       location,
			ReleaseChannel: releaseChannel,
			BookKeeper: &cloudv1alpha1.BookKeeper{
				Replicas: &bookieReplicas,
				Resources: &cloudv1alpha1.BookkeeperNodeResource{
					DefaultNodeResource: cloudv1alpha1.DefaultNodeResource{
						Cpu:    bookieCPU,
						Memory: bookieMem,
					},
				},
			},
			Broker: cloudv1alpha1.Broker{
				Replicas: &brokerReplicas,
				Resources: &cloudv1alpha1.DefaultNodeResource{
					Cpu:    brokerCPU,
					Memory: brokerMem,
				},
			},
		},
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
	getPulsarClusterChanged(pulsarCluster, d)
	pc, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Create(ctx, pulsarCluster, metav1.CreateOptions{
		FieldManager: "terraform-create",
	})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_CREATE_PULSAR_CLUSTER: %w", err))
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
	err = retry.RetryContext(ctx, 15*time.Minute, func() *retry.RetryError {
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
	pulsarCluster, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
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
	_ = d.Set("pulsar_tls_service_urls", flattenStringSlice(httpTlsServiceUrls))
	_ = d.Set("websocket_service_urls", flattenStringSlice(websocketServiceUrls))
	_ = d.Set("kafka_service_urls", flattenStringSlice(kafkaServiceUrls))
	_ = d.Set("mqtt_service_urls", flattenStringSlice(mqttServiceUrls))

	if len(pulsarCluster.Spec.ServiceEndpoints) > 0 {
		dnsName := pulsarCluster.Spec.ServiceEndpoints[0].DnsName
		_ = d.Set("http_tls_service_url", fmt.Sprintf("https://%s", dnsName))
		_ = d.Set("pulsar_tls_service_url", fmt.Sprintf("pulsar+ssl://%s:6651", dnsName))
		if pulsarCluster.Spec.Config != nil {
			if pulsarCluster.Spec.Config.WebsocketEnabled != nil &&
				*pulsarCluster.Spec.Config.WebsocketEnabled {
				_ = d.Set("websocket_service_url", fmt.Sprintf("wss://%s:9443", dnsName))
			}
			if pulsarCluster.Spec.Config.Protocols != nil {
				if pulsarCluster.Spec.Config.Protocols.Kafka != nil {
					_ = d.Set("kafka_service_url", fmt.Sprintf("%s:9093", dnsName))
				}
				if pulsarCluster.Spec.Config.Protocols.Mqtt != nil {
					_ = d.Set("mqtt_service_url", fmt.Sprintf("mqtts://%s:8883", dnsName))
				}
			}
		}
	}
	if pulsarCluster.Spec.Config != nil {
		err = d.Set("config", flattenPulsarClusterConfig(pulsarCluster.Spec.Config))
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER_CONFIG: %w", err))
		}
	}
	brokerImage := strings.Split(pulsarCluster.Spec.Broker.Image, ":")
	_ = d.Set("pulsar_version", brokerImage[1])
	bookkeeperImage := strings.Split(pulsarCluster.Spec.BookKeeper.Image, ":")
	_ = d.Set("bookkeeper_version", bookkeeperImage[1])
	releaseChannel := pulsarCluster.Spec.ReleaseChannel
	if releaseChannel != "" {
		_ = d.Set("release_channel", releaseChannel)
	}
	d.SetId(fmt.Sprintf("%s/%s", pulsarCluster.Namespace, pulsarCluster.Name))
	return nil
}

func resourcePulsarClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_PULSAR_CLUSTER: %w", err))
	}
	pulsarCluster, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_READ_PULSAR_CLUSTER: %w", err))
	}
	if d.HasChange("bookie_replicas") {
		brokerReplicas := int32(d.Get("bookie_replicas").(int))
		pulsarCluster.Spec.Broker.Replicas = &brokerReplicas
	}
	if d.HasChange("broker_replicas") {
		bookieReplicas := int32(d.Get("broker_replicas").(int))
		pulsarCluster.Spec.Broker.Replicas = &bookieReplicas
	}
	if d.HasChange("compute_unit") {
		computeUnit := d.Get("compute_unit").(float64)
		pulsarCluster.Spec.Broker.Resources.Cpu = resource.NewMilliQuantity(
			int64(computeUnit*2*1000), resource.DecimalSI)
		pulsarCluster.Spec.Broker.Resources.Memory = resource.NewQuantity(
			int64(computeUnit*8*1024*1024*1024), resource.DecimalSI)
	}
	if d.HasChange("storage_unit") {
		storageUnit := d.Get("storage_unit").(float64)
		pulsarCluster.Spec.BookKeeper.Resources.Cpu = resource.NewMilliQuantity(
			int64(storageUnit*2*1000), resource.DecimalSI)
		pulsarCluster.Spec.BookKeeper.Resources.Memory = resource.NewQuantity(
			int64(storageUnit*8*1024*1024*1024), resource.DecimalSI)
	}
	changed := getPulsarClusterChanged(pulsarCluster, d)
	if d.HasChange("bookie_replicas") ||
		d.HasChange("broker_replicas") ||
		d.HasChange("compute_unit") ||
		d.HasChange("storage_unit") || changed {
		_, err = clientSet.CloudV1alpha1().PulsarClusters(namespace).Update(ctx, pulsarCluster, metav1.UpdateOptions{
			FieldManager: "terraform-update",
		})
		if err != nil {
			return diag.FromErr(fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: %w", err))
		}
		// Delay 10 seconds to wait for api server start reconcile.
		time.Sleep(10 * time.Second)
		err = retry.RetryContext(ctx, 15*time.Minute, func() *retry.RetryError {
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
	err = clientSet.CloudV1alpha1().PulsarClusters(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_DELETE_PULSAR_CLUSTER: %w", err))
	}
	return nil
}

func getPulsarClusterChanged(pulsarCluster *cloudv1alpha1.PulsarCluster, d *schema.ResourceData) bool {
	changed := false
	if pulsarCluster.Spec.Config == nil {
		pulsarCluster.Spec.Config = &cloudv1alpha1.Config{}
	}
	config := d.Get("config").([]interface{})
	if len(config) > 0 {
		for _, configItem := range config {
			configItemMap := configItem.(map[string]interface{})
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
	return changed
}
