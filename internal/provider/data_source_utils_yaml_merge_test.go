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
)

func TestAccDataSourceUtilsYamlMerge(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUtilsYamlMerge_config(basic_inputYaml1, basic_inputYaml2, map[string]string{"ELEM1": "value1"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.utils_yaml_merge.test", "output", basic_ouputYaml),
				),
			},
		},
	})
}

func testAccDataSourceUtilsYamlMerge_config(yaml1, yaml2 string, envs map[string]string) string {
	for k, v := range envs {
		os.Setenv(k, v)
	}
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
  elem1: !env ELEM1
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

const basic_ouputYaml = `root:
  elem1: value1
  child1:
    cc1: 1
    cc2: 2
  elem2: value2
list:
  - name: a1
    map:
      a1: 1
      b1: 1
      a2: 2
  - name: a2
  - name: a3
`
