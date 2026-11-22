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

	"github.com/zclconf/go-cty/cty"
)

func evalDeviceTemplateExpr(t *testing.T, expr string, vars map[string]cty.Value) (any, error) {
	t.Helper()
	return renderHCLTemplateValueWithFunctions("${"+expr+"}", vars, deviceTemplateFunctions)
}

func TestParseVlans(t *testing.T) {
	vars := map[string]cty.Value{
		"vlans": cty.TupleVal([]cty.Value{cty.NumberIntVal(10), cty.StringVal("20-29"), cty.StringVal("40, 50 - 52")}),
	}
	tests := []struct {
		expr string
		want any
	}{
		{`parse_vlans(vlans)`, map[string]any{
			"ids":    []any{int64(10), int64(40)},
			"ranges": []any{map[string]any{"from": int64(20), "to": int64(29)}, map[string]any{"from": int64(50), "to": int64(52)}},
		}},
		{`parse_vlans(10)`, map[string]any{"ids": []any{int64(10)}}},
		{`parse_vlans("1001-1003")`, map[string]any{"ranges": []any{map[string]any{"from": int64(1001), "to": int64(1003)}}}},
		{`parse_vlans("10,20-29,4094")`, map[string]any{
			"ids":    []any{int64(10), int64(4094)},
			"ranges": []any{map[string]any{"from": int64(20), "to": int64(29)}},
		}},
		{`parse_vlans([null, 10])`, map[string]any{"ids": []any{int64(10)}}},
		{`parse_vlans([])`, map[string]any{}},
		{`parse_vlans(null)`, nil},
		{`jsonencode(parse_vlans(["5", 6]))`, `{"ids":[5,6]}`},
	}
	for _, tt := range tests {
		got, err := evalDeviceTemplateExpr(t, tt.expr, vars)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s = %#v, want %#v", tt.expr, got, tt.want)
		}
	}
}

func TestParseVlans_Errors(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{`parse_vlans("30-20")`, `invalid VLAN range "30-20"`},
		{`parse_vlans(5000)`, `invalid VLAN "5000": VLAN ID 5000 is out of valid range (1-4094)`},
		{`parse_vlans("0-10")`, `invalid VLAN range "0-10"`},
		{`parse_vlans("99999999999999999999")`, `invalid VLAN "99999999999999999999": VLAN IDs must be between 1 and 4094`},
		{`parse_vlans(["SERVICE_PROD"])`, `invalid VLAN "SERVICE_PROD": must be an ID or a range`},
		{`parse_vlans("all")`, `invalid VLAN "all"`},
		{`parse_vlans("+5")`, `invalid VLAN "+5"`},
		{`parse_vlans(10.5)`, `failed: 10.5 is not a whole number`},
		{`parse_vlans([1, 10.5])`, `element 1: 10.5 is not a whole number`},
		{`parse_vlans({ids = [10]})`, `must be a number, a string or a list`},
		{`parse_vlans([[10]])`, `element 0: must be a number or a string`},
	}
	for _, tt := range tests {
		_, err := evalDeviceTemplateExpr(t, tt.expr, nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want %q", tt.expr, err, tt.want)
		}
	}
}

func TestExpandInterfaces(t *testing.T) {
	vars := map[string]cty.Value{
		"ports": cty.TupleVal([]cty.Value{cty.StringVal("1/10"), cty.StringVal("1/20-23")}),
	}
	tests := []struct {
		expr string
		want []any
	}{
		{`expand_interfaces(ports)`, []any{"1/10", "1/20", "1/21", "1/22", "1/23"}},
		{`expand_interfaces("1/120-122")`, []any{"1/120", "1/121", "1/122"}},
		{`expand_interfaces("1/1/1-4")`, []any{"1/1/1", "1/1/2", "1/1/3", "1/1/4"}},
		{`expand_interfaces("1/5, 1/7 - 8")`, []any{"1/5", "1/7", "1/8"}},
		{`expand_interfaces(["10-12", 20])`, []any{"10", "11", "12", "20"}},
		{`expand_interfaces("1/3-3")`, []any{"1/3"}},
		{`expand_interfaces([])`, []any{}},
		{`expand_interfaces(null)`, []any{}},
	}
	for _, tt := range tests {
		got, err := evalDeviceTemplateExpr(t, tt.expr, vars)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s = %#v, want %#v", tt.expr, got, tt.want)
		}
	}
}

func TestExpandInterfaces_Errors(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{`expand_interfaces("1/23-20")`, `invalid interface range "1/23-20": start must not be greater than end`},
		{`expand_interfaces("1/20-1/23")`, `invalid interface range "1/20-1/23": only the last number can be a range`},
		{`expand_interfaces("eth1-4")`, `invalid interface range "eth1-4": only the last number can be a range`},
		{`expand_interfaces("1/1-2000")`, `more than 1024 interfaces`},
		{`expand_interfaces(1.5)`, `failed: 1.5 is not a whole number`},
		{`expand_interfaces(true)`, `must be a number, a string or a list`},
	}
	for _, tt := range tests {
		_, err := evalDeviceTemplateExpr(t, tt.expr, nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want %q", tt.expr, err, tt.want)
		}
	}
}

const networkFunctionsEndpointTemplate = `interfaces:
  ethernets:
%{ for port in expand_interfaces(ports) ~}
    - id: "${port}"
      description: ${name}
      switchport:
        mode: trunk
        trunk_allowed_vlans: ${jsonencode(parse_vlans(vlans))}
%{ endfor ~}
`

// parse_vlans and expand_interfaces in an endpoint file template rendered by
// render_device_configs: one ethernet per port, VLANs in data model format.
func TestRenderDeviceConfigs_NetworkTemplateFunctions(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: endpoint
      type: file
      file: endpoint.yaml
  devices:
    - name: leaf1
  template_instances:
    - name: EP
      template: endpoint
      devices: [leaf1]
      items:
        - variables: {name: ESX01, ports: ["1/10", "1/20-21"], vlans: "10,1001-1003"}
`)
	fileTemplates := map[string]string{"endpoint.yaml": networkFunctionsEndpointTemplate}
	result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := getMapVal(renderedDevice(t, result, "leaf1"), "configuration")
	var ids []string
	for _, e := range getSliceVal(getMapVal(cfg, "interfaces"), "ethernets") {
		em := toMapStringAny(e)
		ids = append(ids, getStringVal(em, "id", ""))
		got := fmt.Sprint(getMapVal(getMapVal(em, "switchport"), "trunk_allowed_vlans"))
		if want := "map[ids:[10] ranges:[map[from:1001 to:1003]]]"; got != want {
			t.Errorf("%s trunk_allowed_vlans = %s, want %s", getStringVal(em, "id", ""), got, want)
		}
	}
	if want := []string{"1/10", "1/20", "1/21"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ethernet ids = %v, want %v", ids, want)
	}
}

// An invalid VLAN range fails the render with the template instance item and
// the range in the error.
func TestRenderDeviceConfigs_NetworkTemplateFunctionsError(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: endpoint
      type: file
      file: endpoint.yaml
  devices:
    - name: leaf1
  template_instances:
    - name: EP
      template: endpoint
      devices: [leaf1]
      items:
        - variables: {name: OK, ports: ["1/10"], vlans: 10}
        - variables: {name: BROKEN, ports: ["1/30"], vlans: "30-20"}
`)
	fileTemplates := map[string]string{"endpoint.yaml": networkFunctionsEndpointTemplate}
	_, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), `template instance "EP" items[1]`) || !strings.Contains(err.Error(), `invalid VLAN range "30-20"`) {
		t.Fatalf("error = %v, want instance item and VLAN range in message", err)
	}
}

// parse_vlans and expand_interfaces are only available in render_device_configs:
// Meraki VLAN lists are strings in CLI notation and port ranges are in the model.
func TestRenderMerakiModel_NoNetworkTemplateFunctions(t *testing.T) {
	for _, fn := range []string{"parse_vlans", "expand_interfaces"} {
		model := mustYamlModel(t, fmt.Sprintf(`
meraki:
  templates:
    networks:
      - name: base
        type: model
        configuration:
          notes: "${%s(10)}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [base]
`, fn))
		_, err := renderMerakiModel(model, nil, false)
		if err == nil || !strings.Contains(err.Error(), "Call to unknown function") {
			t.Errorf("%s: error = %v, want unknown function", fn, err)
		}
	}
}
