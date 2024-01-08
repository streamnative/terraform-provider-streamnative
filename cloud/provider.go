package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sncloud "github.com/tuteng/sncloud-go-sdk"
	"golang.org/x/oauth2/clientcredentials"
	"net/url"
	"os"
)

const (
	GlobalDefaultIssuer           = "https://auth.streamnative.cloud/"
	GlobalDefaultAudience         = "https://api.streamnative.cloud"
	GlobalDefaultAPIServer        = "api.streamnative.cloud"
	ServiceAccountAdminAnnotation = "annotations.cloud.streamnative.io/service-account-role"
	KeyFileTypeServiceAccount     = "sn_service_account"
)

type KeyFile struct {
	Type         string `json:"type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ClientEmail  string `json:"client_email"`
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"key_file_path":       "The path of the private key file",
		"organization":        "The organization name",
		"name":                "The service account name",
		"admin":               "Whether the service account is admin",
		"private_key_data":    "The private key data",
		"availability-mode":   "The availability mode, supporting 'zonal' and 'regional'",
		"pool_name":           "The infrastructure pool name to use.",
		"pool_namespace":      "The infrastructure pool namespace to use",
		"instance_name":       "The pulsar instance name",
		"location":            "The location of the pulsar cluster",
		"bookie_replicas":     "The number of bookie replicas",
		"broker_replicas":     "The number of broker replicas",
		"compute_unit":        "compute unit, 1 compute unit is 2 cpu and 8gb memory",
		"storage_unit":        "storage unit, 1 storage unit is 2 cpu and 8gb memory",
		"cluster_ready":       "Pulsar cluster is ready, it will be set to 'True' after the cluster is ready",
		"instance_ready":      "Pulsar instance is ready, it will be set to 'True' after the instance is ready",
		"websocket_enabled":   "Whether the websocket is enabled",
		"function_enabled":    "Whether the function is enabled",
		"transaction_enabled": "Whether the transaction is enabled",
		"kafka":               "Controls the kafka protocol config of pulsar cluster",
		"mqtt":                "Controls the mqtt protocol config of pulsar cluster",
		"categories": "Controls the audit log categories config of pulsar cluster, supported categories: " +
			"\"Management\", \"Describe\", \"Produce\", \"Consume\"",
		"custom":                 "Controls the custom config of pulsar cluster",
		"http_tls_service_url":   "The service url of the pulsar cluster, use it to management the pulsar cluster",
		"pulsar_tls_service_url": "The service url of the pulsar cluster, use it to produce and consume message",
		"kafka_service_url": "If you want to connect to the pulsar cluster using the kafka protocol," +
			" use this kafka service url",
		"mqtt_service_url": "If you want to connect to the pulsar cluster using the mqtt protocol, " +
			"use this mqtt service url",
		"websocket_service_url": "If you want to connect to the pulsar cluster using the websocket protocol, " +
			"use this websocket service url",
		"pulsar_version":     "The version of the pulsar cluster",
		"bookkeeper_version": "The version of the bookkeeper cluster",
	}
}

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"key_file_path": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KEY_FILE_PATH", nil),
				Description: descriptions["key_file_path"],
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"streamnative_service_account": resourceServiceAccount(),
			"streamnative_pulsar_instance": resourcePulsarInstance(),
			"streamnative_pulsar_cluster":  resourcePulsarCluster(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"streamnative_service_account": dataSourceServiceAccount(),
			"streamnative_pulsar_instance": dataSourcePulsarInstance(),
			"streamnative_pulsar_cluster":  dataSourcePulsarCluster(),
		},
	}
	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return providerConfigure(d, provider.TerraformVersion)
	}
	return provider
}

func providerConfigure(d *schema.ResourceData, terraformVersion string) (interface{}, diag.Diagnostics) {
	_ = terraformVersion
	keyFilePath := d.Get("key_file_path").(string)
	keyFile, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	var v KeyFile
	err = json.Unmarshal(keyFile, &v)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	if v.Type != KeyFileTypeServiceAccount {
		return nil, diag.FromErr(fmt.Errorf("open %s: unsupported format", keyFilePath))
	}

	defaultIssuer := os.Getenv("GLOBAL_DEFAULT_ISSUER")
	if defaultIssuer == "" {
		defaultIssuer = GlobalDefaultIssuer
	}
	defaultAudience := os.Getenv("GLOBAL_DEFAULT_AUDIENCE")
	if defaultAudience == "" {
		defaultAudience = GlobalDefaultAudience
	}
	defaultApiServer := os.Getenv("GLOBAL_DEFAULT_API_SERVER")
	if defaultApiServer == "" {
		defaultApiServer = GlobalDefaultAPIServer
	}
	debug := os.Getenv("TF_LOG")

	values := url.Values{
		"audience": {defaultAudience},
	}
	config := clientcredentials.Config{
		ClientID:       v.ClientID,
		ClientSecret:   v.ClientSecret,
		TokenURL:       fmt.Sprintf("%soauth/token", defaultIssuer),
		EndpointParams: values,
	}
	token, err := config.Token(context.Background())
	if err != nil {
		return nil, diag.FromErr(err)
	}
	configuration := sncloud.NewConfiguration()
	configuration.Host = defaultApiServer
	configuration.Scheme = "https"
	if debug == "debug" {
		configuration.Debug = true
	} else {
		configuration.Debug = false
	}
	configuration.AddDefaultHeader("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	apiClient := sncloud.NewAPIClient(configuration)
	return apiClient, nil
}
