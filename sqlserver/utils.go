package sqlserver

import (
	"fmt"
	"terraform-provider-sqlserver/sqlserver/model"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/rs/zerolog"
)

func getLoginID(meta interface{}, data *schema.ResourceData) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port

	login := data.Get("sql_login").([]interface{})
	login0 := login[0].(map[string]interface{})
	loginName := login0[loginNameProp].(string)

	loginID := fmt.Sprintf("sqlserver://%s:%s/login/%s", host, port, loginName)

	return loginID
}

func getUserID(meta interface{}, data *schema.ResourceData) string {
	provider := meta.(sqlserverProvider)
	host := provider.host
	port := provider.port
	database := data.Get(databaseProp).(string)
	username := data.Get(usernameProp).(string)
	return fmt.Sprintf("sqlserver://%s:%s/%s/%s", host, port, database, username)
}

func loggerFromMeta(meta interface{}, resource, function string) zerolog.Logger {
	return meta.(model.Provider).ResourceLogger(resource, function)
}
