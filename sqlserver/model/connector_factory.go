package model

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ConnectorFactory interface {
  GetConnector(data *schema.ResourceData, host string, port string, login interface{}) (interface{}, error)
}
