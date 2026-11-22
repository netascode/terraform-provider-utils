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

// renderedVlans returns id → "name/vn_segment" of the rendered VLANs of a device.
func renderedVlans(t *testing.T, result map[string]any, name string) map[string]string {
	t.Helper()
	cfg := getMapVal(renderedDevice(t, result, name), "configuration")
	got := map[string]string{}
	for _, v := range getSliceVal(getMapVal(cfg, "vlan"), "vlans") {
		vm := toMapStringAny(v)
		got[fmtString(vm["id"])] = fmtString(vm["name"]) + "/" + fmtString(vm["vn_segment"])
	}
	return got
}

func fmtString(v any) string {
	return fmt.Sprint(v)
}

const serviceL2Template = `vlan:
  vlans:
    - id: ${vlan_id}
      name: ${vlan_name}
      vn_segment: ${try(vni, l2vni_base + vlan_id)}
`

// Instances apply a template to device groups (either membership direction) and
// devices with their own variables. An optional variable of one instance does not
// leak into another instance on the same device (unlike device group variables).
func TestRenderDeviceConfigs_TemplateInstancesVariablesAreLocal(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: service_l2
      type: file
      file: templates/service_l2.yaml
  global:
    variables:
      l2vni_base: 30000
  device_groups:
    - name: LEAFS
      devices: [leaf2]
  devices:
    - name: leaf1
      device_groups: [LEAFS]
    - name: leaf2
    - name: leaf3
    - name: spine1
  template_instances:
    - name: WEB
      template: service_l2
      device_groups: [LEAFS]
      variables:
        vlan_id: 1001
        vlan_name: WEB
        vni: 40001
    - name: APP
      template: service_l2
      device_groups: [LEAFS]
      devices: [leaf3]
      variables:
        vlan_id: 1002
        vlan_name: APP
`)
	fileTemplates := map[string]string{"templates/service_l2.yaml": serviceL2Template}
	both := map[string]string{"1001": "WEB/40001", "1002": "APP/31002"}
	want := map[string]map[string]string{
		"leaf1":  both,
		"leaf2":  both,
		"leaf3":  {"1002": "APP/31002"},
		"spine1": {},
	}
	for _, resolve := range []bool{false, true} {
		result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		for name, w := range want {
			if got := renderedVlans(t, result, name); !reflect.DeepEqual(got, w) {
				t.Errorf("resolve=%v %s: vlans = %v, want %v", resolve, name, got, w)
			}
		}
	}
}

// Items render the template once each, with item variables on top of instance
// variables on top of device variables. Item devices and device groups narrow
// the devices of the instance.
func TestRenderDeviceConfigs_TemplateInstanceItems(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: endpoint
      type: file
      file: templates/endpoint.yaml
  device_groups:
    - name: BORDER
      devices: [leaf2, leaf3]
  devices:
    - name: leaf1
      variables:
        mode: device
    - name: leaf2
      variables:
        mode: device
    - name: leaf3
  template_instances:
    - name: RACK1
      template: endpoint
      devices: [leaf1, leaf2]
      variables:
        mode: access
      items:
        - variables: {name: esx01, port: "1/10"}
        - devices: [leaf2]
          variables: {name: backup01, port: "1/12", mode: trunk}
        - device_groups: [BORDER]
          variables: {name: fw01, port: "1/13"}
`)
	fileTemplates := map[string]string{
		"templates/endpoint.yaml": `interfaces:
  ethernets:
    - id: "${port}"
      description: "${name} ${mode}"
`,
	}
	want := map[string]map[string]string{
		"leaf1": {"1/10": "esx01 access"},
		"leaf2": {"1/10": "esx01 access", "1/12": "backup01 trunk", "1/13": "fw01 access"},
		"leaf3": {},
	}
	result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, w := range want {
		if got := renderedEthernets(t, result, name); !reflect.DeepEqual(got, w) {
			t.Errorf("%s: ethernets = %v, want %v", name, got, w)
		}
	}
}

// Template instances are merged after device group configurations and before
// device templates and configuration. Model and CLI templates work as well, and
// an explicit empty items list renders nothing.
func TestRenderDeviceConfigs_TemplateInstancePrecedenceModelAndCli(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: hostname
      type: model
      configuration:
        system:
          hostname: ${prefix}-${DEVICE.name}
          description: instance
    - name: banner
      type: cli
      stage: pre
      order: 3
      content: banner motd ${text}
    - name: device_desc
      type: model
      configuration:
        system:
          description: device template
  device_groups:
    - name: ALL
      devices: [leaf1, leaf2]
      configuration:
        system:
          hostname: group
          description: group
    - name: MOTD
      devices: [leaf1]
      templates: [banner]
      variables: {text: group}
  devices:
    - name: leaf1
    - name: leaf2
      templates: [device_desc]
  template_instances:
    - name: NAMES
      template: hostname
      device_groups: [ALL]
      variables:
        prefix: dc1
    - name: MOTD
      template: banner
      devices: [leaf1]
      items:
        - variables: {text: one}
        - variables: {text: two}
    - name: NOTHING
      template: banner
      devices: [leaf1, leaf2]
      items: []
`)
	result, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][2]string{
		"leaf1": {"dc1-leaf1", "instance"},
		"leaf2": {"dc1-leaf2", "device template"},
	} {
		system := getMapVal(getMapVal(renderedDevice(t, result, name), "configuration"), "system")
		if got := [2]string{getStringVal(system, "hostname", ""), getStringVal(system, "description", "")}; got != want {
			t.Errorf("%s: hostname/description = %v, want %v", name, got, want)
		}
	}

	var got []string
	for _, c := range getSliceVal(renderedDevice(t, result, "leaf1"), "cli_templates") {
		m := toMapStringAny(c)
		got = append(got, getStringVal(m, "name", "")+"|"+getStringVal(m, "content", "")+"|"+getStringVal(m, "stage", "")+"|"+fmtString(m["order"]))
	}
	// A device group with the same name as an instance keeps its own name
	want := []string{
		"banner/MOTD|banner motd group|pre|3",
		"banner/instance/MOTD/items[0]|banner motd one|pre|3",
		"banner/instance/MOTD/items[1]|banner motd two|pre|3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("leaf1 cli_templates = %v, want %v", got, want)
	}
	if cli := getSliceVal(renderedDevice(t, result, "leaf2"), "cli_templates"); len(cli) != 0 {
		t.Errorf("leaf2 cli_templates = %v, want none", cli)
	}
}

// All problems with template instances are reported at once, with their path.
func TestRenderDeviceConfigs_TemplateInstanceValidation(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: service_l2
      type: file
      file: templates/service_l2.yaml
    - name: odd
      type: other
  device_groups:
    - name: LEAFS
      devices: [leaf1]
  devices:
    - name: leaf1
      device_groups: [SPINES]
    - name: leaf2
  template_instances:
    - name: A
      template: missing
      devices: [leaf9]
      device_groups: [NOPE]
    - name: B
      template: service_l2
    - name: C
      template: odd
      device_groups: [SPINES]
    - name: D
      template: service_l2
      device_groups: [LEAFS]
      items:
        - devices: [leaf2]
        - device_groups: [GONE]
    - template: service_l2
      devices: [leaf1]
    - name: A
      template: service_l2
      devices: [leaf1]
    - name: E
      devices: [leaf1]
      item:
        - variables: {}
      variables: [1]
    - name: F
      template: service_l2
      devices: leaf1
      device_groups: [1]
      items: {a: 1}
    - name: G
      template: service_l2
      devices: [leaf1, leaf2]
      items:
        - name: x
          device: leaf1
        - name: x
        - name: a/b
        - 1
        - variables: oops
    - name: H/I
      template: service_l2
      devices: [leaf1]
    - just a string
`)
	_, err := renderDeviceConfigs(model, map[string]string{"templates/service_l2.yaml": serviceL2Template}, nil, nil, nil, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{
		`template_instances[A]: unknown template "missing"`,
		`template_instances[A]: unknown device "leaf9"`,
		`template_instances[A]: unknown device group "NOPE"`,
		`template_instances[B]: devices or device_groups is required`,
		`template_instances[C]: template "odd" has unsupported type "other"`,
		`template_instances[D].items[0]: device "leaf2" is not a target of the template instance`,
		`template_instances[D].items[1]: unknown device group "GONE"`,
		`template_instances[4]: name is required`,
		`template_instances[A]: duplicate name`,
		`template_instances[E]: template is required`,
		`template_instances[E]: unknown attribute "item"`,
		`template_instances[E]: variables must be a map`,
		`template_instances[F]: devices must be a list of strings`,
		`template_instances[F]: device_groups must be a list of strings`,
		`template_instances[F]: devices or device_groups is required`,
		`template_instances[F]: items must be a list`,
		`template_instances[G].items[name=x]: unknown attribute "device"`,
		`template_instances[G].items[name=x]: duplicate name`,
		`template_instances[G].items[name=a/b]: name must not contain "/"`,
		`template_instances[G].items[3]: must be a map`,
		`template_instances[G].items[4]: variables must be a map`,
		`template_instances[H/I]: name must not contain "/"`,
		`template_instances[10]: must be a map`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	// SPINES is only referenced by a device, which is a valid target
	if strings.Contains(err.Error(), `"SPINES"`) {
		t.Errorf("device group referenced only by devices reported as unknown:\n%v", err)
	}
}

// Render errors name the instance and item.
func TestRenderDeviceConfigs_TemplateInstanceRenderError(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: service_l2
      type: file
      file: templates/service_l2.yaml
  devices:
    - name: leaf1
  template_instances:
    - name: WEB
      template: service_l2
      devices: [leaf1]
      items:
        - variables: {vlan_id: 1, vlan_name: a, vni: 1}
        - variables: {vlan_name: b, vni: 2}
`)
	_, err := renderDeviceConfigs(model, map[string]string{"templates/service_l2.yaml": serviceL2Template}, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), `template instance "WEB" items[1]`) {
		t.Fatalf("error = %v, want it to name the instance and item", err)
	}
}

// benchmarkInstancesModel returns a model with leaves devices, an endpoint
// instance per device with items items, and globalVars global variables.
func benchmarkInstancesModel(b *testing.B, leaves, items, globalVars int) map[string]any {
	b.Helper()
	var sb strings.Builder
	sb.WriteString("nxos:\n  templates:\n    - name: endpoint\n      type: file\n      file: templates/endpoint.yaml\n")
	sb.WriteString("  global:\n    variables:\n")
	for i := 0; i < globalVars; i++ {
		fmt.Fprintf(&sb, "      var%d: {name: v%d, list: [1, 2, 3]}\n", i, i)
	}
	sb.WriteString("  devices:\n")
	for l := 1; l <= leaves; l++ {
		fmt.Fprintf(&sb, "    - name: leaf%d\n", l)
	}
	sb.WriteString("  template_instances:\n")
	for l := 1; l <= leaves; l++ {
		fmt.Fprintf(&sb, "    - name: ENDPOINTS_LEAF%d\n      template: endpoint\n      devices: [leaf%d]\n      items:\n", l, l)
		for i := 1; i <= items; i++ {
			fmt.Fprintf(&sb, "        - variables: {name: host%d, port: \"1/%d\"}\n", i, i)
		}
	}
	decoded, err := yamlDecode(sb.String())
	if err != nil {
		b.Fatal(err)
	}
	return orderedMapToPlainMap(decoded).(map[string]any)
}

// Many items per device with many global variables (each item renders the template once).
func BenchmarkRenderDeviceConfigs_TemplateInstanceItems(b *testing.B) {
	model := benchmarkInstancesModel(b, 4, 200, 200)
	fileTemplates := map[string]string{
		"templates/endpoint.yaml": "interfaces:\n  ethernets:\n    - id: \"${port}\"\n      description: \"${name}\"\n",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, false); err != nil {
			b.Fatal(err)
		}
	}
}

// An instance whose items are null renders once, like an instance without
// items. Named items name their CLI templates and error messages.
func TestRenderDeviceConfigs_TemplateInstanceNullAndNamedItems(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: banner
      type: cli
      content: banner motd ${text}
  devices:
    - name: leaf1
  template_instances:
    - name: ONCE
      template: banner
      devices: [leaf1]
      variables: {text: once}
      items:
    - name: NAMED
      template: banner
      devices: [leaf1]
      items:
        - name: first
          variables: {text: one}
        - variables: {text: two}
        - name: 1001
          variables: {text: three}
`)
	result, err := renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range getSliceVal(renderedDevice(t, result, "leaf1"), "cli_templates") {
		m := toMapStringAny(c)
		got = append(got, getStringVal(m, "name", "")+"|"+getStringVal(m, "content", ""))
	}
	want := []string{
		"banner/instance/ONCE|banner motd once",
		"banner/instance/NAMED/first|banner motd one",
		"banner/instance/NAMED/items[1]|banner motd two",
		"banner/instance/NAMED/1001|banner motd three",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cli_templates = %v, want %v", got, want)
	}

	model = mustYamlModel(t, `
nxos:
  templates:
    - name: banner
      type: cli
      content: banner motd ${text}
  devices:
    - name: leaf1
  template_instances:
    - name: NAMED
      template: banner
      devices: [leaf1]
      items:
        - name: broken
`)
	_, err = renderDeviceConfigs(model, nil, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), `template instance "NAMED" items[name=broken]`) {
		t.Fatalf("error = %v, want it to name the item", err)
	}
}

// Instance templates see DEVICES and DEVICE (with the variables of all devices)
// and their interfaces get the configuration of interface groups. Instance
// variables take precedence over device variables, also for !env tags, which
// are only resolved in the resolved output.
func TestRenderDeviceConfigs_TemplateInstanceVariablesAndInterfaceGroups(t *testing.T) {
	t.Setenv("TF_ACC_RDC_INSTANCE_TOKEN", "secret")
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: endpoint
      type: file
      file: templates/endpoint.yaml
  interface_groups:
    - name: HOST
      configuration:
        mtu: 9216
        description: interface group
  devices:
    - name: leaf1
      variables:
        peer: leaf2
        token: device
    - name: leaf2
      variables:
        mgmt_ip: 10.0.0.2
  template_instances:
    - name: SERVERS
      template: endpoint
      devices: [leaf1]
      variables:
        token: !env TF_ACC_RDC_INSTANCE_TOKEN
      items:
        - variables: {port: "1/10"}
`)
	fileTemplates := map[string]string{
		"templates/endpoint.yaml": `interfaces:
  ethernets:
    - id: "${port}"
      description: "${DEVICE.name}->${[for d in DEVICES : d.variables.mgmt_ip if d.name == DEVICE.variables.peer][0]} ${token}"
      interface_groups: [HOST]
`,
	}
	for resolve, wantToken := range map[bool]string{false: "!env TF_ACC_RDC_INSTANCE_TOKEN", true: "secret"} {
		result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		cfg := getMapVal(renderedDevice(t, result, "leaf1"), "configuration")
		eth := toMapStringAny(getSliceVal(getMapVal(cfg, "interfaces"), "ethernets")[0])
		if got, want := getStringVal(eth, "description", ""), "leaf1->10.0.0.2 "+wantToken; got != want {
			t.Errorf("resolve=%v: description = %q, want %q", resolve, got, want)
		}
		if got := getIntVal(eth, "mtu", 0); got != 9216 {
			t.Errorf("resolve=%v: mtu = %d, want 9216 (from interface group)", resolve, got)
		}
	}
}

// Instances are validated against all devices, but only rendered for the
// managed devices.
func TestRenderDeviceConfigs_TemplateInstanceManagedDevices(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: service_l2
      type: file
      file: templates/service_l2.yaml
  global:
    variables:
      l2vni_base: 30000
  devices:
    - name: leaf1
    - name: leaf2
  template_instances:
    - name: WEB
      template: service_l2
      devices: [leaf1, leaf2]
      variables: {vlan_id: 1001, vlan_name: WEB}
`)
	fileTemplates := map[string]string{"templates/service_l2.yaml": serviceL2Template}
	result, err := renderDeviceConfigs(model, fileTemplates, []string{"leaf2"}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	devices := getSliceVal(getMapVal(result, "nxos"), "devices")
	if len(devices) != 1 {
		t.Fatalf("rendered %d devices, want 1", len(devices))
	}
	if got := renderedVlans(t, result, "leaf2"); !reflect.DeepEqual(got, map[string]string{"1001": "WEB/31001"}) {
		t.Errorf("leaf2: vlans = %v", got)
	}
}

// Instance and item variables may refer to other instance and item variables,
// as device group variables may refer to other variables. They are resolved
// with the variables of the item, not with a device variable of the same name.
func TestRenderDeviceConfigs_TemplateInstanceVariableReferences(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: svc
      type: model
      configuration:
        system:
          description: ${desc}
    - name: ep
      type: model
      configuration:
        interfaces:
          ethernets:
            - id: ${port}
              description: ${desc}
  devices:
    - name: leaf1
      variables:
        vlan_id: 99
        host: device
  template_instances:
    - name: SVC
      template: svc
      devices: [leaf1]
      variables: {vlan_id: 10, desc: "VLAN_${vlan_id}"}
    - name: EP
      template: ep
      devices: [leaf1]
      variables: {desc: "to ${host}"}
      items:
        - variables: {host: a, port: "1/1"}
        - variables: {host: b, port: "1/2"}
`)
	for _, resolve := range []bool{false, true} {
		result, err := renderDeviceConfigs(model, nil, nil, nil, nil, resolve)
		if err != nil {
			t.Fatalf("resolve=%v: %v", resolve, err)
		}
		system := getMapVal(getMapVal(renderedDevice(t, result, "leaf1"), "configuration"), "system")
		if got := getStringVal(system, "description", ""); got != "VLAN_10" {
			t.Errorf("resolve=%v: system.description = %q, want %q", resolve, got, "VLAN_10")
		}
		want := map[string]string{"1/1": "to a", "1/2": "to b"}
		if got := renderedEthernets(t, result, "leaf1"); !reflect.DeepEqual(got, want) {
			t.Errorf("resolve=%v: ethernets = %v, want %v", resolve, got, want)
		}
	}
}

// Template instances (and items of one instance) must not set a value in a map
// to different values on a device. Equal values (also numbers of different Go
// types), list items, other devices and device configuration overriding an
// instance value are allowed. All conflicts are reported at once.
func TestRenderDeviceConfigs_TemplateInstanceConflicts(t *testing.T) {
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: svc
      type: model
      configuration:
        system: {description: "${desc}"}
        features: {vn_segment: true}
        evpn: {vni_count: "${vni_count}"}
        vlans:
          - id: "${vlan}"
            name: "${vlan_name}"
    - name: svc_file
      type: file
      file: svc.yaml
    - name: port
      type: model
      configuration:
        system: {hostname: "${host}"}
  devices:
    - name: leaf1
    - name: leaf2
      configuration:
        system: {description: device}
  template_instances:
    - {name: A, template: svc, devices: [leaf1, leaf2], variables: {desc: a, vni_count: 10, vlan: 1001, vlan_name: WEB}}
    - {name: B, template: svc, devices: [leaf1], variables: {desc: b, vni_count: 10, vlan: 1001, vlan_name: APP}}
    - {name: F, template: svc_file, devices: [leaf1, leaf2]}
    - name: P
      template: port
      devices: [leaf1]
      items:
        - variables: {host: x}
        - name: second
          variables: {host: y}
`)
	fileTemplates := map[string]string{"svc.yaml": "features:\n  vn_segment: true\nevpn:\n  vni_count: 10\n"}
	_, err := renderDeviceConfigs(model, fileTemplates, []string{"leaf1"}, nil, nil, false)
	if err == nil {
		t.Fatal("expected conflicts on leaf1")
	}
	for _, want := range []string{
		`device leaf1: conflicting template instances:`,
		`system.description: template instance "A" sets "a", template instance "B" sets "b"`,
		`system.hostname: template instance "P" items[0] sets "x", template instance "P" items[name=second] sets "y"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	for _, unwanted := range []string{"vn_segment", "vni_count", "vlans"} {
		if strings.Contains(err.Error(), unwanted) {
			t.Errorf("error reports allowed value %s:\n%v", unwanted, err)
		}
	}

	// leaf2 only has instances A and F with equal values; its own configuration
	// overrides the instance value without a conflict
	result, err := renderDeviceConfigs(model, fileTemplates, []string{"leaf2"}, nil, nil, false)
	if err != nil {
		t.Fatalf("leaf2: %v", err)
	}
	system := getMapVal(getMapVal(renderedDevice(t, result, "leaf2"), "configuration"), "system")
	if got := getStringVal(system, "description", ""); got != "device" {
		t.Errorf("leaf2 system.description = %q, want device", got)
	}
}

// Values are compared by their text: types the Terraform modules treat the same
// are equal, integers are compared exactly.
func TestScalarsEqual(t *testing.T) {
	tests := []struct {
		a, b any
		want bool
	}{
		{"a", "a", true},
		{"a", "b", false},
		{10, int64(10), true},
		{10, "10", true},
		{int64(10), 10.0, true},
		{true, "true", true},
		{true, false, false},
		{1.5, "1.5", true},
		{1.5, 1.50, true},
		{int64(9007199254740993), int64(9007199254740992), false},
		{"10", "010", false},
		{1, true, false},
	}
	for _, tt := range tests {
		if got := scalarsEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("scalarsEqual(%#v, %#v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
