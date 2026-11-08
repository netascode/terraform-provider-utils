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
        configuration:
          variables:
            floor: "1"
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
