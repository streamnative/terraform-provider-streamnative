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
		"private_key_data": "The private key data",
	}
}

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["client_id"],
			},
			"key_file_path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["key_file_path"],
				DefaultFunc: schema.MultiEnvDefaultFunc([]string{"PULSAR_KEY_FILE", "PULSAR_KEY_FILE_PATH"}, ""),
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
	issuer := auth.Issuer{
		IssuerEndpoint: GlobalDefaultIssuer,
		ClientID:       clientID,
		Audience:       GlobalDefaultAudience,
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
	snConfig := &config.SnConfig{
		Server:                   GlobalDefaultAPIServer,
		CertificateAuthorityData: base64.StdEncoding.EncodeToString([]byte(GlobalDefaultCertificateAuthorityData)),
		Auth: config.Auth{
			IssuerEndpoint: GlobalDefaultIssuer,
			Audience:       GlobalDefaultAudience,
			ClientID:       clientID,
		},
	}
	err = options.SaveConfig(snConfig)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	err = options.Complete()
	if err != nil {
		return nil, diag.FromErr(err)
	}
	err = options.Store.SaveGrant(issuer.Audience, *grant)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	factory := cmdutil.NewFactory(options)
	return factory, nil
}
