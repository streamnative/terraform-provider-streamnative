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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IstioEnabledAnnotation = "annotations.cloud.streamnative.io/istio-enabled"
	UrsaEngineAnnotation   = "cloud.streamnative.io/engine"
	UrsaEngineValue        = "ursa"
)

func dataSourcePulsarCluster() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePulsarClusterRead,
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
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["instance_name"],
			},
			"location": {
				Type:        schema.TypeString,
				Description: descriptions["location"],
				Computed:    true,
			},
			"release_channel": {
				Type:        schema.TypeString,
				Description: descriptions["release_channel"],
				Computed:    true,
			},
			"bookie_replicas": {
				Type:        schema.TypeInt,
				Description: descriptions["bookie_replicas"],
				Computed:    true,
			},
			"broker_replicas": {
				Type:        schema.TypeInt,
				Description: descriptions["broker_replicas"],
				Computed:    true,
			},
			"compute_unit": {
				Deprecated:  "Deprecated. Please use compute_unit_per_broker instead.",
				Type:        schema.TypeFloat,
				Description: descriptions["compute_unit_per_broker"],
				Computed:    true,
			},
			"compute_unit_per_broker": {
				Type:        schema.TypeFloat,
				Description: descriptions["compute_unit_per_broker"],
				Computed:    true,
			},
			"storage_unit": {
				Deprecated:  "Deprecated. Please use storage_unit_per_bookie instead.",
				Type:        schema.TypeFloat,
				Description: descriptions["storage_unit_per_bookie"],
				Computed:    true,
			},
			"storage_unit_per_broker": {
				Type:        schema.TypeFloat,
				Description: descriptions["storage_unit_per_bookie"],
				Computed:    true,
			},
			"config": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"websocket_enabled": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"function_enabled": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: descriptions["function_enabled"],
						},
						"transaction_enabled": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: descriptions["transaction_enabled"],
						},
						"protocols": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: descriptions["protocols"],
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"kafka": {
										Type:        schema.TypeMap,
										Computed:    true,
										Description: descriptions["kafka"],
									},
									"mqtt": {
										Type:        schema.TypeMap,
										Computed:    true,
										Description: descriptions["mqtt"],
									},
								},
							},
						},
						"audit_log": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: descriptions["audit_log"],
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"categories": {
										Type:        schema.TypeList,
										Computed:    true,
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
							Computed:    true,
							Description: descriptions["lakehouse_storage"],
						},
						"custom": {
							Type:        schema.TypeMap,
							Computed:    true,
							Description: descriptions["custom"],
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

func dataSourcePulsarClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	instanceName := d.Get("instance_name").(string)
	clientSet, err := getClientSet(getFactoryFromMeta(meta))
	if err != nil {
		return diag.FromErr(fmt.Errorf("ERROR_INIT_CLIENT_ON_READ_PULSAR_CLUSTER: %w", err))
	}
	pulsarCluster, err := clientSet.CloudV1alpha1().PulsarClusters(namespace).Get(ctx, name, metav1.GetOptions{})
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
	_ = d.Set("type", pulsarInstance.Spec.Type)
	releaseChannel := pulsarCluster.Spec.ReleaseChannel
	if releaseChannel != "" {
		_ = d.Set("release_channel", releaseChannel)
	}

	_ = d.Set("instance_name", pulsarInstance.Name)
	d.SetId(fmt.Sprintf("%s/%s", pulsarCluster.Namespace, pulsarCluster.Name))
	return nil
}
