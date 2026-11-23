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
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

// The expected values and errors match Terraform 1.16 (checked against
// `terraform console`), including the cases of the Terraform unit tests, except
// where noted: for these inputs Terraform returns an address outside the prefix,
// an unrelated value, panics or fails at the end of the address space.

func cidrNum(s string) cty.Value {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(s)
	}
	return cty.NumberVal(new(big.Float).SetInt(n))
}

func TestTemplateCidrFunctions(t *testing.T) {
	s, n := cty.StringVal, cty.NumberIntVal
	tests := []struct {
		fn   string
		args []cty.Value
		want cty.Value // cty.NilVal if an error is expected
		err  string
	}{
		// cidrhost
		{"cidrhost", []cty.Value{s("192.168.1.0/24"), n(5)}, s("192.168.1.5"), ""},
		{"cidrhost", []cty.Value{s("192.168.1.0/24"), n(-5)}, s("192.168.1.251"), ""},
		{"cidrhost", []cty.Value{s("192.168.1.0/24"), n(-256)}, s("192.168.1.0"), ""},
		{"cidrhost", []cty.Value{s("010.001.0.0/24"), n(6)}, s("10.1.0.6"), ""},
		{"cidrhost", []cty.Value{s("10.0.0.0/08"), n(1)}, s("10.0.0.1"), ""},
		{"cidrhost", []cty.Value{s("10.0.0.5/24"), n(7)}, s("10.0.0.7"), ""},
		{"cidrhost", []cty.Value{s("10.0.0.0/32"), n(-1)}, s("10.0.0.0"), ""},
		{"cidrhost", []cty.Value{s("0.0.0.0/0"), n(-1)}, s("255.255.255.255"), ""},
		{"cidrhost", []cty.Value{s("::/0"), n(-1)}, s("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"), ""},
		{"cidrhost", []cty.Value{s("fe80::/64"), cidrNum("18446744073709551615")}, s("fe80::ffff:ffff:ffff:ffff"), ""},
		{"cidrhost", []cty.Value{s("fe80::/64"), cidrNum("-18446744073709551616")}, s("fe80::"), ""},
		{"cidrhost", []cty.Value{s("2001:db8::/32"), cidrNum("39614081257132168796771975168")}, s("2001:db8:8000::"), ""},
		{"cidrhost", []cty.Value{s("192.168.1.0/24"), n(-257)}, cty.NilVal, "prefix of 24 does not accommodate a host numbered -257"}, // Terraform: "numbered -1"
		{"cidrhost", []cty.Value{s("192.168.1.0/24"), n(256)}, cty.NilVal, "prefix of 24 does not accommodate a host numbered 256"},
		{"cidrhost", []cty.Value{s("192.168.1.0/30"), n(255)}, cty.NilVal, "prefix of 30 does not accommodate a host numbered 255"},
		{"cidrhost", []cty.Value{s("10.0.0.0/32"), n(1)}, cty.NilVal, "prefix of 32 does not accommodate a host numbered 1"},
		{"cidrhost", []cty.Value{s("fe80::/64"), cidrNum("18446744073709551616")}, cty.NilVal, "prefix of 64 does not accommodate a host numbered 18446744073709551616"}, // Terraform: fe80:0:0:1::
		{"cidrhost", []cty.Value{s("::ffff:192.168.0.0/112"), n(5)}, s("192.168.0.5"), ""},                                                                               // Terraform: 0:5::ffff:c0a8:0
		{"cidrhost", []cty.Value{s("::ffff:010.1.1.0/120"), n(3)}, s("10.1.1.3"), ""},                                                                                    // Terraform: 0:3::ffff:a01:100
		{"cidrhost", []cty.Value{s("::FFFF:10.1.1.0/120"), n(-1)}, s("10.1.1.255"), ""},
		{"cidrhost", []cty.Value{s("::ffff:10.1.1.256/120"), n(1)}, cty.NilVal, "invalid CIDR address: ::ffff:10.1.1.256/120"},
		{"cidrhost", []cty.Value{s("1.2.3.4::/64"), n(1)}, cty.NilVal, "invalid CIDR address: 1.2.3.4::/64"},
		{"cidrhost", []cty.Value{s("not-a-cidr"), n(6)}, cty.NilVal, "invalid CIDR expression: invalid CIDR address: not-a-cidr"},
		{"cidrhost", []cty.Value{s("10.256.0.0/8"), n(6)}, cty.NilVal, "invalid CIDR expression: invalid CIDR address: 10.256.0.0/8"},
		{"cidrhost", []cty.Value{s("10.0.0.0/33"), n(1)}, cty.NilVal, "invalid CIDR address: 10.0.0.0/33"},
		{"cidrhost", []cty.Value{s("fe80::/129"), n(1)}, cty.NilVal, "invalid CIDR address: fe80::/129"},
		{"cidrhost", []cty.Value{s("fe80::1%eth0/64"), n(1)}, cty.NilVal, "invalid CIDR address: fe80::1%eth0/64"},
		{"cidrhost", []cty.Value{s(" 10.0.0.0/24"), n(1)}, cty.NilVal, "invalid CIDR address:  10.0.0.0/24"},
		{"cidrhost", []cty.Value{s("10.0.0/24"), n(1)}, cty.NilVal, "invalid CIDR address: 10.0.0/24"},
		{"cidrhost", []cty.Value{s("1.2.3.4.5/24"), n(1)}, cty.NilVal, "invalid CIDR address: 1.2.3.4.5/24"},
		{"cidrhost", []cty.Value{s("10.0.0.0/"), n(1)}, cty.NilVal, "invalid CIDR address: 10.0.0.0/"},
		{"cidrhost", []cty.Value{s("10.0.0.0/+8"), n(1)}, cty.NilVal, "invalid CIDR address: 10.0.0.0/+8"},
		{"cidrhost", []cty.Value{s("10.0.0.0/8/8"), n(1)}, cty.NilVal, "invalid CIDR address: 10.0.0.0/8/8"},
		{"cidrhost", []cty.Value{s("10.0.0.0/24"), cty.NumberFloatVal(1.5)}, cty.NilVal, "value must be a whole number"},

		// cidrnetmask
		{"cidrnetmask", []cty.Value{s("192.168.1.0/24")}, s("255.255.255.0"), ""},
		{"cidrnetmask", []cty.Value{s("192.168.1.0/32")}, s("255.255.255.255"), ""},
		{"cidrnetmask", []cty.Value{s("0.0.0.0/0")}, s("0.0.0.0"), ""},
		{"cidrnetmask", []cty.Value{s("010.001.0.0/24")}, s("255.255.255.0"), ""},
		{"cidrnetmask", []cty.Value{s("10.0.0.0/1")}, s("128.0.0.0"), ""},
		{"cidrnetmask", []cty.Value{s("::ffff:192.168.0.0/112")}, s("ffff:ffff:ffff:ffff:ffff:ffff:ffff:0"), ""},
		{"cidrnetmask", []cty.Value{s("not-a-cidr")}, cty.NilVal, "invalid CIDR expression: invalid CIDR address: not-a-cidr"},
		{"cidrnetmask", []cty.Value{s("110.256.0.0/8")}, cty.NilVal, "invalid CIDR address: 110.256.0.0/8"},
		{"cidrnetmask", []cty.Value{s("1::/64")}, cty.NilVal, "IPv6 addresses cannot have a netmask: 1::/64"},

		// cidrsubnet
		{"cidrsubnet", []cty.Value{s("192.168.2.0/20"), n(4), n(6)}, s("192.168.6.0/24"), ""},
		{"cidrsubnet", []cty.Value{s("fe80::/48"), n(16), n(6)}, s("fe80:0:0:6::/64"), ""},
		{"cidrsubnet", []cty.Value{s("::ffff:192.168.0.0/112"), n(8), n(6)}, s("192.168.6.0/24"), ""},
		{"cidrsubnet", []cty.Value{s("fe80::/48"), n(33), n(6)}, s("fe80::3:0:0:0/81"), ""},
		{"cidrsubnet", []cty.Value{s("010.001.0.0/24"), n(4), n(1)}, s("10.1.0.16/28"), ""},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(0), n(0)}, s("10.0.0.0/8"), ""},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(8), n(255)}, s("10.255.0.0/16"), ""},
		{"cidrsubnet", []cty.Value{s("10.1.2.3/16"), n(8), n(4)}, s("10.1.4.0/24"), ""},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(24), n(16777215)}, s("10.255.255.255/32"), ""},
		{"cidrsubnet", []cty.Value{s("::/0"), n(64), cidrNum("9223372036854775808")}, s("8000::/64"), ""},
		{"cidrsubnet", []cty.Value{s("::/0"), n(65), cidrNum("18446744073709551616")}, s("8000::/65"), ""},
		{"cidrsubnet", []cty.Value{s("fd00::/8"), n(56), cidrNum("1099511627776")}, s("fd00:100::/64"), ""},
		{"cidrsubnet", []cty.Value{s("192.168.0.0/30"), n(4), n(6)}, cty.NilVal, "insufficient address space to extend prefix of 30 by 4"},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(25), n(0)}, cty.NilVal, "insufficient address space to extend prefix of 8 by 25"},
		{"cidrsubnet", []cty.Value{s("10.1.0.0/16"), n(-8), n(0)}, cty.NilVal, "must extend prefix by at least zero bits, not -8"}, // Terraform: 10.1.0.0/8
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(9223372036854775807), n(0)}, cty.NilVal, "insufficient address space to extend prefix of 8 by 9223372036854775807"},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(9223372036854775800), n(0)}, cty.NilVal, "insufficient address space to extend prefix of 8 by 9223372036854775800"},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(0), n(1)}, cty.NilVal, "prefix extension of 0 does not accommodate a subnet numbered 1"},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(8), n(256)}, cty.NilVal, "prefix extension of 8 does not accommodate a subnet numbered 256"},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), n(8), n(-1)}, cty.NilVal, "prefix extension of 8 does not accommodate a subnet numbered -1"},                                         // Terraform: 0.1.0.0/16
		{"cidrsubnet", []cty.Value{s("::/0"), n(64), cidrNum("18446744073709551616")}, cty.NilVal, "prefix extension of 64 does not accommodate a subnet numbered 18446744073709551616"}, // Terraform: panic
		{"cidrsubnet", []cty.Value{s("192.168.0.0/168"), n(2), n(16)}, cty.NilVal, "invalid CIDR address: 192.168.0.0/168"},
		{"cidrsubnet", []cty.Value{s("not-a-cidr"), n(4), n(6)}, cty.NilVal, "invalid CIDR expression: invalid CIDR address: not-a-cidr"},
		{"cidrsubnet", []cty.Value{s("10.0.0.0/8"), cty.NumberFloatVal(0.5), n(1)}, cty.NilVal, "value must be a whole number, between -9223372036854775808 and 9223372036854775807"},

		// cidrsubnets
		{"cidrsubnets", []cty.Value{s("10.0.0.0/21"), n(3), n(3), n(3), n(4), n(4), n(4), n(7), n(7), n(7)}, cty.ListVal([]cty.Value{
			s("10.0.0.0/24"), s("10.0.1.0/24"), s("10.0.2.0/24"), s("10.0.3.0/25"), s("10.0.3.128/25"),
			s("10.0.4.0/25"), s("10.0.4.128/28"), s("10.0.4.144/28"), s("10.0.4.160/28"),
		}), ""},
		{"cidrsubnets", []cty.Value{s("010.0.0.0/21"), n(3)}, cty.ListVal([]cty.Value{s("10.0.0.0/24")}), ""},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/16"), n(8), n(4), n(8)}, cty.ListVal([]cty.Value{s("10.0.0.0/24"), s("10.0.16.0/20"), s("10.0.32.0/24")}), ""},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/16"), n(8), n(8), n(2)}, cty.ListVal([]cty.Value{s("10.0.0.0/24"), s("10.0.1.0/24"), s("10.0.64.0/18")}), ""},
		{"cidrsubnets", []cty.Value{s("255.255.255.0/24"), n(1), n(1)}, cty.ListVal([]cty.Value{s("255.255.255.0/25"), s("255.255.255.128/25")}), ""},
		{"cidrsubnets", []cty.Value{s("0.0.0.0/8"), n(8)}, cty.ListVal([]cty.Value{s("0.0.0.0/16")}), ""},                        // Terraform: not enough remaining address space
		{"cidrsubnets", []cty.Value{s("0.0.0.0/0"), n(1), n(1)}, cty.ListVal([]cty.Value{s("0.0.0.0/1"), s("128.0.0.0/1")}), ""}, // Terraform: not enough remaining address space
		{"cidrsubnets", []cty.Value{s("10.0.0.0/8")}, cty.ListValEmpty(cty.String), ""},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/30"), n(1), n(3)}, cty.NilVal, "would extend prefix to 33 bits, which is too long for an IPv4 address"},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/8"), n(1), n(1), n(1)}, cty.NilVal, "not enough remaining address space for a subnet with a prefix of 9 bits after 10.128.0.0/9"},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/8"), n(1), n(0)}, cty.NilVal, "must extend prefix by at least one bit"},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/8"), n(1), n(-1)}, cty.NilVal, "must extend prefix by at least one bit"},
		{"cidrsubnets", []cty.Value{s("fe80::/48"), n(1), n(33)}, cty.NilVal, "may not extend prefix by more than 32 bits"},
		{"cidrsubnets", []cty.Value{s("not-a-cidr"), n(1)}, cty.NilVal, "invalid CIDR expression: invalid CIDR address: not-a-cidr"},
		{"cidrsubnets", []cty.Value{s("10.0.0.0/16"), n(8), cty.NumberFloatVal(1.5)}, cty.NilVal, "value must be a whole number"},

		// cidrcontains (OpenTofu)
		{"cidrcontains", []cty.Value{s("10.250.0.0/16"), s("10.250.1.11")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("10.250.0.0/16"), s("10.251.0.1")}, cty.False, ""},
		{"cidrcontains", []cty.Value{s("10.250.0.0/16"), s("10.250.1.0/24")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("10.250.1.0/24"), s("10.250.0.0/16")}, cty.False, ""},
		{"cidrcontains", []cty.Value{s("10.250.0.0/16"), s("010.250.001.011")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("2001:db8::/32"), s("2001:db8::1")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("10.0.0.0/8"), s("::ffff:10.1.2.3")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("10.0.0.0/8"), s("::ffff:011.1.2.3")}, cty.False, ""},
		{"cidrcontains", []cty.Value{s("::ffff:10.0.0.0/104"), s("10.1.2.3")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("::ffff:10.0.0.0/104"), s("10.1.0.0/16")}, cty.True, ""},
		{"cidrcontains", []cty.Value{s("2001:db8::/32"), s("10.1.2.3")}, cty.NilVal, "address family mismatch: 2001:db8::/32 vs. 10.1.2.3"},
		{"cidrcontains", []cty.Value{s("10.0.0.0/8"), s("2001:db8::1")}, cty.NilVal, "address family mismatch: 10.0.0.0/8 vs. 2001:db8::1"},
		{"cidrcontains", []cty.Value{s("not-a-cidr"), s("10.0.0.1")}, cty.NilVal, "invalid CIDR address: not-a-cidr"},
		{"cidrcontains", []cty.Value{s("10.0.0.0/8"), s("10.0.0.256")}, cty.NilVal, "invalid IP address or prefix: 10.0.0.256"},
	}
	for _, tt := range tests {
		got, err := templateFunctions[tt.fn].Call(tt.args)
		name := tt.fn + "(" + cidrArgs(tt.args) + ")"
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("%s: error = %v, want %q", name, err, tt.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
			continue
		}
		if !got.RawEquals(tt.want) {
			t.Errorf("%s = %#v, want %#v", name, got, tt.want)
		}
	}
}

func cidrArgs(args []cty.Value) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if a.Type() == cty.String {
			parts[i] = `"` + a.AsString() + `"`
		} else {
			parts[i] = a.AsBigFloat().Text('f', -1)
		}
	}
	return strings.Join(parts, ", ")
}

// cidrTemplateExpr uses all IP network functions with the variable prefix
// 10.10.0.0/16 and renders to cidrTemplateWant.
const cidrTemplateExpr = `${cidrhost(cidrsubnet(prefix, 8, 1), 1)} ${cidrnetmask(cidrsubnet(prefix, 8, 1))} ${cidrsubnets(prefix, 8, 8)[1]} ${cidrcontains(prefix, cidrhost(prefix, -2))}`

const cidrTemplateWant = "10.10.1.1 255.255.255.0 10.10.1.0/24 true"

// The IP network functions are available in all template types of all render
// functions.
func TestTemplateCidrFunctions_RenderFunctions(t *testing.T) {
	t.Run("render_meraki_model", func(t *testing.T) {
		model := mustYamlModel(t, `
meraki:
  templates:
    networks:
      - name: model_tmpl
        type: model
        variables:
          computed: '`+cidrTemplateExpr+`'
        configuration:
          notes: '`+cidrTemplateExpr+`'
          tags: ['${computed}']
      - name: file_tmpl
        type: file
        file: network.yaml
    devices:
      - name: device_tmpl
        type: model
        configuration:
          notes: '`+cidrTemplateExpr+`'
  domains:
    - name: EMEA
      organizations:
        - name: Org1
          networks:
            - name: Net1
              templates: [model_tmpl, file_tmpl]
              variables:
                prefix: 10.10.0.0/16
              devices:
                - name: dev1
                  templates: [device_tmpl]
`)
		fileTemplates := map[string]string{"network.yaml": "time_zone: " + cidrTemplateExpr + "\n"}
		for _, resolve := range []bool{false, true} {
			result, err := renderMerakiModel(model, fileTemplates, resolve)
			if err != nil {
				t.Fatalf("resolve=%v: %v", resolve, err)
			}
			net := firstNetwork(t, result)
			got := []string{
				getStringVal(net, "notes", ""),
				getStringVal(net, "time_zone", ""),
				getStringVal(toMapStringAny(getSliceVal(net, "devices")[0]), "notes", ""),
				fmt.Sprint(getSliceVal(net, "tags")),
			}
			want := []string{cidrTemplateWant, cidrTemplateWant, cidrTemplateWant, "[" + cidrTemplateWant + "]"}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("resolve=%v: network notes, file template, device notes, computed variable = %q, want %q", resolve, got, want)
			}
		}
	})

	t.Run("render_device_configs", func(t *testing.T) {
		model := mustYamlModel(t, `
nxos:
  templates:
    - name: model_tmpl
      type: model
      configuration:
        system:
          description: '`+cidrTemplateExpr+`'
    - name: file_tmpl
      type: file
      file: device.yaml
    - name: cli_tmpl
      type: cli
      content: '`+cidrTemplateExpr+`'
    - name: instance_tmpl
      type: cli
      content: '`+cidrTemplateExpr+`'
  devices:
    - name: leaf1
      templates: [model_tmpl, file_tmpl, cli_tmpl]
      variables:
        prefix: 10.10.0.0/16
  template_instances:
    - name: INST
      template: instance_tmpl
      devices: [leaf1]
`)
		fileTemplates := map[string]string{"device.yaml": "system:\n  hostname: " + cidrTemplateExpr + "\n"}
		result, err := renderDeviceConfigs(model, fileTemplates, nil, nil, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		device := renderedDevice(t, result, "leaf1")
		system := getMapVal(getMapVal(device, "configuration"), "system")
		got := map[string]string{
			"model": getStringVal(system, "description", ""),
			"file":  getStringVal(system, "hostname", ""),
		}
		for _, c := range getSliceVal(device, "cli_templates") {
			m := toMapStringAny(c)
			got[getStringVal(m, "name", "")] = getStringVal(m, "content", "")
		}
		want := map[string]string{
			"model":                       cidrTemplateWant,
			"file":                        cidrTemplateWant,
			"cli_tmpl":                    cidrTemplateWant,
			"instance_tmpl/instance/INST": cidrTemplateWant,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("rendered = %q, want %q", got, want)
		}
	})
}
