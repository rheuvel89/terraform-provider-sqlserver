package sqlserver

import (
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// enumKeys returns the friendly names of an enum map, sorted, for validation and docs.
func enumKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// enumName returns the friendly name of an msdb value, or "" when the value is unknown.
func enumName(m map[string]int, value int) string {
	for k, v := range m {
		if v == value {
			return k
		}
	}
	return ""
}

func enumValidation(m map[string]int) schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(validation.StringInSlice(enumKeys(m), false))
}
