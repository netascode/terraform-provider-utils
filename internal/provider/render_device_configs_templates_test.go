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
	"reflect"
	"testing"
)

// Template expressions left in the output of file templates (from variables
// referring to other variables, or escaped as `$${...}`) are rendered by the
// final template pass also when nested in maps and lists, as in model templates.
// Nested null values of file templates are removed like those of model templates.
func TestRenderDeviceConfigs_FileTemplateNestedExpressions(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: global_file
      type: file
      file: global.yaml
    - name: group_file
      type: file
      file: group.yaml
    - name: device_file
      type: file
      file: device.yaml
    - name: instance_file
      type: file
      file: instance.yaml
  global:
    templates: [global_file]
    variables:
      site: DC1
      global_desc: "${site}-global"
  device_groups:
    - name: LEAFS
      templates: [group_file]
      variables:
        group_desc: "${site}-group"
  devices:
    - name: leaf1
      device_groups: [LEAFS]
      templates: [device_file]
      variables:
        device_desc: "${DEVICE.name}-device"
  template_instances:
    - name: EP
      template: instance_file
      devices: [leaf1]
      variables: {site: DC2, desc: "${site}-${port}"}
      items:
        - variables: {port: "1/5"}
`)
	fileTemplates := map[string]string{
		"global.yaml": `system:
  hostname: null
interfaces:
  ethernets:
    - id: "1/1"
      description: "${global_desc}"
`,
		"group.yaml": `interfaces:
  ethernets:
    - id: "1/2"
      description: "${group_desc}"
`,
		"device.yaml": `interfaces:
  ethernets:
    - id: "1/3"
      description: "${device_desc}"
    - id: "1/4"
      description: "$${upper(DEVICE.name)}"
`,
		"instance.yaml": `interfaces:
  ethernets:
    - id: "${port}"
      description: "${desc}"
`,
	}
	want := map[string]string{
		"1/1": "DC1-global",
		"1/2": "DC1-group",
		"1/3": "leaf1-device",
		"1/4": "LEAF1",
		"1/5": "DC2-1/5",
	}
	for _, resolve := range []bool{false, true} {
		result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		if got := renderedEthernets(t, result, "leaf1"); !reflect.DeepEqual(got, want) {
			t.Errorf("resolve=%v: ethernets = %v, want %v", resolve, got, want)
		}
		stripped := stripNulls(result).(map[string]any)
		system := getMapVal(getMapVal(renderedDevice(t, stripped, "leaf1"), "configuration"), "system")
		if _, ok := system["hostname"]; ok {
			t.Errorf("resolve=%v: system = %v, want null hostname removed", resolve, system)
		}
	}
}

// CLI templates with the same name are rendered once per device: the entry of
// the most specific level (global, device group, template instance, device
// template, direct cli_templates entry) replaces earlier ones in place, so that
// the names stay unique (the Terraform modules use them as resource keys).
func TestRenderDeviceConfigs_DuplicateCliTemplateNames(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: banner
      type: cli
      content: "banner motd ${msg}"
    - name: ntp
      type: cli
      content: "ntp server ${ntp}"
      order: 5
  global:
    templates: [banner, ntp, banner]
    variables: {msg: global, ntp: 1.1.1.1}
  device_groups:
    - name: LEAFS
      templates: [banner, banner]
      variables: {msg: group}
  devices:
    - name: leaf1
      device_groups: [LEAFS]
      templates: [banner]
      variables: {msg: device}
      cli_templates:
        - name: ntp
          content: "ntp server 2.2.2.2"
        - name: extra
          content: "first"
        - name: extra
          content: "second"
    - name: leaf2
`)
	type cli struct {
		name, content string
		order         int
	}
	want := map[string][]cli{
		"leaf1": {
			{"banner", "banner motd device", 0},
			{"ntp", "ntp server 2.2.2.2", 0},
			{"banner/LEAFS", "banner motd group", 0},
			{"extra", "second", 0},
		},
		"leaf2": {
			{"banner", "banner motd global", 0},
			{"ntp", "ntp server 1.1.1.1", 5},
		},
	}
	result, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, w := range want {
		var got []cli
		for _, c := range getSliceVal(renderedDevice(t, result, name), "cli_templates") {
			m := toMapStringAny(c)
			got = append(got, cli{getStringVal(m, "name", ""), getStringVal(m, "content", ""), getIntVal(m, "order", -1)})
		}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: cli_templates = %v, want %v", name, got, w)
		}
	}
}
