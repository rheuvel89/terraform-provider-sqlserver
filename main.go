package main

import (
  "github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
  "terraform-provider-sqlserver/sqlserver"
)

// These will be set by goreleaser to appropriate values for the compiled binary
var (
  version string = "dev"
  commit  string = "none"
)

func main() {
  plugin.Serve(&plugin.ServeOpts{
    ProviderFunc: mssql.New(version, commit),
  })
}
