package sqlserver

import (
	"testing"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceClassifierFunction_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceClassifierFunctionConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("sqlserver_classifier_function.test", "name", "test_classifier"),
				),
			},
		},
	})
}

const testAccResourceClassifierFunctionConfig_basic = `
resource "sqlserver_classifier_function" "test" {
  name = "test_classifier"
  definition = "RETURN 'default';"
}
`
