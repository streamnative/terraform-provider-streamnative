package cloud

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
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
				Description:  descriptions["name"],
				ValidateFunc: validateNotBlank,
			},
			"instance_name": {
				Type:        schema.TypeString,
				Description: descriptions["instance_name"],
				Computed:    true,
			},
			"location": {
				Type:        schema.TypeString,
				Description: descriptions["location"],
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
				Type:        schema.TypeFloat,
				Description: descriptions["compute_unit"],
				Computed:    true,
			},
			"storage_unit": {
				Type:        schema.TypeFloat,
				Description: descriptions["storage_unit"],
				Computed:    true,
			},
			"websocket_enabled": {
				Type:        schema.TypeBool,
				Description: descriptions["websocket_enabled"],
				Computed:    true,
			},
			"function_enabled": {
				Type:        schema.TypeBool,
				Description: descriptions["function_enabled"],
				Computed:    true,
			},
			"transaction_enabled": {
				Type:        schema.TypeBool,
				Description: descriptions["transaction_enabled"],
				Computed:    true,
			},
			"kafka": {
				Type:        schema.TypeMap,
				Description: descriptions["kafka"],
				Computed:    true,
			},
			"mqtt": {
				Type:        schema.TypeMap,
				Description: descriptions["mqtt"],
				Computed:    true,
			},
			"categories": {
				Type:        schema.TypeSet,
				Description: descriptions["categories"],
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"custom": {
				Type:        schema.TypeMap,
				Computed:    true,
				Description: descriptions["custom"],
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

func dataSourcePulsarClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
		if pulsarCluster.Spec.Config.Protocols != nil {
			if pulsarCluster.Spec.Config.Protocols.Kafka != nil {
				_ = d.Set("kafka", map[string]interface{}{})
			}
			if pulsarCluster.Spec.Config.Protocols.Mqtt != nil {
				_ = d.Set("mqtt", map[string]interface{}{})
			}
		}
		if pulsarCluster.Spec.Config.FunctionEnabled != nil {
			_ = d.Set("function_enabled", *pulsarCluster.Spec.Config.FunctionEnabled)
		}
		if pulsarCluster.Spec.Config.TransactionEnabled != nil {
			_ = d.Set("transaction_enabled", *pulsarCluster.Spec.Config.TransactionEnabled)
		}
		if pulsarCluster.Spec.Config.WebsocketEnabled != nil {
			_ = d.Set("websocket_enabled", *pulsarCluster.Spec.Config.WebsocketEnabled)
		}
		if pulsarCluster.Spec.Config.AuditLog != nil {
			categories := make([]interface{}, len(pulsarCluster.Spec.Config.AuditLog.Categories))
			for i, category := range pulsarCluster.Spec.Config.AuditLog.Categories {
				categories[i] = category
			}
			_ = d.Set("categories", categories)
		}
		if pulsarCluster.Spec.Config.Custom != nil {
			_ = d.Set("custom", pulsarCluster.Spec.Config.Custom)
		}
	}
	brokerImage := strings.Split(pulsarCluster.Spec.Broker.Image, ":")
	_ = d.Set("pulsar_version", brokerImage[1])
	bookkeeperImage := strings.Split(pulsarCluster.Spec.BookKeeper.Image, ":")
	_ = d.Set("bookkeeper_version", bookkeeperImage[1])
	if pulsarCluster.Spec.Config.AuditLog != nil && len(pulsarCluster.Spec.Config.AuditLog.Categories) > 0 {
		_ = d.Set("categories", pulsarCluster.Spec.Config.AuditLog.Categories)
	}
	d.SetId(fmt.Sprintf("%s/%s", pulsarCluster.Namespace, pulsarCluster.Name))
	return nil
}
