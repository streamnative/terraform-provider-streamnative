package cloud

import (
	"context"
	"encoding/base64"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/go-homedir"
	"github.com/streamnative/cloud-cli/pkg/auth"
	"github.com/streamnative/cloud-cli/pkg/auth/store"
	"github.com/streamnative/cloud-cli/pkg/cmd"
	"github.com/streamnative/cloud-cli/pkg/config"
	"github.com/streamnative/cloud-cli/pkg/plugin"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
	"path/filepath"
)

const (
	GlobalDefaultIssuer                   = "https://auth.streamnative.cloud/"
	GlobalDefaultAudience                 = "https://api.streamnative.cloud"
	GlobalDefaultAPIServer                = "https://api.streamnative.cloud"
	GlobalDefaultCertificateAuthorityData = ``
)

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"key_file_path":     "The path of the private key file",
		"organization":      "The organization name",
		"name":              "The service account name",
		"admin":             "Whether the service account is admin",
		"private_key_data":  "The private key data",
		"availability-mode": "The availability mode, supporting 'zonal' and 'regional'",
		"pool_name":         "The infrastructure pool name to use.",
		"pool_namespace":    "The infrastructure pool namespace to use",
		"instance_name":     "The pulsar instance name",
		"location":          "The location of the pulsar cluster",
		"bookie_replicas":   "The number of bookie replicas",
		"broker_replicas":   "The number of broker replicas",
		"compute_unit":      "compute unit, 1 compute unit is 2 cpu and 8gb memory",
		"storage_unit":      "storage unit, 1 storage unit is 2 cpu and 8gb memory",
		"cluster_ready":     "Pulsar cluster is ready, it will be set to 'True' after the cluster is ready",
		"instance_ready":    "Pulsar instance is ready, it will be set to 'True' after the instance is ready",
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
	defaultApiServer := os.Getenv("GLOBAL_DEFAULT_API_SERVER")
	if defaultApiServer == "" {
		defaultApiServer = GlobalDefaultAPIServer
	}
	credsProvider := auth.NewClientCredentialsProviderFromKeyFile(keyFilePath)
	keyFile, err := credsProvider.GetClientCredentials()
	if err != nil {
		return nil, diag.FromErr(err)
	}
	issuer := auth.Issuer{
		IssuerEndpoint: defaultIssuer,
		ClientID:       keyFile.ClientID,
		Audience:       defaultAudience,
	}
	flow, err := auth.NewDefaultClientCredentialsFlow(issuer, keyFilePath)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	grant, err := flow.Authorize()
	if err != nil {
		return nil, diag.FromErr(err)
	}
	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	options := cmd.NewOptions(streams)
	options.ConfigDir = configDir
	options.ConfigPath = filepath.Join(configDir, "config")
	options.BackendOverride = "memory"
	snConfig := &config.SnConfig{
		Server:                   defaultApiServer,
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
		options.Store = store.NewMemoryStore()
		options.Factory, err = plugin.NewDefaultFactory(options.Store, func() (auth.Issuer, error) {
			return issuer, nil
		})
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
