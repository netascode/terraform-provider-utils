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
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// renderedEthernets returns id → description of the rendered ethernet interfaces of a device.
func renderedEthernets(t *testing.T, result map[string]any, name string) map[string]string {
	t.Helper()
	cfg := getMapVal(renderedDevice(t, result, name), "configuration")
	got := map[string]string{}
	for _, e := range getSliceVal(getMapVal(cfg, "interfaces"), "ethernets") {
		em := toMapStringAny(e)
		got[getStringVal(em, "id", "")] = getStringVal(em, "description", "")
	}
	return got
}

// A single file template attached to a device group selects the entries of a
// global endpoint list that apply to the device being rendered (AVD-style
// connected endpoints with `switches` per endpoint).
func TestRenderDeviceConfigs_DeviceVariableFiltersGlobalList(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: endpoints
      type: file
      file: templates/endpoints.yaml
  global:
    variables:
      endpoints:
        - name: server1
          switches: [leaf1, leaf2]
          port: "1/10"
        - name: server2
          switches: [leaf2]
          port: "1/11"
        - name: server3
          switches: [leaf3]
          port: "1/12"
  device_groups:
    - name: LEAFS
      templates: [endpoints]
      devices: [leaf3]
  devices:
    - name: leaf1
      device_groups: [LEAFS]
    - name: leaf2
      device_groups: [LEAFS]
    - name: leaf3
    - name: spine1
`)
	fileTemplates := map[string]string{
		"templates/endpoints.yaml": `interfaces:
  ethernets:
%{ for e in endpoints ~}
%{ if contains(e.switches, DEVICE.name) ~}
    - id: "${e.port}"
      description: "${e.name}"
%{ endif ~}
%{ endfor ~}
`,
	}
	want := map[string]map[string]string{
		"leaf1":  {"1/10": "server1"},
		"leaf2":  {"1/10": "server1", "1/11": "server2"},
		"leaf3":  {"1/12": "server3"},
		"spine1": {},
	}
	for _, resolve := range []bool{false, true} {
		result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		for name, w := range want {
			if got := renderedEthernets(t, result, name); !reflect.DeepEqual(got, w) {
				t.Errorf("resolve=%v %s: ethernets = %v, want %v", resolve, name, got, w)
			}
		}
	}
}

// DEVICE is available in global/group/device model templates, CLI templates,
// ${...} values in configuration and interface group configurations, and lists
// the device groups the device belongs to (either direction), in device group order.
func TestRenderDeviceConfigs_DeviceVariableEverywhere(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: global_tmpl
      type: model
      configuration:
        system:
          location: '${DEVICE.name}@${join(",", DEVICE.device_groups)}'
    - name: group_tmpl
      type: model
      configuration:
        system:
          contact: '${contains(DEVICE.device_groups, "LEAFS") ? "leaf" : "other"}-${DEVICE.name}'
    - name: device_tmpl
      type: model
      configuration:
        system:
          banner: "device ${DEVICE.name}"
    - name: group_cli
      type: cli
      content: "! ${DEVICE.name} in ${length(DEVICE.device_groups)} groups"
  global:
    templates: [global_tmpl]
    configuration:
      system:
        hostname: "${upper(DEVICE.name)}"
  interface_groups:
    - name: uplink
      configuration:
        description: "${DEVICE.name} uplink"
  device_groups:
    - name: LEAFS
      templates: [group_tmpl, group_cli]
    - name: BORDER
      devices: [leaf1]
  devices:
    - name: leaf1
      device_groups: [LEAFS]
      templates: [device_tmpl]
      configuration:
        interfaces:
          ethernets:
            - id: "1/1"
              interface_groups: [uplink]
    - name: spine1
`)
	for _, resolve := range []bool{false, true} {
		result, err := renderDeviceConfigs(model, nil, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		system := getMapVal(getMapVal(renderedDevice(t, result, "leaf1"), "configuration"), "system")
		for attr, want := range map[string]string{
			"location": "leaf1@LEAFS,BORDER",
			"contact":  "leaf-leaf1",
			"banner":   "device leaf1",
			"hostname": "LEAF1",
		} {
			if got := getStringVal(system, attr, ""); got != want {
				t.Errorf("resolve=%v leaf1: system.%s = %q, want %q", resolve, attr, got, want)
			}
		}
		if got := renderedEthernets(t, result, "leaf1"); got["1/1"] != "leaf1 uplink" {
			t.Errorf("resolve=%v leaf1: interface group description = %q, want %q", resolve, got["1/1"], "leaf1 uplink")
		}
		if got := renderedCliContents(t, result, "leaf1"); len(got) != 1 || got[0] != "! leaf1 in 2 groups" {
			t.Errorf("resolve=%v leaf1: cli template contents = %q, want [%q]", resolve, got, "! leaf1 in 2 groups")
		}

		spine := getMapVal(getMapVal(renderedDevice(t, result, "spine1"), "configuration"), "system")
		if got := getStringVal(spine, "location", ""); got != "spine1@" {
			t.Errorf("resolve=%v spine1: system.location = %q, want %q", resolve, got, "spine1@")
		}
		if got := getStringVal(spine, "hostname", ""); got != "SPINE1" {
			t.Errorf("resolve=%v spine1: system.hostname = %q, want %q", resolve, got, "SPINE1")
		}
	}
}

// A user variable named DEVICE takes precedence over the built-in one, at device
// and at device group level, and only where it is defined.
func TestRenderDeviceConfigs_VariableNamedDevice(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: own
      type: model
      configuration:
        system:
          location: "${DEVICE}"
    - name: own_cli
      type: cli
      content: "! ${DEVICE}"
    - name: builtin
      type: model
      configuration:
        system:
          location: "${DEVICE.name}"
  device_groups:
    - name: CUSTOM
      templates: [own, own_cli]
      variables:
        DEVICE: from-group
  devices:
    - name: leaf1
      templates: [own]
      variables:
        DEVICE: from-device
    - name: leaf2
      templates: [builtin]
    - name: leaf3
      device_groups: [CUSTOM]
`)
	result, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"leaf1": "from-device", "leaf2": "leaf2", "leaf3": "from-group"} {
		if got := renderedLocation(t, result, name); got != want {
			t.Errorf("%s: location = %q, want %q", name, got, want)
		}
	}
	if got := renderedCliContents(t, result, "leaf3"); len(got) != 1 || got[0] != "! from-group" {
		t.Errorf("leaf3: cli template contents = %q, want [%q]", got, "! from-group")
	}
}

// DEVICE does not affect the single conversion of GLOBAL per render, and is not
// part of the native device variables.
func TestRenderDeviceConfigs_DeviceVariableGlobalConvertedOnce(t *testing.T) {
	rctx := renderAll(t, mustYamlModel(t, `
nxos:
  templates:
    - name: tmpl
      type: model
      configuration:
        system:
          location: "${DEVICE.name}-${length(GLOBAL.devices)}"
  device_groups:
    - name: A
      templates: [tmpl]
  devices:
    - name: leaf1
      device_groups: [A]
    - name: leaf2
      device_groups: [A]
`))
	if rctx.globalCtyConversions != 1 {
		t.Errorf("GLOBAL converted %d times, want 1", rctx.globalCtyConversions)
	}
	for _, d := range rctx.devices {
		if _, ok := buildDeviceVariables(rctx, d.(map[string]any))["DEVICE"]; ok {
			t.Error("buildDeviceVariables must not include DEVICE (it is added as a cty value)")
		}
	}
}

// DEVICES lists all devices of the model in model order, also devices not
// managed in this run, with their device groups (membership in either
// direction, in device group order, then groups listed by the device that are
// not defined, as in GLOBAL) and variables (global, device group and device
// variables). DEVICE is the element of DEVICES of the device rendered.
func TestRenderDeviceConfigs_DevicesVariable(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: tmpl
      type: model
      configuration:
        system:
          location: '${jsonencode(DEVICES)}'
          contact: '${join(",", [for d in DEVICES : d.variables.mgmt_ip if contains(d.device_groups, "VPC") && d.name != DEVICE.name])}'
          banner: '${jsonencode(DEVICE) == jsonencode([for d in DEVICES : d if d.name == DEVICE.name][0])}'
  global:
    templates: [tmpl]
    variables:
      site: dc1
      role: none
  device_groups:
    - name: LEAFS
      devices: [leaf2]
      variables:
        role: leaf
    - name: VPC
      variables:
        vpc: true
  devices:
    - name: leaf1
      device_groups: [LEAFS, VPC]
      variables:
        leaf_id: 1
        mgmt_ip: 10.0.0.1
    - name: leaf2
      device_groups: [BORDER, VPC]
      variables:
        leaf_id: 2
        mgmt_ip: 10.0.0.2
        role: border
    - name: spine1
      device_groups: [SPINES, SPINES]
      managed: false
`)
	var want any
	if err := json.Unmarshal([]byte(`[
  {"name": "leaf1", "device_groups": ["LEAFS", "VPC"], "variables": {"site": "dc1", "role": "leaf", "vpc": true, "leaf_id": 1, "mgmt_ip": "10.0.0.1"}},
  {"name": "leaf2", "device_groups": ["LEAFS", "VPC", "BORDER"], "variables": {"site": "dc1", "role": "border", "vpc": true, "leaf_id": 2, "mgmt_ip": "10.0.0.2"}},
  {"name": "spine1", "device_groups": ["SPINES"], "variables": {"site": "dc1", "role": "none"}}
]`), &want); err != nil {
		t.Fatal(err)
	}
	for _, resolve := range []bool{false, true} {
		// Only leaf1 is managed in this run, DEVICES still lists all devices
		result, err := renderDeviceConfigs(model, nil, []string{"leaf1"}, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		if n := len(getSliceVal(getMapVal(result, "nxos"), "devices")); n != 1 {
			t.Fatalf("resolve=%v: %d devices rendered, want 1", resolve, n)
		}
		system := getMapVal(getMapVal(renderedDevice(t, result, "leaf1"), "configuration"), "system")
		var got any
		if err := json.Unmarshal([]byte(getStringVal(system, "location", "")), &got); err != nil {
			t.Fatalf("resolve=%v: DEVICES is not JSON: %v", resolve, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("resolve=%v: DEVICES = %v, want %v", resolve, got, want)
		}
		if got := getStringVal(system, "contact", ""); got != "10.0.0.2" {
			t.Errorf("resolve=%v: vPC peer mgmt_ip = %q, want %q", resolve, got, "10.0.0.2")
		}
		if got := system["banner"]; got != true {
			t.Errorf("resolve=%v: DEVICE == own element of DEVICES = %v, want true", resolve, got)
		}
	}
}

// A user variable named DEVICES takes precedence over the built-in one, and does
// not change DEVICE.
func TestRenderDeviceConfigs_VariableNamedDevices(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: tmpl
      type: model
      configuration:
        system:
          location: "${DEVICES}/${DEVICE.name}/${DEVICE.variables.DEVICES}"
  devices:
    - name: leaf1
      templates: [tmpl]
      variables:
        DEVICES: custom
`)
	result, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := renderedLocation(t, result, "leaf1"); got != "custom/leaf1/custom" {
		t.Errorf("location = %q, want %q", got, "custom/leaf1/custom")
	}
}

// DEVICES is converted once per render, shared by all devices, and not at all
// without templates.
func TestRenderDeviceConfigs_DevicesVariableConvertedOnce(t *testing.T) {
	rctx := renderAll(t, mustYamlModel(t, `
nxos:
  templates:
    - name: tmpl
      type: model
      configuration:
        system:
          location: "${DEVICE.name}-${length(DEVICES)}"
  device_groups:
    - name: A
      templates: [tmpl]
  devices:
    - name: leaf1
      device_groups: [A]
    - name: leaf2
      device_groups: [A]
    - name: leaf3
`))
	if rctx.devicesCtyConversions != 1 {
		t.Errorf("DEVICES converted %d times, want 1", rctx.devicesCtyConversions)
	}

	rctx = renderAll(t, mustYamlModel(t, `
nxos:
  devices:
    - name: leaf1
      configuration:
        system:
          location: fixed
`))
	if rctx.devicesCtyConversions != 0 {
		t.Errorf("DEVICES converted %d times without any template, want 0", rctx.devicesCtyConversions)
	}
}

// An entry of DEVICES that cannot be converted fails the render with an error
// naming the device of the entry, not only the device being rendered.
func TestRenderDeviceConfigs_DevicesVariableConversionError(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: tmpl
      type: model
      configuration:
        system:
          location: "${DEVICE.name}"
  global:
    variables:
      GLOBAL: custom
  devices:
    - name: leaf1
      templates: [tmpl]
    - name: spine1
      managed: false
      variables:
        ratio: .nan
`)
	_, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), `key "DEVICES": device "spine1": key "variables": key "ratio"`) {
		t.Fatalf("error = %v, want it to name the device of the DEVICES entry", err)
	}
}
