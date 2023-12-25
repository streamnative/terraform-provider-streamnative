package cloud

import (
	"context"
	"encoding/base64"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/go-homedir"
	"github.com/streamnative/cloud-cli/pkg/auth"
	"github.com/streamnative/cloud-cli/pkg/cmd"
	"github.com/streamnative/cloud-cli/pkg/config"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
		"client_id":        "The OAuth 2.0 client identifier",
		"key_file_path":    "The path of the private key file",
		"organization":     "The organization name",
		"name":             "The service account name",
		"admin":            "Whether the service account is admin",
		"private_key_data": "The private key data",
	}
}

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"client_id": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("CLIENT_ID", nil),
				Description: descriptions["client_id"],
			},
			"key_file_path": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KEY_FILE_PATH", nil),
				Description: descriptions["key_file_path"],
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"streamnative_service_account": resourceServiceAccount(),
		},
	}
	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return providerConfigure(d, provider.TerraformVersion)
	}
	return provider
}

func providerConfigure(d *schema.ResourceData, terraformVersion string) (interface{}, diag.Diagnostics) {
	_ = terraformVersion

	clientID := d.Get("client_id").(string)
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
	issuer := auth.Issuer{
		IssuerEndpoint: defaultIssuer,
		ClientID:       clientID,
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
			ClientID:       clientID,
		},
	}
	err = options.SaveConfig(snConfig)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	if options.AuthOptions.Factory == nil {
		err = options.Complete()
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
