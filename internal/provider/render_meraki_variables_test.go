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
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func networksOf(t *testing.T, result map[string]any) []any {
	t.Helper()
	meraki := getMapVal(result, "meraki")
	domain := toMapStringAny(getSliceVal(meraki, "domains")[0])
	org := toMapStringAny(getSliceVal(domain, "organizations")[0])
	return getSliceVal(org, "networks")
}

// Template defaults computed from site variables, chained expressions, a map
// key referring to a sibling key, type preservation, overriding a computed
// default with a literal of another kind, escapes and file templates looping
// over a computed list.
func TestRenderMerakiModel_ComputedVariables(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: inventory
        type: file
        file: inventory.yaml.tftpl
        variables:
          aps: []
          ap_list: '${[for i, ap in aps : merge({name = format("%s-AP-%02d", site_prefix, i + 1)}, ap)]}'
          ap_count: "${length(ap_list)}"
      - name: base
        type: model
        templates: [inventory]
        variables:
          infra_subnet: "10.${site_octet}.255.0/24"
          wireless:
            corp_ssid: "${site_prefix}-Data"
            guest_ssid: "${wireless.corp_ssid}-Guest"
          dns: ["${dns_primary}", 8.8.4.4]
          dns_primary: 8.8.8.8
          literal: "$${not_a_variable} %%{ not_a_directive }"
          servers: "${dns}"
        configuration:
          notes: "${infra_subnet} ${wireless.guest_ssid} ${ap_count}"
          syslog_servers: "${servers}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [base]
              variables:
                site_prefix: NET1
                site_octet: 21
                aps:
                  - serial: Q2AP-0001
                  - serial: Q2AP-0002
                    name: NET1-Lobby
            - name: Net2
              templates: [base]
              variables:
                site_prefix: NET2
                site_octet: 22
                servers: [192.0.2.1]
                ap_list:
                  - name: Custom
                    serial: Q2AP-0003
`)
	fileTemplates := map[string]string{
		"inventory.yaml.tftpl": "devices:\n%{ for ap in ap_list ~}\n  - name: ${ap.name}\n    serial: ${ap.serial}\n%{ endfor ~}\n",
	}
	result, err := renderMerakiModel(model, fileTemplates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	networks := networksOf(t, result)

	net1 := toMapStringAny(networks[0])
	if got := net1["notes"]; got != "10.21.255.0/24 NET1-Data-Guest 2" {
		t.Errorf("Net1.notes: got %v", got)
	}
	if got := net1["syslog_servers"]; !reflect.DeepEqual(got, []any{"8.8.8.8", "8.8.4.4"}) {
		t.Errorf("Net1.syslog_servers: expected computed list, got %#v", got)
	}
	wantDevices := []any{
		map[string]any{"name": "NET1-AP-01", "serial": "Q2AP-0001"},
		map[string]any{"name": "NET1-Lobby", "serial": "Q2AP-0002"},
	}
	if got := getSliceVal(net1, "devices"); !reflect.DeepEqual(got, wantDevices) {
		t.Errorf("Net1.devices: got %#v", got)
	}
	vars := getMapVal(net1, "variables")
	wantVars := map[string]any{
		"ap_count":     int64(2),
		"infra_subnet": "10.21.255.0/24",
		"wireless":     map[string]any{"corp_ssid": "NET1-Data", "guest_ssid": "NET1-Data-Guest"},
		"literal":      "${not_a_variable} %{ not_a_directive }",
		"servers":      []any{"8.8.8.8", "8.8.4.4"},
	}
	for k, want := range wantVars {
		if got := vars[k]; !reflect.DeepEqual(got, want) {
			t.Errorf("Net1.variables.%s: expected evaluated value %#v, got %#v", k, want, got)
		}
	}
	if got := getSliceVal(vars, "ap_list"); !reflect.DeepEqual(got, wantDevices) {
		t.Errorf("Net1.variables.ap_list: got %#v", got)
	}

	// A literal list replaces a computed default (no kind conflict), and
	// expressions referring to it use the literal value
	net2 := toMapStringAny(networks[1])
	if got := net2["notes"]; got != "10.22.255.0/24 NET2-Data-Guest 1" {
		t.Errorf("Net2.notes: got %v", got)
	}
	if got := net2["syslog_servers"]; !reflect.DeepEqual(got, []any{"192.0.2.1"}) {
		t.Errorf("Net2.syslog_servers: expected network list, got %#v", got)
	}
	if got := getSliceVal(net2, "devices"); !reflect.DeepEqual(got, []any{map[string]any{"name": "Custom", "serial": "Q2AP-0003"}}) {
		t.Errorf("Net2.devices: got %#v", got)
	}

	// The input model must not be modified
	tmpl := toMapStringAny(getSliceVal(getMapVal(getMapVal(model, "meraki"), "templates"), "networks")[1])
	if got := getMapVal(getMapVal(tmpl, "variables"), "wireless")["guest_ssid"]; got != "${wireless.corp_ssid}-Guest" {
		t.Errorf("template variables of the input model were modified: %v", got)
	}
}

// Device templates see network-level expressions evaluated with the device's
// variables (late binding), and a device's rendered variables include the
// network variables whose value differs for the device.
func TestRenderMerakiModel_ComputedDeviceVariables(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          device_prefix: "${site_prefix}-${floor}"
          floor: 1
        configuration:
          notes: "${device_prefix}"
    devices:
      - name: ap
        type: model
        variables:
          ap_note: "AP on ${device_prefix}"
        configuration:
          notes: "${ap_note}/${rack}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
              variables:
                site_prefix: NET1
                rack: "${floor * 10}"
              devices:
                - name: d1
                  templates: [ap]
                - name: d2
                  templates: [ap]
                  variables:
                    floor: 2
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["notes"]; got != "NET1-1" {
		t.Errorf("notes: got %v", got)
	}
	devices := getSliceVal(net, "devices")
	d1 := toMapStringAny(devices[0])
	if got := d1["notes"]; got != "AP on NET1-1/10" {
		t.Errorf("d1.notes: got %v", got)
	}
	if got := getMapVal(d1, "variables"); !reflect.DeepEqual(got, map[string]any{"ap_note": "AP on NET1-1"}) {
		t.Errorf("d1.variables: expected the evaluated device default only, got %#v", got)
	}
	d2 := toMapStringAny(devices[1])
	if got := d2["notes"]; got != "AP on NET1-2/20" {
		t.Errorf("d2.notes: expected network expressions evaluated with the device variable, got %v", got)
	}
	wantD2 := map[string]any{"ap_note": "AP on NET1-2", "floor": 2, "device_prefix": "NET1-2", "rack": int64(20)}
	if got := getMapVal(d2, "variables"); !reflect.DeepEqual(got, wantD2) {
		t.Errorf("d2.variables: expected the device variables and the network variables changed for it, got %#v", got)
	}
	// The rendered network variables deep merged with the rendered device
	// variables are the variables used for rendering
	merged, err := mergeMerakiVariables(getMapVal(net, "variables"), getMapVal(d2, "variables"))
	if err != nil {
		t.Fatalf("merging rendered variables: %v", err)
	}
	if got := merged["device_prefix"]; got != "NET1-2" {
		t.Errorf("merged d2 variables: got %v", got)
	}
}

// Networks and devices without templates have their variables evaluated as
// well (eager evaluation), and the evaluated network variables are used by
// the device templates.
func TestRenderMerakiModel_ComputedVariablesWithoutTemplates(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    devices:
      - name: dev
        type: model
        configuration:
          notes: "${name}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              variables:
                site: NET1
                name: "${site}-device"
              devices:
                - name: d1
                  templates: [dev]
                - name: d2
                  variables:
                    label: "${site}-d2"
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := getMapVal(net, "variables")["name"]; got != "NET1-device" {
		t.Errorf("variables.name: expected evaluated value, got %v", got)
	}
	devices := getSliceVal(net, "devices")
	if got := toMapStringAny(devices[0])["notes"]; got != "NET1-device" {
		t.Errorf("d1.notes: got %v", got)
	}
	if got := getMapVal(toMapStringAny(devices[1]), "variables"); !reflect.DeepEqual(got, map[string]any{"label": "NET1-d2"}) {
		t.Errorf("d2.variables: got %#v", got)
	}
}

func TestRenderMerakiModel_ComputedVariableErrors(t *testing.T) {
	tests := []struct {
		name      string
		variables string
		want      []string
		notWant   []string
	}{
		{
			name:      "cycle",
			variables: `{a: "${b}", b: "${c}", c: "${a}"}`,
			want:      []string{`variable cycle: a -> b -> c -> a`},
		},
		{
			name:      "self reference",
			variables: `{tags: '${concat(tags, ["x"])}'}`,
			want:      []string{`variable cycle: tags -> tags`},
		},
		{
			name:      "keys of a map looking each other up dynamically",
			variables: `{wireless: {a: '${lookup(wireless, "b", "")}', b: '${lookup(wireless, "a", "")}'}}`,
			want:      []string{`variable cycle: wireless.a -> wireless.b -> wireless.a`},
		},
		{
			name:      "unknown variable, dependents not reported, independent errors all reported",
			variables: `{a: "${missing}", b: "${a}-x", c: "${tonumber(\"x\")}"}`,
			want:      []string{`variable "a": evaluating template`, `missing`, `variable "c": evaluating template`},
			notWant:   []string{`variable "b"`},
		},
		{
			name:      "parse error",
			variables: `{a: "${"}`,
			want:      []string{`variable "a": parsing template`},
		},
		{
			name:      "list item path",
			variables: `{aps: [{name: ok}, {name: "${missing}"}]}`,
			want:      []string{`variable "aps[1].name"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - {name: net, type: model, configuration: {notes: x}}
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
              variables: `+tt.variables+`
`)
			_, err := renderMerakiModel(model, nil, false)
			if err == nil {
				t.Fatalf("expected error")
			}
			msg := err.Error()
			if !strings.Contains(msg, `network "Net1": variables:`) {
				t.Errorf("expected error to name the network, got %v", msg)
			}
			for _, want := range tt.want {
				if !strings.Contains(msg, want) {
					t.Errorf("expected error to contain %q, got %v", want, msg)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(msg, notWant) {
					t.Errorf("expected error not to contain %q, got %v", notWant, msg)
				}
			}
		})
	}
}

// try() makes a reference to an optional variable safe, and a device error
// names the device.
func TestRenderMerakiModel_ComputedVariablesOptionalAndDeviceErrors(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          port: '${try(syslog_port, 514)}'
        configuration:
          notes: "${port}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := firstNetwork(t, result)["notes"]; got != int64(514) {
		t.Errorf("notes: got %v", got)
	}

	model = mustYamlModel(t, `
meraki:
  templates:
    devices:
      - {name: dev, type: model, variables: {label: "${missing}"}, configuration: {notes: "${label}"}}
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              devices:
                - name: d1
                  templates: [dev]
`)
	_, err = renderMerakiModel(model, nil, false)
	if err == nil || !strings.Contains(err.Error(), `network "Net1": device "d1": variables:`) || !strings.Contains(err.Error(), `variable "label"`) {
		t.Errorf("expected device error naming the device and variable, got %v", err)
	}
}

func TestRenderMerakiModel_ComputedVariablesEnvTags(t *testing.T) {
	t.Setenv("MERAKI_TEST_TOKEN", "secret-token")
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: splunk
        type: model
        variables:
          token: !env MERAKI_TEST_TOKEN
          auth: "Bearer ${token}"
          auth_pure: "${token}"
        configuration:
          notes: "${auth}"
          name_tag: "${auth_pure}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [splunk]
`)
	raw, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, raw)
	if got := net["notes"]; got != "Bearer !env MERAKI_TEST_TOKEN" {
		t.Errorf("raw notes: got %v", got)
	}
	vars := getMapVal(net, "variables")
	if got := vars["auth"]; got != "Bearer !env MERAKI_TEST_TOKEN" {
		t.Errorf("raw variables.auth: expected tag kept as text, got %v", got)
	}
	if got := vars["auth_pure"]; got != "!env MERAKI_TEST_TOKEN" {
		t.Errorf("raw variables.auth_pure: expected tag to be kept, got %v", got)
	}

	resolvedRender, err := renderMerakiModel(model, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolvedNative, err := resolveYamlTags(resolvedRender)
	if err != nil {
		t.Fatalf("resolving tags: %v", err)
	}
	net = firstNetwork(t, toMapStringAny(resolvedNative))
	if got := net["notes"]; got != "Bearer secret-token" {
		t.Errorf("resolved notes: got %v", got)
	}
	if got := getMapVal(net, "variables")["auth"]; got != "Bearer secret-token" {
		t.Errorf("resolved variables.auth: got %v", got)
	}
}

func TestEvaluateMerakiVariables(t *testing.T) {
	// No expressions: returned as is
	vars := map[string]any{"a": 1, "b": []any{"x"}}
	got, computed, err := evaluateMerakiVariables(vars, make(merakiExpressionCache), false)
	if err != nil || computed || !reflect.DeepEqual(got, vars) {
		t.Errorf("no expressions: got %#v, %v, %v", got, computed, err)
	}

	// For-expression symbols are not references: `list` does not depend on
	// the variable `ap`, which would be a cycle
	vars = map[string]any{
		"ap":   "${length(list)}",
		"list": "${[for ap in aps : ap * 2]}",
		"aps":  []any{1, "${one}"},
		"one":  3,
	}
	got, computed, err = evaluateMerakiVariables(vars, make(merakiExpressionCache), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !computed {
		t.Errorf("expected computed")
	}
	if !reflect.DeepEqual(got["list"], []any{int64(2), int64(6)}) || got["ap"] != int64(2) {
		t.Errorf("got %#v", got)
	}
	if vars["ap"] != "${length(list)}" {
		t.Errorf("input modified: %#v", vars)
	}
}

func TestStaticTraversalPath(t *testing.T) {
	tests := []struct {
		expr string
		want [][]any
	}{
		{`${a.b.c}`, [][]any{{"a", "b", "c"}}},
		{`${a["b"][0].c}`, [][]any{{"a", "b", 0, "c"}}},
		{`${a[i].c}`, [][]any{{"a"}, {"i"}}},
		{`${[for x in xs : x.y]}`, [][]any{{"xs"}}},
		{`${a[1.5]}`, [][]any{{"a"}}},
	}
	for _, tt := range tests {
		expr, diags := hclsyntax.ParseTemplate([]byte(tt.expr), "t", hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			t.Fatalf("%s: %s", tt.expr, diags.Error())
		}
		var got [][]any
		for _, tr := range expr.Variables() {
			got = append(got, staticTraversalPath(tr))
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %#v, want %#v", tt.expr, got, tt.want)
		}
	}
}

func TestMerakiPathsOverlap(t *testing.T) {
	tests := []struct {
		a, b []any
		want bool
	}{
		{[]any{"a"}, []any{"a", "b"}, true},
		{[]any{"a", "b"}, []any{"a"}, true},
		{[]any{"a", "b"}, []any{"a", "c"}, false},
		{[]any{"x", "0"}, []any{"x", 0, "name"}, true},
		{[]any{"a"}, []any{"b"}, false},
	}
	for _, tt := range tests {
		if got := merakiPathsOverlap(tt.a, tt.b); got != tt.want {
			t.Errorf("%#v vs %#v: got %v", tt.a, tt.b, got)
		}
	}
}

func TestRenderMerakiModelFunction_ComputedVariables(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					yaml = <<-EOT
					meraki:
					  templates:
					    networks:
					      - name: branch
					        type: model
					        variables:
					          aps: []
					          ap_names: '$${[for i, ap in aps : try(ap.name, format("%s-AP-%02d", site_prefix, i + 1))]}'
					          wireless:
					            guest_ssid: "$${site_prefix}-Guest"
					        configuration:
					          notes: '$${join(",", ap_names)}/$${wireless.guest_ssid}'
					  domains:
					    - name: EMEA
					      organizations:
					        - name: Org1
					          networks:
					            - name: Net1
					              templates: [branch]
					              variables:
					                site_prefix: NET1
					                aps:
					                  - serial: Q2AP-0001
					                  - serial: Q2AP-0002
					                    name: Lobby
					EOT
					result = provider::utils::render_meraki_model([local.yaml], {}, "", {})
					network = local.result.resolved.meraki.domains[0].organizations[0].networks[0]
				}

				output "notes" {
					value = local.network.notes
				}
				output "ap_names" {
					value = join(",", local.network.variables.ap_names)
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("notes", "NET1-AP-01,Lobby/NET1-Guest"),
					resource.TestCheckOutput("ap_names", "NET1-AP-01,Lobby"),
				),
			},
		},
	})
}

func TestRenderMerakiModelFunction_ComputedVariableCycle(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					yaml = <<-EOT
					meraki:
					  templates:
					    networks:
					      - {name: net, type: model, variables: {a: "$${b}", b: "$${a}"}, configuration: {notes: "$${a}"}}
					  domains:
					    - name: EMEA
					      organizations:
					        - name: Org1
					          networks:
					            - {name: Net1, templates: [net]}
					EOT
				}

				output "result" {
					value = provider::utils::render_meraki_model([local.yaml], {}, "", {})
				}
				`,
				ExpectError: regexp.MustCompile(`variable cycle: a -> b -> a`),
			},
		},
	})
}

// BenchmarkRenderMerakiModel_ComputedVariables renders 100 networks with 20
// devices each, with literal and with computed template defaults (evaluated
// once per network and once per device).
func BenchmarkRenderMerakiModel_ComputedVariables(b *testing.B) {
	build := func(defaults string) map[string]any {
		var sb strings.Builder
		sb.WriteString(`
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
` + defaults + `
        configuration:
          notes: '${join(",", ap_names)} ${subnet}'
    devices:
      - name: dev
        type: model
        configuration:
          notes: "${subnet}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
`)
		for n := 0; n < 100; n++ {
			sb.WriteString("            - name: Net" + strings.Repeat("x", n%5) + "\n              templates: [net]\n              variables:\n                site_prefix: NET\n                site_octet: 21\n                aps: [")
			for a := 0; a < 10; a++ {
				sb.WriteString("{serial: Q2AP}, ")
			}
			sb.WriteString("]\n              devices:\n")
			for d := 0; d < 20; d++ {
				sb.WriteString("                - {name: d, templates: [dev]}\n")
			}
		}
		decoded, err := yamlDecode(sb.String())
		if err != nil {
			b.Fatal(err)
		}
		return orderedMapToPlainMap(decoded).(map[string]any)
	}
	cases := map[string]string{
		"literal":  "          ap_names: [NET-AP-01, NET-AP-02]\n          subnet: 10.21.0.0/24",
		"computed": "          ap_names: '${[for i, ap in aps : format(\"%s-AP-%02d\", site_prefix, i + 1)]}'\n          subnet: \"10.${site_octet}.0.0/24\"",
	}
	for name, defaults := range cases {
		model := build(defaults)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := renderMerakiModel(model, nil, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Devices without own variables share their evaluated variables (see
// merakiScope); their rendered variables must not be aliased.
func TestRenderMerakiModel_SharedDeviceScopeNotAliased(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    devices:
      - name: dev
        type: model
        variables:
          ports: ["${first_port}", 2]
          first_port: 1
        configuration:
          notes: "${ports[0]}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              devices:
                - {name: d1, templates: [dev]}
                - {name: d2, templates: [dev]}
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	devices := getSliceVal(firstNetwork(t, result), "devices")
	d1, d2 := toMapStringAny(devices[0]), toMapStringAny(devices[1])
	if got := d1["notes"]; got != int64(1) {
		t.Errorf("d1.notes: got %#v", got)
	}
	getSliceVal(getMapVal(d1, "variables"), "ports")[0] = "changed"
	if got := getSliceVal(getMapVal(d2, "variables"), "ports")[0]; got != int64(1) {
		t.Errorf("d2.variables.ports aliased with d1: got %#v", got)
	}
}

// Computed values are merged like literal values once evaluated: maps
// overriding keys of a computed map (and computed maps overriding maps) are
// deep merged, also over several levels and with expressions in the map
// overrides; literal lists and scalars replace computed values.
func TestRenderMerakiModel_ComputedVariablesDeepMerge(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: base
        type: model
        variables:
          org_wireless: {corp_ssid: Data, guest_ssid: Guest, bw: 0}
          wireless: '${merge(org_wireless, {corp_ssid = "${site_prefix}-Data"})}'
          switching:
            mtu: 9198
            vlans: {data: 10}
          servers: '${["192.0.2.1", "192.0.2.2"]}'
        configuration:
          notes: '${wireless.corp_ssid}/${wireless.guest_ssid}/${wireless.bw}/${switching.mtu}/${switching.vlans.data}/${switching.vlans.voice}/${join(",", servers)}'
      - name: profile
        type: model
        templates: [base]
        variables:
          wireless:
            bw: 5000
          switching: '${ {vlans = {voice = 20}} }'
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [profile]
              variables:
                site_prefix: NET1
                wireless:
                  guest_ssid: "${wireless.corp_ssid}-Guest"
                servers: [198.51.100.1]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["notes"]; got != "NET1-Data/NET1-Data-Guest/5000/9198/10/20/198.51.100.1" {
		t.Errorf("notes: got %v", got)
	}
	wantWireless := map[string]any{"corp_ssid": "NET1-Data", "guest_ssid": "NET1-Data-Guest", "bw": 5000}
	if got := getMapVal(getMapVal(net, "variables"), "wireless"); !reflect.DeepEqual(got, wantWireless) {
		t.Errorf("variables.wireless: got %#v", got)
	}

	// A computed map and a literal list at different levels are a conflict
	model = mustYamlModel(t, `
meraki:
  templates:
    networks:
      - {name: base, type: model, variables: {wireless: '${ {a = 1} }'}, configuration: {notes: x}}
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [base]
              variables:
                wireless: {b: 2}
            - name: Net2
              templates: [base]
              variables:
                wireless: '${["a"]}'
`)
	_, err = renderMerakiModel(model, nil, false)
	if err == nil || !strings.Contains(err.Error(), `network "Net2"`) || !strings.Contains(err.Error(), `variable "wireless": conflicting types of evaluated values (map vs list)`) {
		t.Errorf("expected a conflict of the evaluated values, got %v", err)
	}
}

// A key of a map can look up other keys of the map dynamically; the key
// itself is null while it is evaluated. Two keys looking each other up
// dynamically are a cycle (see TestRenderMerakiModel_ComputedVariableErrors).
func TestRenderMerakiModel_ComputedVariablesDynamicSelfLookup(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          wireless:
            corp_ssid: Data
            guest_ssid: '${lookup(wireless, "corp_ssid", "")}-Guest-${lookup(wireless, "guest_ssid", "") == null ? "unset" : "set"}'
        configuration:
          notes: "${wireless.guest_ssid}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := firstNetwork(t, result)["notes"]; got != "Data-Guest-unset" {
		t.Errorf("notes: got %v", got)
	}
}

// Values of !env tags are never evaluated, also if they contain `${` or `%{`,
// and errors that only occur with resolved values do not reveal them.
func TestRenderMerakiModel_ComputedVariablesEnvTagValues(t *testing.T) {
	t.Setenv("MERAKI_TEST_SECRET", "p%{x}a${b}")
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          secret: !env MERAKI_TEST_SECRET
          auth: "Bearer ${secret}"
          nested: {secret: !env MERAKI_TEST_SECRET}
          deferred: '${ {a = 1} }'
        configuration:
          notes: "${auth}"
          secret: "${secret}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
              variables:
                deferred: {secret: !env MERAKI_TEST_SECRET}
`)
	if _, err := renderMerakiModel(model, nil, false); err != nil {
		t.Fatalf("raw: unexpected error: %v", err)
	}
	resolvedRender, err := renderMerakiModel(model, nil, true)
	if err != nil {
		t.Fatalf("resolved: unexpected error: %v", err)
	}
	resolvedNative, err := resolveYamlTags(resolvedRender)
	if err != nil {
		t.Fatalf("resolving tags: %v", err)
	}
	net := firstNetwork(t, toMapStringAny(resolvedNative))
	if got := net["notes"]; got != "Bearer p%{x}a${b}" {
		t.Errorf("notes: got %v", got)
	}
	if got := net["secret"]; got != "p%{x}a${b}" {
		t.Errorf("secret: got %v", got)
	}
	vars := getMapVal(net, "variables")
	if got := getMapVal(vars, "nested")["secret"]; got != "p%{x}a${b}" {
		t.Errorf("variables.nested.secret: got %v", got)
	}
	if got := getMapVal(vars, "deferred"); !reflect.DeepEqual(got, map[string]any{"a": int64(1), "secret": "p%{x}a${b}"}) {
		t.Errorf("variables.deferred: got %#v", got)
	}

	t.Setenv("MERAKI_TEST_TOKEN", "secret-token")
	failing := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          token: !env MERAKI_TEST_TOKEN
          port: "${tonumber(token)}"
        configuration:
          notes: "${port}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
`)
	_, err = renderMerakiModel(failing, nil, true)
	if err == nil {
		t.Fatalf("expected error")
	}
	if msg := resolvedRenderError(err, failing).Error(); strings.Contains(msg, "secret-token") {
		t.Errorf("error reveals the value of the !env tag: %s", msg)
	}
}

// The rendered device variables are the same in the raw and resolved output
// for a network without templates and expressions with !env tags.
func TestRenderMerakiModel_ComputedDeviceVariablesEnvTagsConsistent(t *testing.T) {
	t.Setenv("MERAKI_TEST_TOKEN", "secret-token")
	model := mustYamlModel(t, `
meraki:
  templates:
    devices:
      - name: dev
        type: model
        variables:
          label: "${site}-x"
        configuration:
          notes: "${label}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              variables:
                site: NET1
                token: !env MERAKI_TEST_TOKEN
              devices:
                - name: d1
                  templates: [dev]
`)
	raw, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("raw: unexpected error: %v", err)
	}
	resolved, err := renderMerakiModel(model, nil, true)
	if err != nil {
		t.Fatalf("resolved: unexpected error: %v", err)
	}
	want := map[string]any{"label": "NET1-x"}
	for name, result := range map[string]map[string]any{"raw": raw, "resolved": resolved} {
		d1 := toMapStringAny(getSliceVal(firstNetwork(t, result), "devices")[0])
		if got := getMapVal(d1, "variables"); !reflect.DeepEqual(got, want) {
			t.Errorf("%s d1.variables: got %#v", name, got)
		}
	}
}
