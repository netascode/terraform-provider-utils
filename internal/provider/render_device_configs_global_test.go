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
	"testing"
)

func renderedDevice(t *testing.T, result map[string]any, name string) map[string]any {
	t.Helper()
	for _, d := range getSliceVal(getMapVal(result, "nxos"), "devices") {
		if dm, ok := d.(map[string]any); ok && getStringVal(dm, "name", "") == name {
			return dm
		}
	}
	t.Fatalf("device %q not found in result", name)
	return nil
}

func renderedLocation(t *testing.T, result map[string]any, name string) string {
	t.Helper()
	d := renderedDevice(t, result, name)
	return getStringVal(getMapVal(getMapVal(d, "configuration"), "system"), "location", "")
}

func renderedCliContents(t *testing.T, result map[string]any, name string) []string {
	t.Helper()
	var contents []string
	for _, c := range getSliceVal(renderedDevice(t, result, name), "cli_templates") {
		contents = append(contents, getStringVal(c.(map[string]any), "content", ""))
	}
	return contents
}

// renderAll renders every device with one render context, like
// renderDeviceConfigs, and returns the context for inspection.
func renderAll(t *testing.T, model map[string]any) *renderContext {
	t.Helper()
	rctx, err := extractRenderContext(model, "nxos", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range rctx.devices {
		if _, err := renderSingleDevice(rctx, d.(map[string]any)); err != nil {
			t.Fatal(err)
		}
	}
	return rctx
}

const globalTemplatesModel = `
nxos:
  templates:
    - name: group_tmpl
      type: model
      configuration:
        system:
          location: "${GLOBAL.global.variables.site}-${length(GLOBAL.devices)}"
    - name: cli_tmpl
      type: cli
      content: "! ${length(GLOBAL.device_groups)} groups"
  global:
    variables:
      site: DC1
  device_groups:
    - name: A
      templates: [group_tmpl, cli_tmpl]
    - name: B
      templates: [group_tmpl]
  devices:
    - name: leaf1
      device_groups: [A]
    - name: leaf2
      device_groups: [A, B]
    - name: leaf3
      device_groups: [B]
`

// GLOBAL is shared by all devices, device groups and CLI templates: every
// consumer must still see the full model.
func TestRenderDeviceConfigs_GlobalSharedAcrossDevicesAndGroups(t *testing.T) {
	model := mustYamlModel(t, globalTemplatesModel)
	for _, resolve := range []bool{false, true} {
		result, err := renderDeviceConfigs(model, nil, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		for _, name := range []string{"leaf1", "leaf2", "leaf3"} {
			if got := renderedLocation(t, result, name); got != "DC1-3" {
				t.Errorf("resolve=%v %s: location = %q, want %q", resolve, name, got, "DC1-3")
			}
		}
		if got := renderedCliContents(t, result, "leaf1"); len(got) != 1 || got[0] != "! 2 groups" {
			t.Errorf("resolve=%v leaf1: cli template contents = %q, want [%q]", resolve, got, "! 2 groups")
		}
	}
}

// GLOBAL (the whole model) is converted once per render, not once per device,
// device group or CLI template collection.
func TestRenderDeviceConfigs_GlobalConvertedOnce(t *testing.T) {
	rctx := renderAll(t, mustYamlModel(t, globalTemplatesModel))
	if rctx.globalCtyConversions != 1 {
		t.Errorf("GLOBAL converted %d times for 3 devices in overlapping groups, want 1", rctx.globalCtyConversions)
	}
	for _, d := range rctx.devices {
		if _, ok := buildDeviceVariables(rctx, d.(map[string]any))["GLOBAL"]; ok {
			t.Error("buildDeviceVariables must not include GLOBAL (it is added as a converted cty value)")
		}
	}
}

// Without templates or template expressions, variables (incl. GLOBAL) are not
// converted at all.
func TestRenderDeviceConfigs_NoTemplatesNoConversion(t *testing.T) {
	rctx := renderAll(t, mustYamlModel(t, `
nxos:
  global:
    variables:
      site: DC1
  device_groups:
    - name: A
      variables:
        role: leaf
  devices:
    - name: leaf1
      device_groups: [A]
      variables:
        id: 1
      configuration:
        system:
          hostname: leaf1
`))
	if rctx.globalCtyConversions != 0 {
		t.Errorf("GLOBAL converted %d times without any template, want 0", rctx.globalCtyConversions)
	}
}

// A user variable named GLOBAL takes precedence over the built-in one, at device
// and at device group level (group file/model templates and group CLI templates),
// and only where it is defined.
func TestRenderDeviceConfigs_VariableNamedGlobal(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: own
      type: model
      configuration:
        system:
          location: "${GLOBAL}"
    - name: own_cli
      type: cli
      content: "! ${GLOBAL}"
    - name: builtin
      type: model
      configuration:
        system:
          location: "${GLOBAL.global.variables.site}"
  global:
    variables:
      site: DC1
  device_groups:
    - name: CUSTOM
      templates: [own, own_cli]
      variables:
        GLOBAL: from-group
  devices:
    - name: leaf1
      templates: [own]
      variables:
        GLOBAL: from-device
    - name: leaf2
      templates: [builtin]
    - name: leaf3
      device_groups: [CUSTOM]
`)
	result, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"leaf1": "from-device", "leaf2": "DC1", "leaf3": "from-group"} {
		if got := renderedLocation(t, result, name); got != want {
			t.Errorf("%s: location = %q, want %q", name, got, want)
		}
	}
	if got := renderedCliContents(t, result, "leaf3"); len(got) != 1 || got[0] != "! from-group" {
		t.Errorf("leaf3: cli template contents = %q, want [%q]", got, "! from-group")
	}
}
