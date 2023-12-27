package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/streamnative/terraform-provider-streamnative/cloud"
)

// Run "go generate" to generate the docs for the registry/website
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.13.0

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: cloud.Provider,
	})
}
