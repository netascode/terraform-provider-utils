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
