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
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestCollectYamlTagValues(t *testing.T) {
	t.Setenv("TF_TEST_MASK_A", "alpha")
	t.Setenv("TF_TEST_MASK_B", "beta")
	model := mustYamlModel(t, `
a: !env TF_TEST_MASK_A
b:
  - !env TF_TEST_MASK_B
  - nested:
      c: !env TF_TEST_MASK_A
d: !env TF_TEST_MASK_UNSET
e: "prefix !env TF_TEST_MASK_A"
f: plain
`)
	got := collectYamlTagValues(model)
	sort.Strings(got)
	if strings.Join(got, ",") != "alpha,beta" {
		t.Errorf("expected [alpha beta], got %v", got)
	}
}

func TestMaskSecrets(t *testing.T) {
	cases := map[string]struct {
		msg     string
		secrets []string
		want    string
	}{
		"exact":            {`cannot convert "s3cret" to number`, []string{"s3cret"}, `cannot convert "***" to number`},
		"upper case":       {`value "S3CRET" invalid`, []string{"s3cret"}, `value "***" invalid`},
		"lower case":       {`value "s3cret" invalid`, []string{"S3CRET"}, `value "***" invalid`},
		"multiple":         {`"user:pass" and "pass"`, []string{"user", "pass"}, `"***:***" and "***"`},
		"longest first":    {`"abcdef"`, []string{"abc", "abcdef"}, `"***"`},
		"empty ignored":    {`nothing`, []string{""}, `nothing`},
		"no secrets":       {`plain message`, nil, `plain message`},
		"repeated":         {`x x x`, []string{"x"}, `*** *** ***`},
		"special chars":    {`got "a.b*c"`, []string{"a.b*c"}, `got "***"`},
		"no partial match": {`token value`, []string{"tok3n"}, `token value`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := maskSecrets(tc.msg, tc.secrets); got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// A template that succeeds with the !env tag text (first pass) but fails with
// the resolved value (second pass) must not expose the value in the error.
func TestRenderDeviceConfigs_ResolvedRenderErrorMasksSecrets(t *testing.T) {
	t.Setenv("TF_TEST_MASK_TOKEN", "tok123secret")
	t.Setenv("TF_TEST_MASK_ENV", "prod")
	model := mustYamlModel(t, `
nxos:
  templates:
    - name: t
      type: model
      configuration:
        demo:
          port: '${env_name == "prod" ? tonumber(upper(token)) : 0}'
  global:
    templates: [t]
    variables:
      token: !env TF_TEST_MASK_TOKEN
      env_name: !env TF_TEST_MASK_ENV
  devices:
    - name: leaf1
`)
	if _, err := renderDeviceConfigs(model, nil, nil, nil, nil, false); err != nil {
		t.Fatalf("first pass should succeed with tag text, got: %v", err)
	}
	_, err := renderDeviceConfigs(model, nil, nil, nil, nil, true)
	if err == nil {
		t.Fatalf("second pass should fail with resolved value")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "tok123secret") {
		t.Fatalf("test precondition: unmasked error should contain the value, got: %v", err)
	}
	masked := resolvedRenderError(err, model).Error()
	if strings.Contains(strings.ToLower(masked), "tok123secret") {
		t.Errorf("masked error still contains the secret: %s", masked)
	}
	if !strings.Contains(masked, secretMask) {
		t.Errorf("masked error does not contain the mask: %s", masked)
	}
}

func TestRenderDeviceConfigsFunction_ResolvedRenderErrorMasksSecrets(t *testing.T) {
	t.Setenv("TF_TEST_MASK_TOKEN", "tok123secret")
	t.Setenv("TF_TEST_MASK_ENV", "prod")
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
					nxos:
					  templates:
					    - name: t
					      type: model
					      configuration:
					        demo:
					          port: '$${env_name == "prod" ? tonumber(token) : 0}'
					  global:
					    templates: [t]
					    variables:
					      token: !env TF_TEST_MASK_TOKEN
					      env_name: !env TF_TEST_MASK_ENV
					  devices:
					    - name: leaf1
					EOT
					result = provider::utils::render_device_configs([local.yaml], {}, "", {}, [], [])
				}

				output "result" {
					value = local.result.resolved
				}
				`,
				ExpectError: regexp.MustCompile(`values\s+of\s+!env\s+tags\s+are\s+masked[\s\S]*cannot\s+convert\s+"\*\*\*"\s+to\s+number`),
			},
		},
	})
}

func TestRenderMerakiModelFunction_ResolvedRenderErrorMasksSecrets(t *testing.T) {
	t.Setenv("TF_TEST_MASK_TOKEN", "tok123secret")
	t.Setenv("TF_TEST_MASK_ENV", "prod")
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
					      - name: t
					        type: model
					        configuration:
					          notes: '$${env_name == "prod" ? tonumber(token) : 0}'
					  domains:
					    - name: EMEA
					      organizations:
					        - name: Org1
					          networks:
					            - name: Net1
					              templates: [t]
					              variables:
					                token: !env TF_TEST_MASK_TOKEN
					                env_name: !env TF_TEST_MASK_ENV
					EOT
					result = provider::utils::render_meraki_model([local.yaml], {}, "", {})
				}

				output "result" {
					value = local.result.resolved
				}
				`,
				ExpectError: regexp.MustCompile(`values\s+of\s+!env\s+tags\s+are\s+masked[\s\S]*cannot\s+convert\s+"\*\*\*"\s+to\s+number`),
			},
		},
	})
}
