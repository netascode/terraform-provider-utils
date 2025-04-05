package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestYamlMergeFunction_Known(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsYamlMerge_config(basic_inputYaml1, basic_inputYaml2, map[string]string{"ELEM1": "value1"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", basic_ouputYaml),
				),
			},
		},
	})
}

func testAccFunctionUtilsYamlMerge_config(yaml1, yaml2 string, envs map[string]string) string {
	for k, v := range envs {
		os.Setenv(k, v)
	}
	return fmt.Sprintf(`
	locals {
		yaml1 = <<-EOT%sEOT
		yaml2 = <<-EOT%sEOT
	}

	output "test" {
		value = provider::utils::yaml_merge([local.yaml1, local.yaml2])
	}
	`, yaml1, yaml2)
}
