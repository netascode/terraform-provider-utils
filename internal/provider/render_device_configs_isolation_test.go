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

// renderDeviceConfigs is run twice per function call (raw and resolved output),
// so it must not modify the model it is given.
func TestRenderDeviceConfigs_DoesNotMutateModel(t *testing.T) {
	// The file template returns ordered maps, into which the device's loopbacks
	// list is merged by reference, followed by interface groups being applied.
	fileTemplates := map[string]string{
		"templates/base.yaml": "interfaces:\n  ethernets:\n    - id: 1/1\n      description: from file\n",
	}
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: base
      type: file
      file: templates/base.yaml
  global:
    templates: [base]
  interface_groups:
    - name: ig
      configuration:
        ospf:
          process: "1"
          area: 0.0.0.0
  devices:
    - name: leaf1
      variables:
        token: !env TF_TEST_UNUSED
      configuration:
        interfaces:
          loopbacks:
            - id: 0
              interface_groups: [ig]
`)
	before := deepCopy(model)

	first, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(before, model) {
		t.Fatalf("model was modified by the render pass:\nbefore: %#v\nafter:  %#v", before, model)
	}

	second, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(before, model) {
		t.Fatalf("model was modified by the second render pass")
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("render passes differ:\nfirst:  %#v\nsecond: %#v", first, second)
	}

	// The rendered loopback must contain the template and interface group config
	device := toMapStringAny(getSliceVal(getMapVal(first, "nxos"), "devices")[0])
	loopback := toMapStringAny(getSliceVal(getMapVal(getMapVal(device, "configuration"), "interfaces"), "loopbacks")[0])
	if _, ok := loopback["ospf"]; !ok {
		t.Errorf("rendered loopback is missing interface group config: %#v", loopback)
	}
}
