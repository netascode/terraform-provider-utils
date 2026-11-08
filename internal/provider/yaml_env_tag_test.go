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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestYamlEncode_EnvTags(t *testing.T) {
	cases := map[string]struct {
		input any
		want  string
	}{
		"map value": {
			input: map[string]any{"name": "!env ORG_NAME"},
			want:  "name: !env ORG_NAME\n",
		},
		"list item": {
			input: map[string]any{"serials": []any{"!env SERIAL_1", "plain"}},
			want:  "serials:\n  - !env SERIAL_1\n  - plain\n",
		},
		"nested ordered map": {
			input: func() any {
				m := NewOrderedMap(0)
				inner := NewOrderedMap(0)
				inner.Set("secret", "!env my_secret")
				m.Set("radius", inner)
				return m
			}(),
			want: "radius:\n  secret: !env my_secret\n",
		},
		"extra whitespace": {
			input: map[string]any{"a": "!env   VAR "},
			want:  "a: !env VAR\n",
		},
		"tag within string": {
			input: map[string]any{"a": "Bearer !env TOKEN"},
			want:  "a: Bearer !env TOKEN\n",
		},
		"invalid variable name": {
			input: map[string]any{"a": "!env FOO BAR"},
			want:  "a: \"!env FOO BAR\"\n",
		},
		"no variable name": {
			input: map[string]any{"a": "!env "},
			want:  "a: \"!env \"\n",
		},
		"other tag": {
			input: map[string]any{"a": "!vault abc"},
			want:  "a: \"!vault abc\"\n",
		},
		"top-level string": {
			input: "!env VAR",
			want:  "!env VAR\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := yamlEncode(tagYamlStrings(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// Encoding and decoding again must preserve the "!env VAR" strings, so that the
// output of yaml_encode can be used as input of the other functions.
func TestYamlEncode_EnvTagsRoundTrip(t *testing.T) {
	input := map[string]any{
		"name":    "!env ORG_NAME",
		"serials": []any{"!env SERIAL_1", "Q2XX-0000-0001"},
		"auth":    "Bearer !env TOKEN",
	}
	encoded, err := yamlEncode(tagYamlStrings(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded, err := yamlDecode(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := orderedMapToPlainMap(decoded); !reflect.DeepEqual(got, input) {
		t.Errorf("round trip mismatch:\nencoded: %s\nexpected: %#v\ngot: %#v", encoded, input, got)
	}
}

// The input value must not be modified.
func TestTagYamlStrings_DoesNotModifyInput(t *testing.T) {
	m := NewOrderedMap(0)
	m.Set("a", "!env VAR")
	list := []any{"!env VAR"}
	m.Set("b", list)
	_ = tagYamlStrings(m)
	if v, _ := m.Get("a"); v != "!env VAR" {
		t.Errorf("input map modified: %#v", v)
	}
	if list[0] != "!env VAR" {
		t.Errorf("input list modified: %#v", list[0])
	}
}

func TestYamlEncodeFunction_EnvTags(t *testing.T) {
	t.Setenv("TF_TEST_DOMAIN", "EMEA")
	t.Setenv("TF_TEST_TOKEN", "secret")
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					rendered = provider::utils::render_meraki_model([<<-EOT
						meraki:
						  templates:
						    networks:
						      - name: t
						        type: model
						        configuration:
						          notes: "Bearer $${token}"
						  domains:
						    - name: !env TF_TEST_DOMAIN
						      organizations:
						        - name: Org1
						          networks:
						            - name: Net1
						              templates: [t]
						              variables:
						                token: !env TF_TEST_TOKEN
						EOT
					], {}, "", {})
				}

				output "test" {
					value = provider::utils::yaml_encode(local.rendered.raw)
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "meraki:\n  domains:\n    - name: !env TF_TEST_DOMAIN\n      organizations:\n        - name: Org1\n          networks:\n            - name: Net1\n              notes: Bearer !env TF_TEST_TOKEN\n              templates:\n                - t\n              variables:\n                token: !env TF_TEST_TOKEN\n"),
				),
			},
		},
	})
}
