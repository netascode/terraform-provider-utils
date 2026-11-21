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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// --- Go unit tests for the rendering pipeline ---

func mustYamlModel(t *testing.T, s string) map[string]any {
	t.Helper()
	decoded, err := yamlDecode(s)
	if err != nil {
		t.Fatalf("decoding YAML: %v", err)
	}
	m, ok := orderedMapToPlainMap(decoded).(map[string]any)
	if !ok {
		t.Fatalf("model is not a map")
	}
	return m
}

func firstNetwork(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	meraki := getMapVal(result, "meraki")
	domain := toMapStringAny(getSliceVal(meraki, "domains")[0])
	org := toMapStringAny(getSliceVal(domain, "organizations")[0])
	return toMapStringAny(getSliceVal(org, "networks")[0])
}

func TestRenderMerakiModel_NetworkModelTemplate(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: base
        type: model
        configuration:
          time_zone: Europe/Berlin
          notes: "Site ${site_id}"
          vlan_id: "${vlan}"
          tags: "${tags}"
          product_types: [appliance]
      - name: override
        type: model
        configuration:
          time_zone: UTC
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [base, override]
              variables:
                site_id: 42
                vlan: 10
                tags: [a, b]
              notes: Own notes
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := getMapVal(result, "meraki")["templates"]; ok {
		t.Errorf("templates section should not be included in result")
	}
	net := firstNetwork(t, result)
	if got := net["time_zone"]; got != "UTC" {
		t.Errorf("time_zone: later template should win, got %v", got)
	}
	if got := net["notes"]; got != "Own notes" {
		t.Errorf("notes: network config should win, got %v", got)
	}
	if got := net["vlan_id"]; got != int64(10) {
		t.Errorf("vlan_id: expected typed int64 10, got %#v", got)
	}
	if got := net["tags"]; !reflect.DeepEqual(got, []any{"a", "b"}) {
		t.Errorf("tags: expected list, got %#v", got)
	}
	if got := net["name"]; got != "Net1" {
		t.Errorf("name: got %v", got)
	}
	if !reflect.DeepEqual(getStringSlice(net, "templates"), []string{"base", "override"}) {
		t.Errorf("templates attribute of network should be preserved")
	}
}

func TestRenderMerakiModel_FileTemplate(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: vlans
        type: file
        file: ./templates/vlans.yaml.tftpl
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [vlans]
              variables:
                vlans: [10, 20]
`)
	fileTemplates := map[string]string{
		"templates/vlans.yaml.tftpl": "appliance:\n  vlans:\n%{ for v in vlans ~}\n    - vlan_id: ${v}\n      name: VLAN${v}\n%{ endfor ~}\n",
	}
	result, err := renderMerakiModel(model, fileTemplates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	vlans := getSliceVal(getMapVal(net, "appliance"), "vlans")
	if len(vlans) != 2 {
		t.Fatalf("expected 2 vlans, got %#v", vlans)
	}
	if got := toMapStringAny(vlans[1])["name"]; got != "VLAN20" {
		t.Errorf("expected VLAN20, got %v", got)
	}
}

func TestRenderMerakiModel_DeviceTemplates(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          floor: "1"
        configuration:
          devices:
            - name: from-template
              templates: [dev]
    devices:
      - name: dev
        type: model
        configuration:
          address: "${address}"
          notes: "${GLOBAL.domains[0].name} floor ${floor}"
          tags: ["${role}"]
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
              variables:
                address: Network Street 1
                role: network-role
              devices:
                - name: own
                  templates: [dev]
                  variables:
                    role: device-role
                  tags: [own]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	devices := getSliceVal(net, "devices")
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %#v", devices)
	}
	byName := map[string]map[string]any{}
	for _, d := range devices {
		dm := toMapStringAny(d)
		byName[getStringVal(dm, "name", "")] = dm
	}

	own := byName["own"]
	if got := own["address"]; got != "Network Street 1" {
		t.Errorf("own.address: expected network variable, got %v", got)
	}
	if got := own["notes"]; got != "EMEA floor 1" {
		t.Errorf("own.notes: expected GLOBAL and network template variable, got %v", got)
	}
	if got := own["tags"]; !reflect.DeepEqual(got, []any{"device-role", "own"}) {
		t.Errorf("own.tags: expected device variable to win and lists merged, got %#v", got)
	}

	tmpl := byName["from-template"]
	if tmpl == nil {
		t.Fatalf("device added by network template missing")
	}
	if got := tmpl["tags"]; !reflect.DeepEqual(got, []any{"network-role"}) {
		t.Errorf("from-template.tags: expected network variable, got %#v", got)
	}
}

func TestRenderMerakiModel_ConfigNotTemplated(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              notes: "literal ${foo}"
              devices:
                - name: dev1
                  notes: "literal ${bar}"
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["notes"]; got != "literal ${foo}" {
		t.Errorf("network notes should be untouched, got %v", got)
	}
	dev := toMapStringAny(getSliceVal(net, "devices")[0])
	if got := dev["notes"]; got != "literal ${bar}" {
		t.Errorf("device notes should be untouched, got %v", got)
	}
}

func TestRenderMerakiModel_InputNotModified(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: base
        type: model
        configuration:
          settings:
            local_status_page_enabled: true
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [base]
              settings:
                remote_status_page_enabled: false
            - name: Net2
              templates: [base]
`)
	if _, err := renderMerakiModel(model, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	org := toMapStringAny(getSliceVal(toMapStringAny(getSliceVal(getMapVal(result, "meraki"), "domains")[0]), "organizations")[0])
	net2 := toMapStringAny(getSliceVal(org, "networks")[1])
	if _, ok := getMapVal(net2, "settings")["remote_status_page_enabled"]; ok {
		t.Errorf("settings of Net1 leaked into Net2: %#v", net2["settings"])
	}
	input := toMapStringAny(getSliceVal(toMapStringAny(getSliceVal(toMapStringAny(getSliceVal(getMapVal(model, "meraki"), "domains")[0]), "organizations")[0]), "networks")[0])
	if _, ok := getMapVal(input, "settings")["local_status_page_enabled"]; ok {
		t.Errorf("input model was modified")
	}
}

func TestRenderMerakiModel_Errors(t *testing.T) {
	cases := map[string]struct {
		model string
		want  string
	}{
		"missing template": {
			model: `
meraki:
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [missing]
`,
			want: `domain "EMEA": organization "Org1": network "Net1": template "missing" not found`,
		},
		"missing file": {
			model: `
meraki:
  templates:
    networks:
      - name: t
        type: file
        file: missing.yaml
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [t]
`,
			want: `file template "t" (path "missing.yaml") not found`,
		},
		"missing device template": {
			model: `
meraki:
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              devices:
                - serial: Q2XX-XXXX-XXXX
                  templates: [missing]
`,
			want: `device "Q2XX-XXXX-XXXX": template "missing" not found`,
		},
		"invalid type": {
			model: `
meraki:
  templates:
    devices:
      - name: t
        type: cli
`,
			want: `device templates: template "t" has unsupported type "cli"`,
		},
		"undefined variable": {
			model: `
meraki:
  templates:
    networks:
      - name: t
        type: model
        configuration:
          notes: "${undefined}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [t]
`,
			want: `rendering model template "t"`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := renderMerakiModel(mustYamlModel(t, tc.model), nil, false)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestRenderMerakiModel_ModelTemplateDirectives(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: t
        type: model
        configuration:
          notes: "%{ if flag }on%{ else }off%{ endif }"
          escaped: "$${literal} %%{ literal }"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [t]
              variables:
                flag: true
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["notes"]; got != "on" {
		t.Errorf("expected directive to be rendered, got %v", got)
	}
	if got := net["escaped"]; got != "${literal} %{ literal }" {
		t.Errorf("expected escapes to be unescaped, got %v", got)
	}
}

func TestRenderMerakiModel_DuplicateTemplateFirstWins(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: t
        type: model
        configuration:
          notes: first
      - name: t
        type: file
        file: second.yaml
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [t]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := firstNetwork(t, result)["notes"]; got != "first" {
		t.Errorf("expected first template definition to win, got %v", got)
	}
}

func TestRenderMerakiModel_EmptyModel(t *testing.T) {
	result, err := renderMerakiModel(map[string]any{}, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, map[string]any{"meraki": map[string]any{}}) {
		t.Errorf("unexpected result: %#v", result)
	}
}

// --- Terraform function tests ---

func TestRenderMerakiModelFunction_Basic(t *testing.T) {
	t.Setenv("TF_ACC_MERAKI_PSK", "secret123")
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRenderMerakiModel_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("time_zone", "Europe/Berlin"),
					resource.TestCheckOutput("notes", "Site 7"),
					resource.TestCheckOutput("vlan_id", "100"),
					resource.TestCheckOutput("file_name", "Guest-Net1"),
					resource.TestCheckOutput("device_address", "Main Street 1"),
					resource.TestCheckOutput("device_notes", "AP floor 3"),
					resource.TestCheckOutput("psk_raw", "!env TF_ACC_MERAKI_PSK"),
					resource.TestCheckOutput("psk_resolved", "secret123"),
					resource.TestCheckOutput("auth_raw", "Bearer !env TF_ACC_MERAKI_PSK"),
					resource.TestCheckOutput("auth_resolved", "Bearer secret123"),
					resource.TestCheckOutput("auth_pure_raw", "!env TF_ACC_MERAKI_PSK"),
					resource.TestCheckOutput("auth_pure_resolved", "secret123"),
					resource.TestCheckOutput("default_time_zone", "UTC"),
					resource.TestCheckOutput("default_user", "user-value"),
					resource.TestCheckOutput("has_templates", "false"),
					resource.TestCheckOutput("has_defaults", "false"),
				),
			},
		},
	})
}

func testAccRenderMerakiModel_basic() string {
	return `
	locals {
		yaml = <<-EOT
		defaults:
		  meraki:
		    domains:
		      organizations:
		        networks:
		          user_default: user-value
		meraki:
		  templates:
		    networks:
		      - name: base
		        type: model
		        configuration:
		          time_zone: Europe/Berlin
		          notes: "Site $${site_id}"
		          auth: "Bearer $${token}"
		          auth_pure: "$${token}"
		          appliance:
		            vlans:
		              - vlan_id: "$${vlan}"
		      - name: wireless
		        type: file
		        file: templates/wireless.yaml
		    devices:
		      - name: ap
		        type: model
		        configuration:
		          address: "$${address}"
		          notes: "AP floor $${floor}"
		  domains:
		    - name: EMEA
		      organizations:
		        - name: Org1
		          networks:
		            - name: Net1
		              templates: [base, wireless]
		              variables:
		                site_id: 7
		                vlan: 100
		                address: Main Street 1
		                token: !env TF_ACC_MERAKI_PSK
		              wireless:
		                ssids:
		                  - name: Corp
		                    psk: !env TF_ACC_MERAKI_PSK
		              devices:
		                - name: ap1
		                  templates: [ap]
		                  variables:
		                    floor: 3
		EOT

		file_templates = {
			"templates/wireless.yaml" = "wireless:\n  ssids:\n    - name: Guest-$${GLOBAL.domains[0].organizations[0].networks[0].name}\n"
		}

		defaults = <<-EOT
		defaults:
		  meraki:
		    domains:
		      organizations:
		        networks:
		          time_zone: UTC
		          user_default: module-value
		EOT

		result   = provider::utils::render_meraki_model([local.yaml], {}, local.defaults, local.file_templates)
		raw      = local.result.raw.meraki.domains[0].organizations[0].networks[0]
		resolved = local.result.resolved.meraki.domains[0].organizations[0].networks[0]
		ssids    = { for s in local.resolved.wireless.ssids : s.name => s }
		raw_ssids = { for s in local.raw.wireless.ssids : s.name => s }
	}

	output "time_zone" {
		value = local.resolved.time_zone
	}
	output "notes" {
		value = local.resolved.notes
	}
	output "vlan_id" {
		value = tostring(local.resolved.appliance.vlans[0].vlan_id)
	}
	output "file_name" {
		value = local.ssids["Guest-Net1"].name
	}
	output "device_address" {
		value = local.resolved.devices[0].address
	}
	output "device_notes" {
		value = local.resolved.devices[0].notes
	}
	output "psk_raw" {
		value = local.raw_ssids["Corp"].psk
	}
	output "psk_resolved" {
		value = local.ssids["Corp"].psk
	}
	output "auth_raw" {
		value = local.raw.auth
	}
	output "auth_resolved" {
		value = local.resolved.auth
	}
	output "auth_pure_raw" {
		value = local.raw.auth_pure
	}
	output "auth_pure_resolved" {
		value = local.resolved.auth_pure
	}
	output "default_time_zone" {
		value = local.result.defaults.meraki.domains.organizations.networks.time_zone
	}
	output "default_user" {
		value = local.result.defaults.meraki.domains.organizations.networks.user_default
	}
	output "has_templates" {
		value = tostring(can(local.result.resolved.meraki.templates))
	}
	output "has_defaults" {
		value = tostring(can(local.result.resolved.defaults))
	}
	`
}

func TestRenderMerakiModelFunction_HclModel(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					model = {
						meraki = {
							templates = {
								networks = [{
									name = "t"
									type = "model"
									configuration = {
										notes = "$${greeting} world"
									}
								}]
							}
							domains = [{
								name = "EMEA"
								organizations = [{
									name = "Org1"
									networks = [{
										name      = "Net1"
										templates = ["t"]
										variables = { greeting = "hello" }
									}]
								}]
							}]
						}
					}
					result = provider::utils::render_meraki_model([], local.model, "", {})
				}

				output "notes" {
					value = local.result.resolved.meraki.domains[0].organizations[0].networks[0].notes
				}
				`,
				Check: resource.TestCheckOutput("notes", "hello world"),
			},
		},
	})
}

func TestRenderMerakiModelFunction_MissingTemplate(t *testing.T) {
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
					  domains:
					    - name: EMEA
					      organizations:
					        - name: Org1
					          networks:
					            - name: Net1
					              templates: [missing]
					EOT
					result = provider::utils::render_meraki_model([local.yaml], {}, "", {})
				}

				output "result" {
					value = local.result.resolved
				}
				`,
				ExpectError: regexp.MustCompile(`template\s+"missing"\s+not\s+found`),
			},
		},
	})
}

// GLOBAL (the whole meraki section) is converted once per render, not once per
// network or device with templates.
func TestRenderMerakiModel_GlobalConvertedOnce(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        configuration:
          notes: "${GLOBAL.domains[0].name}"
    devices:
      - name: dev
        type: model
        configuration:
          notes: "${GLOBAL.domains[0].organizations[0].name}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
              devices:
                - name: d1
                  templates: [dev]
                - name: d2
                  templates: [dev]
            - name: Net2
              templates: [net]
              devices:
                - name: d3
                  templates: [dev]
                - name: d4
`)
	for _, resolve := range []bool{false, true} {
		result, rctx, err := renderMerakiModelContext(model, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		if rctx.globalCtyConversions != 1 {
			t.Errorf("resolve=%v: GLOBAL converted %d times for 2 networks and 3 devices with templates, want 1", resolve, rctx.globalCtyConversions)
		}
		networks := getSliceVal(toMapStringAny(getSliceVal(toMapStringAny(getSliceVal(getMapVal(result, "meraki"), "domains")[0]), "organizations")[0]), "networks")
		for _, n := range networks {
			nm := toMapStringAny(n)
			if got := nm["notes"]; got != "EMEA" {
				t.Errorf("resolve=%v network %v: notes = %v, want EMEA", resolve, nm["name"], got)
			}
			for _, d := range getSliceVal(nm, "devices") {
				dm := toMapStringAny(d)
				want := any("Org1")
				if dm["name"] == "d4" {
					want = nil // no templates
				}
				if got := dm["notes"]; got != want {
					t.Errorf("resolve=%v device %v: notes = %v, want %v", resolve, dm["name"], got, want)
				}
			}
		}
	}
}

// Without networks or devices referencing templates, variables (incl. GLOBAL)
// are not converted at all.
func TestRenderMerakiModel_NoTemplatesNoConversion(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    devices:
      - name: unused
        type: model
        configuration:
          notes: "${GLOBAL.domains[0].name}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              variables:
                site: DC1
              devices:
                - name: d1
                  variables:
                    id: 1
`)
	_, rctx, err := renderMerakiModelContext(model, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if rctx.globalCtyConversions != 0 {
		t.Errorf("GLOBAL converted %d times without any template in use, want 0", rctx.globalCtyConversions)
	}
}

// --- Template inheritance and default variables ---

func TestRenderMerakiModel_TemplateInheritance(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: base
        type: model
        configuration:
          time_zone: UTC
          notes: "base ${site}"
          product_types: [appliance]
      - name: branch
        type: model
        templates: [base]
        configuration:
          time_zone: Europe/Rome
          product_types: [switch]
      - name: small-branch
        type: model
        templates: [branch]
        configuration:
          product_types: [wireless]
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [small-branch]
              variables:
                site: rome
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["notes"]; got != "base rome" {
		t.Errorf("notes: expected inherited from grandparent, got %v", got)
	}
	if got := net["time_zone"]; got != "Europe/Rome" {
		t.Errorf("time_zone: child should override parent, got %v", got)
	}
	if got := net["product_types"]; !reflect.DeepEqual(got, []any{"appliance", "switch", "wireless"}) {
		t.Errorf("product_types: expected lists merged in chain order, got %#v", got)
	}
	if got := getStringSlice(net, "templates"); !reflect.DeepEqual(got, []string{"small-branch"}) {
		t.Errorf("templates: expected network's own list to be preserved, got %#v", got)
	}
}

func TestMerakiTemplateSet_Expand(t *testing.T) {
	set, err := indexMerakiTemplates(getSliceVal(getMapVal(getMapVal(mustYamlModel(t, `
meraki:
  templates:
    networks:
      - {name: base, type: model}
      - {name: other, type: model}
      - {name: a, type: model, templates: [base]}
      - {name: b, type: model, templates: [other, base]}
      - {name: c, type: model, templates: [a, b]}
`), "meraki"), "templates"), "networks"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cases := []struct {
		names []string
		want  []string
	}{
		{[]string{"a"}, []string{"base", "a"}},
		// b applies base after other (base wins), also when combined with a
		{[]string{"b"}, []string{"other", "base", "b"}},
		{[]string{"a", "b"}, []string{"other", "base", "a", "b"}},
		{[]string{"b", "a"}, []string{"other", "base", "b", "a"}},
		{[]string{"c"}, []string{"other", "base", "a", "b", "c"}},
		{[]string{"base", "a"}, []string{"base", "a"}},
		{[]string{"base", "base"}, []string{"base"}},
		{[]string{"a", "a", "b"}, []string{"other", "base", "a", "b"}},
	}
	for _, tc := range cases {
		got, err := set.expand(tc.names)
		if err != nil {
			t.Fatalf("expand(%v): unexpected error: %v", tc.names, err)
		}
		if !reflect.DeepEqual(got.templates, tc.want) {
			t.Errorf("expand(%v) = %v, want %v", tc.names, got.templates, tc.want)
		}
	}

	// Memoized by the referenced template names
	first, _ := set.expand([]string{"a", "b"})
	second, _ := set.expand([]string{"a", "b"})
	if first != second {
		t.Errorf("expand: expected memoized result")
	}
}

func TestMerakiTemplateSet_ExpandPreservesParentOrder(t *testing.T) {
	// b lists its parents as [q, p], so that p overrides q. The order must be
	// kept when b is inherited together with a template that also inherits p.
	set, err := indexMerakiTemplates(getSliceVal(getMapVal(getMapVal(mustYamlModel(t, `
meraki:
  templates:
    networks:
      - {name: p, type: model}
      - {name: q, type: model}
      - {name: a, type: model, templates: [p]}
      - {name: b, type: model, templates: [q, p]}
      - {name: c, type: model, templates: [a, b]}
`), "meraki"), "templates"), "networks"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for names, want := range map[string][]string{
		"b": {"q", "p", "b"},
		"c": {"q", "p", "a", "b", "c"},
	} {
		got, err := set.expand([]string{names})
		if err != nil {
			t.Fatalf("expand(%s): unexpected error: %v", names, err)
		}
		if !reflect.DeepEqual(got.templates, want) {
			t.Errorf("expand(%s) = %v, want %v", names, got.templates, want)
		}
	}
}

func TestRenderMerakiModel_TemplateDefaultVariables(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: base
        type: model
        variables:
          syslog_port: 514
          site: base-site
          servers: [1.1.1.1]
        configuration:
          notes: "${site}:${syslog_port}:${servers[0]}:${role}"
      - name: branch
        type: model
        templates: [base]
        variables:
          site: branch-site
          role: branch
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Defaults
              templates: [branch]
            - name: Overridden
              templates: [branch]
              variables:
                site: own-site
                syslog_port: 1514
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	org := toMapStringAny(getSliceVal(toMapStringAny(getSliceVal(getMapVal(result, "meraki"), "domains")[0]), "organizations")[0])
	networks := getSliceVal(org, "networks")
	defaults := toMapStringAny(networks[0])
	if got := defaults["notes"]; got != "branch-site:514:1.1.1.1:branch" {
		t.Errorf("Defaults.notes: expected child defaults to override parent defaults, got %v", got)
	}
	wantVars := map[string]any{"syslog_port": 514, "site": "branch-site", "role": "branch", "servers": []any{"1.1.1.1"}}
	if got := getMapVal(defaults, "variables"); !reflect.DeepEqual(got, wantVars) {
		t.Errorf("Defaults.variables: expected template defaults, got %#v", got)
	}
	overridden := toMapStringAny(networks[1])
	if got := overridden["notes"]; got != "own-site:1514:1.1.1.1:branch" {
		t.Errorf("Overridden.notes: expected network variables to win, got %v", got)
	}
	if got := getMapVal(overridden, "variables")["site"]; got != "own-site" {
		t.Errorf("Overridden.variables.site: expected network variable, got %v", got)
	}

	// The template definitions must not be aliased by the result
	getSliceVal(getMapVal(defaults, "variables"), "servers")[0] = "changed"
	base := toMapStringAny(getSliceVal(getMapVal(getMapVal(model, "meraki"), "templates"), "networks")[0])
	if got := getSliceVal(getMapVal(base, "variables"), "servers")[0]; got != "1.1.1.1" {
		t.Errorf("template variables modified through result: %v", got)
	}
}

func TestRenderMerakiModel_DeviceTemplateInheritanceAndDefaults(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          floor: "1"
          role: network-default
    devices:
      - name: dev-base
        type: model
        variables:
          role: device-default
          rack: r0
          floor: "0"
        configuration:
          notes: "${role}/${rack}/${floor}"
      - name: ap
        type: model
        templates: [dev-base]
        variables:
          rack: r1
        configuration:
          tags: [ap]
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
              devices:
                - name: d1
                  templates: [ap]
                - name: d2
                  templates: [ap]
                  variables:
                    rack: r9
            - name: Net2
              templates: [net]
              variables:
                rack: network-rack
              devices:
                - name: d3
                  templates: [ap]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := getMapVal(net, "variables"); !reflect.DeepEqual(got, map[string]any{"floor": "1", "role": "network-default"}) {
		t.Errorf("network variables: expected network template defaults, got %#v", got)
	}
	devices := getSliceVal(net, "devices")
	d1 := toMapStringAny(devices[0])
	// Device template defaults override network template defaults
	if got := d1["notes"]; got != "device-default/r1/0" {
		t.Errorf("d1.notes: got %v", got)
	}
	if got := d1["tags"]; !reflect.DeepEqual(got, []any{"ap"}) {
		t.Errorf("d1.tags: got %#v", got)
	}
	if got := getMapVal(d1, "variables"); !reflect.DeepEqual(got, map[string]any{"role": "device-default", "rack": "r1", "floor": "0"}) {
		t.Errorf("d1.variables: expected device template defaults, got %#v", got)
	}
	d2 := toMapStringAny(devices[1])
	if got := d2["notes"]; got != "device-default/r9/0" {
		t.Errorf("d2.notes: expected device variable to win, got %v", got)
	}

	// The network's own variables override device template defaults, which are
	// then not part of the rendered device variables, so that the rendered
	// network variables merged with the rendered device variables are the
	// variables used for rendering
	org := toMapStringAny(getSliceVal(toMapStringAny(getSliceVal(getMapVal(result, "meraki"), "domains")[0]), "organizations")[0])
	net2 := toMapStringAny(getSliceVal(org, "networks")[1])
	d3 := toMapStringAny(getSliceVal(net2, "devices")[0])
	if got := d3["notes"]; got != "device-default/network-rack/0" {
		t.Errorf("d3.notes: expected network variable to override device template default, got %v", got)
	}
	if got := getMapVal(d3, "variables"); !reflect.DeepEqual(got, map[string]any{"role": "device-default", "floor": "0"}) {
		t.Errorf("d3.variables: expected only effective device template defaults, got %#v", got)
	}
	effective := mergeShallow(getMapVal(net2, "variables"), getMapVal(d3, "variables"))
	if want := map[string]any{"role": "device-default", "rack": "network-rack", "floor": "0"}; !reflect.DeepEqual(effective, want) {
		t.Errorf("network + device variables: got %#v, want %#v", effective, want)
	}
}

func TestRenderMerakiModel_FileTemplateInheritance(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: base
        type: model
        configuration:
          time_zone: UTC
      - name: vlans
        type: file
        file: vlans.yaml.tftpl
        templates: [base]
        variables:
          vlans: [10]
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [vlans]
`)
	fileTemplates := map[string]string{
		"vlans.yaml.tftpl": "appliance:\n  vlans:\n%{ for v in vlans ~}\n    - vlan_id: ${v}\n%{ endfor ~}\n",
	}
	result, err := renderMerakiModel(model, fileTemplates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["time_zone"]; got != "UTC" {
		t.Errorf("time_zone: expected inherited from model template, got %v", got)
	}
	if got := getSliceVal(getMapVal(net, "appliance"), "vlans"); len(got) != 1 {
		t.Errorf("vlans: expected rendered with default variable, got %#v", got)
	}
}

func TestRenderMerakiModel_TemplateInheritanceErrors(t *testing.T) {
	network := `
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [a]
`
	cases := map[string]struct {
		model string
		want  string
	}{
		"cycle": {
			model: `
meraki:
  templates:
    networks:
      - {name: a, type: model, templates: [b]}
      - {name: b, type: model, templates: [c]}
      - {name: c, type: model, templates: [a]}
` + network,
			want: `network "Net1": template inheritance cycle: a -> b -> c -> a`,
		},
		"self reference": {
			model: `
meraki:
  templates:
    networks:
      - {name: a, type: model, templates: [a]}
` + network,
			want: `template inheritance cycle: a -> a`,
		},
		"missing parent": {
			model: `
meraki:
  templates:
    networks:
      - {name: a, type: model, templates: [missing]}
` + network,
			want: `network "Net1": template "a": parent template "missing" not found`,
		},
		"parent of other kind": {
			model: `
meraki:
  templates:
    devices:
      - {name: dev, type: model}
    networks:
      - {name: a, type: model, templates: [dev]}
` + network,
			want: `template "a": parent template "dev" not found`,
		},
		"template listed after its child": {
			model: `
meraki:
  templates:
    networks:
      - {name: base, type: model}
      - {name: a, type: model, templates: [base]}
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [a, base]
`,
			want: `network "Net1": inconsistent order of templates "base", "a"`,
		},
		"conflicting parent order": {
			model: `
meraki:
  templates:
    networks:
      - {name: p, type: model}
      - {name: q, type: model}
      - {name: x, type: model, templates: [p, q]}
      - {name: y, type: model, templates: [q, p]}
      - {name: a, type: model, templates: [x, y]}
` + network,
			want: `template "a": inconsistent order of templates`,
		},
		"templates not a list": {
			model: `
meraki:
  templates:
    networks:
      - {name: a, type: model, templates: base}
` + network,
			want: `template "a": 'templates' must be a list of template names`,
		},
		"variables not a map": {
			model: `
meraki:
  templates:
    networks:
      - {name: a, type: model, variables: [x]}
` + network,
			want: `template "a": 'variables' must be a map`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := renderMerakiModel(mustYamlModel(t, tc.model), nil, false)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestRenderMerakiModel_TemplateAttributesInConfigurationIgnored(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: other
        type: model
        configuration:
          notes: from other
      - name: net
        type: model
        variables:
          site: from-variables
        configuration:
          notes: "net ${site}"
          templates: [other]
          variables:
            site: "${undefined}"
      - name: file
        type: file
        file: net.yaml.tftpl
    devices:
      - name: dev
        type: model
        configuration:
          notes: "dev ${site}"
          templates: [other]
          variables:
            site: ignored
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net, file]
              devices:
                - name: d1
                  templates: [dev]
`)
	fileTemplates := map[string]string{
		"net.yaml.tftpl": "time_zone: UTC\ntemplates: [other]\nvariables:\n  site: from-file\n",
	}
	result, err := renderMerakiModel(model, fileTemplates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, result)
	if got := net["notes"]; got != "net from-variables" {
		t.Errorf("notes: expected 'other' not to be applied, got %v", got)
	}
	if got := net["time_zone"]; got != "UTC" {
		t.Errorf("time_zone: expected file template to be applied, got %v", got)
	}
	if got := getStringSlice(net, "templates"); !reflect.DeepEqual(got, []string{"net", "file"}) {
		t.Errorf("templates: expected network's own list only, got %#v", got)
	}
	if got := getMapVal(net, "variables"); !reflect.DeepEqual(got, map[string]any{"site": "from-variables"}) {
		t.Errorf("variables: expected template defaults only, got %#v", got)
	}
	d1 := toMapStringAny(getSliceVal(net, "devices")[0])
	if got := d1["notes"]; got != "dev from-variables" {
		t.Errorf("d1.notes: expected network variables, got %v", got)
	}
	if got := getStringSlice(d1, "templates"); !reflect.DeepEqual(got, []string{"dev"}) {
		t.Errorf("d1.templates: expected device's own list only, got %#v", got)
	}
	if _, ok := d1["variables"]; ok {
		t.Errorf("d1.variables: expected none, got %#v", d1["variables"])
	}

	// The input model must not be modified
	tmpl := toMapStringAny(getSliceVal(getMapVal(getMapVal(model, "meraki"), "templates"), "networks")[1])
	if _, ok := getMapVal(tmpl, "configuration")["templates"]; !ok {
		t.Errorf("configuration of the input model was modified")
	}
}

func TestRenderMerakiModel_UnusedBrokenTemplatesIgnored(t *testing.T) {
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - {name: ok, type: model, configuration: {notes: ok}}
      - {name: missing-parent, type: model, templates: [missing]}
      - {name: bad-variables, type: model, variables: [x]}
    devices:
      - {name: x, type: model, templates: [y]}
      - {name: y, type: model, templates: [x]}
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [ok]
`)
	result, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unused templates should not fail the rendering: %v", err)
	}
	if got := firstNetwork(t, result)["notes"]; got != "ok" {
		t.Errorf("notes: got %v", got)
	}
}

func TestRenderMerakiModel_EnvTagsInDefaultVariables(t *testing.T) {
	t.Setenv("MERAKI_TEST_TOKEN", "secret-token")
	model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          token: !env MERAKI_TEST_TOKEN
        configuration:
          notes: "Bearer ${token}"
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [net]
`)
	// raw: tags are kept, also within strings and in the rendered variables
	raw, err := renderMerakiModel(model, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	net := firstNetwork(t, raw)
	if got := net["notes"]; got != "Bearer !env MERAKI_TEST_TOKEN" {
		t.Errorf("raw notes: got %v", got)
	}
	if got := getMapVal(net, "variables")["token"]; got != "!env MERAKI_TEST_TOKEN" {
		t.Errorf("raw variables.token: got %v", got)
	}

	// resolved: tags are resolved within strings; whole-value tags are resolved
	// by resolveYamlTags afterwards
	resolvedRender, err := renderMerakiModel(model, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolvedNative, err := resolveYamlTags(resolvedRender)
	if err != nil {
		t.Fatalf("resolving tags: %v", err)
	}
	resolved := toMapStringAny(resolvedNative)
	net = firstNetwork(t, resolved)
	if got := net["notes"]; got != "Bearer secret-token" {
		t.Errorf("resolved notes: got %v", got)
	}
	if got := getMapVal(net, "variables")["token"]; got != "secret-token" {
		t.Errorf("resolved variables.token: got %v", got)
	}

	// errors that only occur with resolved values do not reveal them
	failing := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: net
        type: model
        variables:
          token: !env MERAKI_TEST_TOKEN
        configuration:
          notes: "${tonumber(token)}"
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
