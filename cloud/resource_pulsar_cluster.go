package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	cloudv1alpha1 "github.com/streamnative/cloud-api-server/pkg/apis/cloud/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
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
			if diff.HasChanges([]string{"organization", "name", "instance_name", "location"}...) {
				return fmt.Errorf("ERROR_UPDATE_PULSAR_CLUSTER: " +
					"The pulsar cluster organization, name, instance_name, location does not support updates, please recreate it")
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
				Description:  descriptions["name"],
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
				Required:     true,
				Description:  descriptions["location"],
				ValidateFunc: validateNotBlank,
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
			"websocket_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: descriptions["websocket_enabled"],
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
			"kafka": {
				Type:        schema.TypeMap,
				Optional:    true,
				Default:     map[string]interface{}{},
				Description: descriptions["kafka"],
			},
			"mqtt": {
				Type:        schema.TypeMap,
				Optional:    true,
				Default:     map[string]interface{}{},
				Description: descriptions["mqtt"],
			},
			"audit_log": {
				Type:        schema.TypeSet,
				Optional:    true,
				MinItems:    1,
				Description: descriptions["audit_log"],
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validateAuditLog,
				},
			},
			"custom": {
				Type:        schema.TypeMap,
				Optional:    true,
				Description: descriptions["custom"],
			},
			"config": {
				Type:     schema.TypeSet,
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
							Type:        schema.TypeSet,
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
							Type:        schema.TypeSet,
							Optional:    true,
							Description: descriptions["audit_log"],
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"categories": {
										Type:        schema.TypeSet,
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
			"pulsar_tls_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["pulsar_tls_service_url"],
			},
			"kafka_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["kafka_service_url"],
			},
			"mqtt_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["mqtt_service_url"],
			},
			"websocket_service_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: descriptions["websocket_service_url"],
			},
		},
	}
}

func resourcePulsarClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	namespace := d.Get("organization").(string)
	name := d.Get("name").(string)
	instanceName := d.Get("instance_name").(string)
	location := d.Get("location").(string)
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
		Get(context.Background(), instanceName, metav1.GetOptions{
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
			InstanceName: instanceName,
			Location:     location,
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
		}
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
	config := d.Get("config").(*schema.Set)
	if config.Len() > 0 {
		for _, configItem := range config.List() {
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
				protocols := configItemMap["protocols"].(*schema.Set)
				for _, protocolItem := range protocols.List() {
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
			categories := make([]string, 0)
			if configItemMap["audit_log"] != nil {
				auditLog := configItemMap["audit_log"].(*schema.Set)
				if auditLog.Len() > 0 {
					for _, category := range auditLog.List() {
						c := category.(map[string]interface{})
						if _, ok := c["categories"]; ok {
							categoriesSchema := c["categories"].(*schema.Set)
							if categoriesSchema.Len() > 0 {
								auditLogEnabled = true
								for _, categoryItem := range categoriesSchema.List() {
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
