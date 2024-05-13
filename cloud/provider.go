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
	"encoding/base64"
	"k8s.io/utils/clock"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
	"github.com/streamnative/cloud-cli/pkg/auth/store"
	"github.com/streamnative/cloud-cli/pkg/plugin"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/go-homedir"
	"github.com/streamnative/cloud-cli/pkg/auth"
	"github.com/streamnative/cloud-cli/pkg/cmd"
	"github.com/streamnative/cloud-cli/pkg/config"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
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
		"key_file_path":        "The path of the private key file",
		"organization":         "The organization name",
		"service_account_name": "The service account name",
		"cluster_name":         "The pulsar cluster name",
		"admin":                "Whether the service account is admin",
		"private_key_data":     "The private key data",
		"availability-mode":    "The availability mode, supporting 'zonal' and 'regional'",
		"pool_name":            "The infrastructure pool name to use, supported pool 'shared-aws', 'shared-gcp'",
		"pool_namespace":       "The infrastructure pool namespace to use, supported 'streamnative'",
		"instance_name":        "The pulsar instance name",
		"location": "The location of the pulsar cluster, " +
			"supported location https://docs.streamnative.io/docs/cluster#cluster-location",
		"release_channel":     "The release channel of the pulsar cluster subscribe to, it must to be lts or rapid, default rapid",
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
		"pulsar_version":         "The version of the pulsar cluster",
		"bookkeeper_version":     "The version of the bookkeeper cluster",
		"type":                   "Type of cloud connection, one of aws or gcp",
		"aws":                    "AWS configuration for the connection",
		"gcp":                    "GCP configuration for the connection",
		"azure":                  "Azure configuration for the connection",
		"cloud_connection_name":  "Name of the cloud connection",
		"environment_type":       "Type of the cloud environment, either: dev, test, staging, production, acc, qa or poc",
		"cloud_environment_name": "Name of the cloud environment",
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
		"wait_for_completion": "If true, will block until the status of CloudEnvironment has a Ready condition",
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
		},
		ResourcesMap: map[string]*schema.Resource{
			"streamnative_service_account":   resourceServiceAccount(),
			"streamnative_pulsar_instance":   resourcePulsarInstance(),
			"streamnative_pulsar_cluster":    resourcePulsarCluster(),
			"streamnative_cloud_connection":  resourceCloudConnection(),
			"streamnative_cloud_environment": resourceCloudEnvironment(),
			"streamnative_apikey":            resourceApiKey(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"streamnative_service_account":   dataSourceServiceAccount(),
			"streamnative_pulsar_instance":   dataSourcePulsarInstance(),
			"streamnative_pulsar_cluster":    dataSourcePulsarCluster(),
			"streamnative_cloud_connection":  dataSourceCloudConnection(),
			"streamnative_cloud_environment": dataSourceCloudEnvironment(),
			"streamnative_apikey":            dataSourceApiKey(),
		},
	}
	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return providerConfigure(d, provider.TerraformVersion)
	}
	return provider
}

func providerConfigure(d *schema.ResourceData, terraformVersion string) (interface{}, diag.Diagnostics) {
	_ = terraformVersion

	home, err := homedir.Dir()
	if err != nil {
		return nil, diag.FromErr(err)
	}
	configDir := filepath.Join(home, ".streamnative")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err = os.MkdirAll(configDir, 0755); err != nil {
			return nil, diag.FromErr(err)
		}
	}
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
	defaultClientId := os.Getenv("GLOBAL_DEFAULT_CLIENT_ID")
	defaultClientSecret := os.Getenv("GLOBAL_DEFAULT_CLIENT_SECRET")
	//defaultClientEmail := os.Getenv("GLOBAL_DEFAULT_CLIENT_EMAIL")
	var keyFile *auth.KeyFile
	var flow *auth.ClientCredentialsFlow
	var grant *auth.AuthorizationGrant
	var issuer auth.Issuer
	if defaultClientId != "" && defaultClientSecret != "" {
		keyFile = &auth.KeyFile{
			ClientID:     defaultClientId,
			ClientSecret: defaultClientSecret,
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
		keyFilePath := d.Get("key_file_path").(string)
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
