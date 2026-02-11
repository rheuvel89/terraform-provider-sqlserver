package main

import (
	"terraform-provider-sqlserver/sqlserver"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

// These will be set by goreleaser to appropriate values for the compiled binary
var (
	version string = "dev"
	commit  string = "none"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: sqlserver.New(version, commit),
	})
}
