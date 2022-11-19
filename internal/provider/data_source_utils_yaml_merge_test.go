package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceUtilsYamlMerge(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUtilsYamlMerge_config(basic_inputYaml1, basic_inputYaml2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.utils_yaml_merge.test", "output", basic_ouputYaml),
				),
			},
		},
	})
}

func testAccDataSourceUtilsYamlMerge_config(yaml1, yaml2 string) string {
	return fmt.Sprintf(`
	locals {
		yaml1 = <<-EOT%sEOT
		yaml2 = <<-EOT%sEOT
	}

	data "utils_yaml_merge" "test" {
		input = [local.yaml1, local.yaml2]
	}
	`, yaml1, yaml2)
}

const basic_inputYaml1 = `
root:
  elem1: value1
  child1:
    cc1: 1
list:
  - name: a1
    map:
      a1: 1
      b1: 1
  - name: a2
`

const basic_inputYaml2 = `
root:
  elem2: value2
  child1:
    cc2: 2
list:
  - name: a1
    map:
      a2: 2
  - name: a3
`

const basic_ouputYaml = `list:
    - map:
        a1: 1
        a2: 2
        b1: 1
      name: a1
    - name: a2
    - name: a3
root:
    child1:
        cc1: 1
        cc2: 2
    elem1: value1
    elem2: value2
`
