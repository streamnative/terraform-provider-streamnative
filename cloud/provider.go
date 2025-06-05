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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/99designs/keyring"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/streamnative/cloud-cli/pkg/auth"
	"github.com/streamnative/cloud-cli/pkg/auth/store"
	"github.com/streamnative/cloud-cli/pkg/cmd"
	"github.com/streamnative/cloud-cli/pkg/config"
	"github.com/streamnative/cloud-cli/pkg/plugin"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/utils/clock"
)

const (
	GlobalDefaultIssuer                   = "https://auth.streamnative.cloud/"
	GlobalDefaultAudience                 = "https://api.streamnative.cloud"
	GlobalDefaultAPIServer                = "https://api.streamnative.cloud"
	GlobalDefaultCertificateAuthorityData = ``
	ServiceAccountAdminAnnotation         = "annotations.cloud.streamnative.io/service-account-role"
	ServiceName                           = "StreamNative"
	KeychainName                          = "terraform"
)

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"key_file_path": "The path of the private key file, you can set it to 'KEY_FILE_PATH' " +
			"environment variable, find it in the cloud console under the service account with admin permission",
		"client_id": "Client ID of the service account, " +
			"you can set it to 'GLOBAL_DEFAULT_CLIENT_ID' environment variable",
		"client_secret": "Client Secret of the service account, " +
			"you can set it to 'GLOBAL_DEFAULT_CLIENT_SECRET' environment variable",
		"organization":                 "The organization name",
		"service_account_name":         "The service account name",
		"service_account_binding_name": "The service account binding name",
		"cluster_name":                 "The pulsar cluster name",
		"cluster_display_name":         "The pulsar cluster display name",
		"admin":                        "Whether the service account is admin",
		"private_key_data":             "The private key data",
		"secret_name":                  "The secret name",
		"secret_data":                  "The secret data map",
		"secret_string_data":           "Write-only string data that will be stored encrypted by the API server",
		"secret_type":                  "The Kubernetes secret type",
		"availability-mode":            "The availability mode, supporting 'zonal' and 'regional'",
		"pool_name":                    "The infrastructure pool name",
		"pool_namespace":               "The infrastructure pool namespace",
		"pool_type":                    "Type of infrastructure pool, one of aws, gcloud and azure",
		"pool_member_name":             "The infrastructure pool member name",
		"pool_member_namespace":        "The infrastructure pool member namespace",
		"pool_member_type":             "Type of infrastructure pool member, one of aws, gcloud and azure",
		"pool_member_location":         "The location of the infrastructure pool member",
		"instance_name":                "The pulsar instance name",
		"instance_type":                "The streamnative cloud instance type, supporting 'serverless', 'dedicated', 'byoc' and 'byoc-pro'",
		"instance_engine":              "The streamnative cloud instance engine, supporting 'ursa' and 'classic', default 'classic'",
		"location": "The location of the pulsar cluster, " +
			"supported location https://docs.streamnative.io/docs/cluster#cluster-location",
		"release_channel":         "The release channel of the pulsar cluster subscribe to, it must to be lts or rapid, default rapid",
		"bookie_replicas":         "The number of bookie replicas",
		"broker_replicas":         "The number of broker replicas",
		"compute_unit_per_broker": "compute unit per broker, 1 compute unit is 2 cpu and 8gb memory",
		"storage_unit_per_bookie": "storage unit per bookie, 1 storage unit is 2 cpu and 8gb memory",
		"cluster_ready":           "Pulsar cluster is ready, it will be set to 'True' after the cluster is ready",
		"instance_ready":          "Pulsar instance is ready, it will be set to 'True' after the instance is ready",
		"websocket_enabled":       "Whether the websocket is enabled",
		"function_enabled":        "Whether the function is enabled",
		"transaction_enabled":     "Whether the transaction is enabled",
		"kafka":                   "Controls the kafka protocol config of pulsar cluster",
		"mqtt":                    "Controls the mqtt protocol config of pulsar cluster",
		"categories": "Controls the audit log categories config of pulsar cluster, supported categories: " +
			"\"Management\", \"Describe\", \"Produce\", \"Consume\"",
		"lakehouse_type":          "The type of the lakehouse",
		"lakehouse_catalog":       "The name of the lakehouse catalog",
		"catalog_credentials":     "The credentials of the lakehouse catalog",
		"catalog_connection_url":  "The connection url of the lakehouse catalog",
		"catalog_warehouse":       "The warehouse of the lakehouse catalog",
		"custom":                  "Controls the custom config of pulsar cluster",
		"http_tls_service_url":    "The service url of the pulsar cluster, use it to management the pulsar cluster.",
		"http_tls_service_urls":   "The service url of the pulsar cluster, use it to management the pulsar cluster. There'll be multiple service urls if the cluster attached with multiple gateways",
		"pulsar_tls_service_url":  "The service url of the pulsar cluster, use it to produce and consume message.",
		"pulsar_tls_service_urls": "The service url of the pulsar cluster, use it to produce and consume message. There'll be multiple service urls if the cluster attached with multiple gateways",
		"kafka_service_url": "If you want to connect to the pulsar cluster using the kafka protocol," +
			" use this kafka service url.",
		"kafka_service_urls": "If you want to connect to the pulsar cluster using the kafka protocol," +
			" use this kafka service url. There'll be multiple service urls if the cluster attached with multiple gateways",
		"mqtt_service_url": "If you want to connect to the pulsar cluster using the mqtt protocol, " +
			"use this mqtt service url.",
		"mqtt_service_urls": "If you want to connect to the pulsar cluster using the mqtt protocol, " +
			"use this mqtt service url.  There'll be multiple service urls if the cluster attached with multiple gateways",
		"websocket_service_url": "If you want to connect to the pulsar cluster using the websocket protocol, " +
			"use this websocket service url.",
		"websocket_service_urls": "If you want to connect to the pulsar cluster using the websocket protocol, " +
			"use this websocket service url. There'll be multiple service urls if the cluster attached with multiple gateways",
		"pulsar_version":         "The version of the pulsar cluster",
		"bookkeeper_version":     "The version of the bookkeeper cluster",
		"type":                   "Type of cloud connection, one of aws or gcp",
		"aws":                    "AWS configuration for the connection",
		"gcp":                    "GCP configuration for the connection",
		"azure":                  "Azure configuration for the connection",
		"cloud_connection_name":  "Name of the cloud connection",
		"environment_type":       "Type of the cloud environment, either: dev, test, staging, production, acc, qa or poc",
		"cloud_environment_name": "Name of the cloud environment",
		"region":                 "The region of the cloud environment, for Azure, it should be the resource group name",
		"zone":                   "The zone of the cloud environment, the underlying infrastructure will only be created in this zone if configured",
		"default_gateway":        "The default gateway of the cloud environment",
		"apikey_name":            "The name of the api key",
		"apikey_description":     "The description of the api key",
		"revoke": "Whether to revoke the api key, if set to true, the api key will be revoked." +
			" By default, after revoking an apikey object, all connections using that apikey will" +
			" fail after 1 minute due to an authentication exception." +
			" if you want delete api key, please revoke this api key first",
		"apikey_ready":    "Apikey is ready, it will be set to 'True' after the api key is ready",
		"token":           "The token of the api key",
		"issued_at":       "The timestamp of when the key was issued, stored as an epoch in seconds",
		"expires_at":      "The timestamp of when the key expires",
		"revoked_at":      "The timestamp of when the key was revoked",
		"encrypted_token": "The encrypted security token issued for the key",
		"key_id":          "The key id of apikey",
		"private_key":     "The private key for decrypting the encrypted token",
		"expiration_time": "The expiration time of the api key, you can set it to " +
			"1m(one minute), 1h(one hour), 1d(one day) or this time format 2025-05-08T15:30:00Z, " +
			"if you set it '0', it will never expire, " +
			"if you don't set it, it will be set to 30d(30 days) by default",
		"wait_for_completion":     "If true, will block until the status of resource has a Ready condition",
		"resource_name":           fmt.Sprintf("The name of StreamNative Cloud resource, should be plural format, valid values are %q.", strings.Join(validResourceNames, ", ")),
		"gateway_name":            "The name of the pulsar gateway",
		"gateway_access":          "The access type of the pulsar gateway, valid values are 'public' and 'private'",
		"gateway_private_service": "The private service configuration of the pulsar gateway, only can be configured when access is private",
		"gateway_allowed_ids": "The whitelist of the private service, only can be configured when access is private." +
			"They are account ids in AWS, the project names in GCP, and the subscription ids in Azure",
		"gateway_private_service_ids": "The private service ids are ids are service names of PrivateLink in AWS, " +
			"the ids of Private Service Attachment in GCP, " +
			"and the aliases of PrivateLinkService in Azure.",
		"gateway_ready":                   "Pulsar gateway is ready, it will be set to 'True' after the gateway is ready",
		"default_gateway_name":            "The name of the pulsar gateway",
		"default_gateway_access":          "The access type of the pulsar gateway, valid values are 'public' and 'private'",
		"default_gateway_private_service": "The private service configuration of the pulsar gateway, only can be configured when access is private",
		"default_gateway_allowed_ids": "The whitelist of the private service, only can be configured when access is private." +
			"They are account ids in AWS, the project names in GCP, and the subscription ids in Azure",
		"default_gateway_private_service_ids": "The private service ids are ids are service names of PrivateLink in AWS, " +
			"the ids of Private Service Attachment in GCP, " +
			"and the aliases of PrivateLinkService in Azure.",
		"oauth2_issuer_url":                    "The issuer url of the oauth2",
		"oauth2_audience":                      "The audience of the oauth2",
		"annotations":                          "The metadata annotations of the resource",
		"rolebinding_ready":                    "The RoleBinding is ready, it will be set to 'True' after the cluster is ready",
		"rolebinding_name":                     "The name of rolebinding",
		"rolebinding_cluster_role_name":        "The predefined role name",
		"rolebinding_service_account_names":    "The list of service accounts that are role binding names ",
		"dns":                                  "The DNS ID and name. Must specify together",
		"rolebinding_user_names":               "The list of users that are role binding names ",
		"rolebinding_condition_cel":            "The conditional role binding CEL(Common Expression Language) expression",
		"rolebinding_condition_resource_names": "The list of conditional role binding resource names",
		"rolebinding_condition_resource_names_organization":    "The conditional role binding resource name - organization",
		"rolebinding_condition_resource_names_instance":        "The conditional role binding resource name - instance",
		"rolebinding_condition_resource_names_cluster":         "The conditional role binding resource name - cluster",
		"rolebinding_condition_resource_names_tenant":          "The conditional role binding resource name - tenant",
		"rolebinding_condition_resource_names_namespace":       "The conditional role binding resource name - namespace",
		"rolebinding_condition_resource_names_topic_domain":    "The conditional role binding resource name - topic domain(persistent/non-persistent)",
		"rolebinding_condition_resource_names_topic_name":      "The conditional role binding resource name - topic name",
		"rolebinding_condition_resource_names_subscription":    "The conditional role binding resource name - subscription",
		"rolebinding_condition_resource_names_service_account": "The conditional role binding resource name - service account",
		"rolebinding_condition_resource_names_secret":          "The conditional role binding resource name - secret",
		"volume_name":                   "The name of the volume",
		"bucket":                        "The bucket name",
		"path":                          "The path of the bucket",
		"bucket_region":                 "The region of the bucket",
		"role_arn":                      "The role arn of the bucket, it is used to access the bucket",
		"volume_ready":                  "Volume is ready, it will be set to 'True' after the volume is ready",
		"catalog_name":                  "The name of the catalog",
		"catalog_mode":                  "The catalog mode, either MANAGED or EXTERNAL",
		"catalog_unity":                 "Unity catalog configuration",
		"catalog_unity_name":            "The name of the unity catalog",
		"catalog_unity_schema_name":     "The schema name of the unity catalog",
		"catalog_unity_catalog_name":    "The catalog name of the unity catalog",
		"catalog_tabular":               "Tabular catalog configuration",
		"catalog_polaris":               "Polaris catalog configuration",
		"catalog_open_catalog":          "Open catalog configuration",
		"catalog_s3_table":              "S3 table catalog configuration",
		"catalog_s3_table_bucket":       "S3 table bucket ARN. Must be in format: arn:aws:s3tables:region:account:bucket/name (e.g., arn:aws:s3tables:ap-northeast-1:592060915564:bucket/test-s3-table-bucket)",
		"catalog_s3_table_region":       "AWS region extracted from S3 table bucket ARN or name",
		"catalog_uri":                   "The URI of the catalog connection",
		"catalog_secret":                "The secret name for the catalog connection",
		"catalog_custom":                "Custom configurations for the catalog connection",
		"catalog_ready":                 "Catalog is ready, it will be set to 'True' after the catalog is ready",
		"catalog":                       "The name of the catalog to use for this pulsar cluster",
		"apply_lakehouse_to_all_topics": "Whether to apply lakehouse storage to all topics in the cluster",
		"lakehouse_storage":             "Controls the lakehouse storage config of pulsar cluster",
		"iam_policy":                    "IAM policy JSON for S3Table catalog access. This policy should be applied to your AWS IAM role to allow access to S3Table resources.",
		"principal_name":                "The principal name of apikey, it is the principal name of the service account that the apikey is associated with, it is used to grant permission on pulsar side",
		"customized_metadata":           "The custom metadata in the api key token",
		"enable_iam_account_creation":   "Whether to create an IAM account for the service account binding",
		"aws_assume_role_arns":          "A list of AWS IAM roles' arn which can be assumed by the AWS IAM role created for the service account binding",
	}
}

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"key_file_path": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KEY_FILE_PATH", nil),
				Description: descriptions["key_file_path"],
			},
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("GLOBAL_DEFAULT_CLIENT_ID", nil),
				Description: descriptions["client_id"],
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("GLOBAL_DEFAULT_CLIENT_SECRET", nil),
				Description: descriptions["client_secret"],
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"streamnative_service_account":         resourceServiceAccount(),
			"streamnative_service_account_binding": resourceServiceAccountBinding(),
			"streamnative_pulsar_instance":         resourcePulsarInstance(),
			"streamnative_pulsar_cluster":          resourcePulsarCluster(),
			"streamnative_cloud_connection":        resourceCloudConnection(),
			"streamnative_cloud_environment":       resourceCloudEnvironment(),
			"streamnative_apikey":                  resourceApiKey(),
			"streamnative_pulsar_gateway":          resourcePulsarGateway(),
			"streamnative_rolebinding":             resourceRoleBinding(),
			"streamnative_volume":                  resourceVolume(),
			"streamnative_catalog":                 resourceCatalog(),
			"streamnative_secret":                  resourceSecret(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"streamnative_service_account":         dataSourceServiceAccount(),
			"streamnative_service_account_binding": dataSourceServiceAccountBinding(),
			"streamnative_pulsar_instance":         dataSourcePulsarInstance(),
			"streamnative_pulsar_cluster":          dataSourcePulsarCluster(),
			"streamnative_cloud_connection":        dataSourceCloudConnection(),
			"streamnative_cloud_environment":       dataSourceCloudEnvironment(),
			"streamnative_apikey":                  dataSourceApiKey(),
			"streamnative_pool":                    dataSourcePool(),
			"streamnative_pool_member":             dataSourcePoolMember(),
			"streamnative_resources":               dataSourceResources(),
			"streamnative_pulsar_gateway":          dataSourcePulsarGateway(),
			"streamnative_rolebinding":             dataSourceRoleBinding(),
			"streamnative_volume":                  dataSourceVolume(),
			"streamnative_catalog":                 dataSourceCatalog(),
			"streamnative_secret":                  dataSourceSecret(),
		},
	}
	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return providerConfigure(d, provider.TerraformVersion)
	}
	return provider
}

func providerConfigure(d *schema.ResourceData, terraformVersion string) (interface{}, diag.Diagnostics) {
	_ = terraformVersion

	defaultIssuer := os.Getenv("GLOBAL_DEFAULT_ISSUER")
	if defaultIssuer == "" {
		defaultIssuer = GlobalDefaultIssuer
	}
	defaultAudience := os.Getenv("GLOBAL_DEFAULT_AUDIENCE")
	if defaultAudience == "" {
		defaultAudience = GlobalDefaultAudience
	}
	defaultAPIServer := os.Getenv("GLOBAL_DEFAULT_API_SERVER")
	if defaultAPIServer == "" {
		defaultAPIServer = GlobalDefaultAPIServer
	}
	clientId := d.Get("client_id").(string)
	clientSecret := d.Get("client_secret").(string)
	keyFilePath := d.Get("key_file_path").(string)
	configDir, err := getConfigDir(clientId, clientSecret, keyFilePath)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	var keyFile *auth.KeyFile
	var flow *auth.ClientCredentialsFlow
	var grant *auth.AuthorizationGrant
	var issuer auth.Issuer
	if clientId != "" && clientSecret != "" {
		keyFile = &auth.KeyFile{
			ClientID:     clientId,
			ClientSecret: clientSecret,
		}
		issuer = auth.Issuer{
			IssuerEndpoint: defaultIssuer,
			ClientID:       keyFile.ClientID,
			Audience:       defaultAudience,
		}
		authorizationGrant := &auth.AuthorizationGrant{
			Type:              auth.GrantTypeClientCredentials,
			ClientCredentials: keyFile,
		}

		refresher, err := auth.NewDefaultClientCredentialsGrantRefresher(issuer, clock.RealClock{})
		if err != nil {
			return nil, diag.FromErr(err)
		}
		grant, err = refresher.Refresh(authorizationGrant)
		if err != nil {
			return nil, diag.FromErr(err)
		}
	} else {
		credsProvider := auth.NewClientCredentialsProviderFromKeyFile(keyFilePath)
		keyFile, err = credsProvider.GetClientCredentials()
		if err != nil {
			return nil, diag.FromErr(err)
		}
		issuer = auth.Issuer{
			IssuerEndpoint: defaultIssuer,
			ClientID:       keyFile.ClientID,
			Audience:       defaultAudience,
		}
		flow, err = auth.NewDefaultClientCredentialsFlow(issuer, keyFilePath)
		if err != nil {
			return nil, diag.FromErr(err)
		}
		grant, err = flow.Authorize()
		if err != nil {
			return nil, diag.FromErr(err)
		}
	}
	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	options := cmd.NewOptions(streams)
	options.ConfigDir = configDir
	options.ConfigPath = filepath.Join(configDir, "config")
	options.BackendOverride = "file"
	snConfig := &config.SnConfig{
		Server:                   defaultAPIServer,
		CertificateAuthorityData: base64.StdEncoding.EncodeToString([]byte(GlobalDefaultCertificateAuthorityData)),
		Auth: config.Auth{
			IssuerEndpoint: defaultIssuer,
			Audience:       defaultAudience,
			ClientID:       keyFile.ClientID,
		},
	}
	err = options.SaveConfig(snConfig)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	apc := &clientcmdapi.AuthProviderConfig{
		Name: "streamnative",
	}
	// Pre-check if the auth provider is already exist for avoid issue
	// auth Provider Plugin streamnative was registered twice
	provider, _ := rest.GetAuthProvider("", apc, nil)
	if provider == nil {
		err = options.Complete()
		if err != nil {
			return nil, diag.FromErr(err)
		}
	} else {
		kr, err := makeKeyring(options.BackendOverride, options.ConfigDir)
		if err != nil {
			return nil, diag.FromErr(err)
		}
		options.Store, err = store.NewKeyringStore(kr)
		if err != nil {
			return nil, diag.FromErr(err)
		}
		options.Factory, err = plugin.NewDefaultFactory(options.Store, func() (auth.Issuer, error) {
			return issuer, nil
		})
		if err != nil {
			return nil, diag.FromErr(err)
		}
		err = options.ServerOptions.Complete(options)
		if err != nil {
			return nil, diag.FromErr(err)
		}
	}
	err = options.Store.SaveGrant(issuer.Audience, *grant)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	factory := cmdutil.NewFactory(options)
	return factory, nil
}

func makeKeyring(backendOverride string, configDir string) (keyring.Keyring, error) {
	var backends []keyring.BackendType
	if backendOverride != "" {
		backends = append(backends, keyring.BackendType(backendOverride))
	}

	return keyring.Open(keyring.Config{
		ServiceName:              ServiceName,
		KeychainName:             KeychainName,
		KeychainTrustApplication: true,
		AllowedBackends:          backends,
		FileDir:                  filepath.Join(configDir, "credentials"),
		FilePasswordFunc:         keyringPrompt,
	})
}

func keyringPrompt(prompt string) (string, error) {
	return "", nil
}

// getConfigDir generate a unique configuration directory based on the provided arguments
func getConfigDir(clientId, clientSecret, keyFilePath string) (string, error) {
	home, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}
	combined := fmt.Sprintf("%s|%s|%s", keyFilePath, clientId, clientSecret)
	hash := sha256.Sum256([]byte(combined))
	dirName := fmt.Sprintf(".streamnative_%x", hash[:8])

	configDir := filepath.Join(home, dirName)

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err = os.MkdirAll(configDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %v", err)
		}
	}
	return configDir, nil
}
