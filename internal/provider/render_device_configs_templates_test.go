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
	"reflect"
	"strings"
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

// Names in the templates lists of global, device groups and devices must be
// templates of type file, model or cli. All problems are reported at once,
// together with those of template instances.
func TestRenderDeviceConfigs_UnknownTemplateReferences(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: base
      type: model
      configuration: {system: {hostname: x}}
    - name: odd
      type: jinja
  global:
    templates: [base, missing_global]
  device_groups:
    - name: LEAFS
      templates: [missing_group, odd]
    - templates: [missing_unnamed]
  devices:
    - name: leaf1
      templates: [base, missing_device]
  template_instances:
    - name: SVC
      template: missing_instance
      devices: [leaf1]
`)
	_, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{
		`invalid template references:`,
		`global.templates: unknown template "missing_global"`,
		`device_groups[LEAFS].templates: unknown template "missing_group"`,
		`device_groups[LEAFS].templates: template "odd" has unsupported type "jinja"`,
		`device_groups[1].templates: unknown template "missing_unnamed"`,
		`devices[leaf1].templates: unknown template "missing_device"`,
		`template_instances[SVC]: unknown template "missing_instance"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), `"base"`) {
		t.Errorf("error mentions the valid template base:\n%v", err)
	}
}

// fail stops rendering with its message in file, model and CLI templates,
// configuration values and template instances, only in the selected branch of
// a conditional, and can be caught with try/can.
func TestRenderDeviceConfigs_FailFunction(t *testing.T) {
	const model = `
nxos:
  templates:
    - name: file_tmpl
      type: file
      file: check.yaml
    - name: model_tmpl
      type: model
      configuration:
        system:
          description: '${vlan < 4000 ? "ok" : fail("model: VLAN ${vlan} is reserved")}'
    - name: cli_tmpl
      type: cli
      content: '${vlan < 4000 ? "ok" : fail("cli: VLAN ${vlan} is reserved")}'
  devices:
    - name: leaf1
      templates: [%s]
      variables: {vlan: %d}
      configuration:
        system:
          location: '${try(fail("caught"), "fallback")}'
`
	fileTemplates := map[string]string{
		"check.yaml": `%{ if vlan >= 4000 }${fail("file: VLAN ${vlan} is reserved on ${DEVICE.name}.")}%{ endif }
system:
  hostname: "${vlan}"
`,
	}
	for _, tmpl := range []string{"file_tmpl", "model_tmpl", "cli_tmpl"} {
		result, err := renderDeviceConfigs(mustYamlModel(t, fmt.Sprintf(model, tmpl, 10)), fileTemplates, nil, nil, nil, false)
		if err != nil {
			t.Fatalf("%s with VLAN 10: %v", tmpl, err)
		}
		system := getMapVal(getMapVal(renderedDevice(t, result, "leaf1"), "configuration"), "system")
		if got := getStringVal(system, "location", ""); got != "fallback" {
			t.Errorf("%s: location = %q, want fallback", tmpl, got)
		}
	}
	for tmpl, want := range map[string]string{
		"file_tmpl":  `Call to function "fail" failed: file: VLAN 4001 is reserved on leaf1.`,
		"model_tmpl": `Call to function "fail" failed: model: VLAN 4001 is reserved.`,
		"cli_tmpl":   `Call to function "fail" failed: cli: VLAN 4001 is reserved.`,
	} {
		_, err := renderDeviceConfigs(mustYamlModel(t, fmt.Sprintf(model, tmpl, 4001)), fileTemplates, nil, nil, nil, false)
		if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "leaf1..") {
			t.Errorf("%s with VLAN 4001: error = %v, want %q", tmpl, err, want)
		}
	}
}
