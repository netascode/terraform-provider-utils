// Copyright © 2022 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Mozilla Public License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mozilla.org/MPL/2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: MPL-2.0

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

func TestYamlMergeFunction_EmptyDocuments(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFunctionUtilsYamlMerge_emptyDocs(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_empty_string", "foo: bar\n"),
					resource.TestCheckOutput("test_comment_only", "foo: bar\n"),
					resource.TestCheckOutput("test_whitespace_only", "foo: bar\n"),
					resource.TestCheckOutput("test_empty_between", "foo: bar\n"),
				),
			},
		},
	})
}

// TestYamlMergeFunction_ScientificNotationString verifies that strings matching
// scientific notation patterns survive the decode→merge→encode round-trip with
// their type intact (issue #155 regression test).
func TestYamlMergeFunction_ScientificNotationString(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					input = <<-EOT
					secret_key: "23211e010211"
					api_key: "1e10"
					EOT
				}
				output "test" {
					value = provider::utils::yaml_merge([local.input])
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "secret_key: \"23211e010211\"\napi_key: \"1e10\"\n"),
				),
			},
		},
	})
}

// TestYamlMergeFunction_ControlCharacters verifies that \r, \t, and other C0
// control characters survive the YAML decode → merge → encode round-trip.
// This is a regression test for issue #174 where v2.0.0 lost these characters
// because the encoder chose block scalar style (|), which cannot represent CR or TAB.
func TestYamlMergeFunction_ControlCharacters(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The YAML below uses YAML double-quoted escape sequences (\r, \n, \t).
				// The HCL heredoc passes them through as literal backslash sequences,
				// so the YAML parser decodes \r → CR (0x0D), \t → TAB (0x09), etc.
				Config: `
				locals {
					input = <<-EOT
					banner:
					  crlf: "line1\r\nline2\n"
					  tab: "col1\tcol2\n"
					EOT
				}
				output "test" {
					value = provider::utils::yaml_merge([local.input])
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "banner:\n  crlf: \"line1\\r\\nline2\\n\"\n  tab: \"col1\\tcol2\\n\"\n"),
				),
			},
		},
	})
}

func testAccFunctionUtilsYamlMerge_emptyDocs() string {
	return `
	locals {
		valid = <<-EOT
		foo: bar
		EOT
	}

	output "test_empty_string" {
		value = provider::utils::yaml_merge(["", local.valid])
	}

	output "test_comment_only" {
		value = provider::utils::yaml_merge(["# just a comment\n# another comment\n", local.valid])
	}

	output "test_whitespace_only" {
		value = provider::utils::yaml_merge(["   \n  \n", local.valid])
	}

	output "test_empty_between" {
		value = provider::utils::yaml_merge([local.valid, "", local.valid])
	}
	`
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
