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
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Unit tests for the resolveYamlTags helper

func TestResolveYamlTags_ResolveEnvTag(t *testing.T) {
	os.Setenv("TEST_RESOLVE_VAR", "resolved_value")
	defer os.Unsetenv("TEST_RESOLVE_VAR")

	result, err := resolveYamlTags("!env TEST_RESOLVE_VAR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "resolved_value" {
		t.Errorf("expected 'resolved_value', got %v", result)
	}
}

func TestResolveYamlTags_NestedEnvTags(t *testing.T) {
	os.Setenv("TEST_DB_URL", "postgres://localhost")
	os.Setenv("TEST_API_KEY", "secret123")
	defer os.Unsetenv("TEST_DB_URL")
	defer os.Unsetenv("TEST_API_KEY")

	input := map[string]any{
		"config": map[string]any{
			"database": "!env TEST_DB_URL",
			"api_key":  "!env TEST_API_KEY",
			"timeout":  30,
		},
		"items": []any{
			"!env TEST_DB_URL",
			"plain_string",
			42,
		},
	}

	result, err := resolveYamlTags(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]any)
	config := m["config"].(map[string]any)
	if config["database"] != "postgres://localhost" {
		t.Errorf("database: expected 'postgres://localhost', got %v", config["database"])
	}
	if config["api_key"] != "secret123" {
		t.Errorf("api_key: expected 'secret123', got %v", config["api_key"])
	}
	if config["timeout"] != 30 {
		t.Errorf("timeout: expected 30, got %v", config["timeout"])
	}

	items := m["items"].([]any)
	if items[0] != "postgres://localhost" {
		t.Errorf("items[0]: expected 'postgres://localhost', got %v", items[0])
	}
	if items[1] != "plain_string" {
		t.Errorf("items[1]: expected 'plain_string', got %v", items[1])
	}
	if items[2] != 42 {
		t.Errorf("items[2]: expected 42, got %v", items[2])
	}
}

func TestResolveYamlTags_NoTags(t *testing.T) {
	input := map[string]any{
		"name":    "hello",
		"count":   42,
		"enabled": true,
		"nothing": nil,
	}

	result, err := resolveYamlTags(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]any)
	if m["name"] != "hello" {
		t.Errorf("name: expected 'hello', got %v", m["name"])
	}
	if m["count"] != 42 {
		t.Errorf("count: expected 42, got %v", m["count"])
	}
	if m["enabled"] != true {
		t.Errorf("enabled: expected true, got %v", m["enabled"])
	}
	if m["nothing"] != nil {
		t.Errorf("nothing: expected nil, got %v", m["nothing"])
	}
}

func TestResolveYamlTags_UnsetEnvError(t *testing.T) {
	os.Unsetenv("TEST_UNSET_VAR_XYZZY")

	_, err := resolveYamlTags("!env TEST_UNSET_VAR_XYZZY")
	if err == nil {
		t.Fatal("expected error for unset environment variable")
	}
	if matched, _ := regexp.MatchString(`environment variable.*not set`, err.Error()); !matched {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveYamlTags_NilInput(t *testing.T) {
	result, err := resolveYamlTags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestResolveYamlTags_EmptyMap(t *testing.T) {
	result, err := resolveYamlTags(map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestResolveYamlTags_StringWithoutTag(t *testing.T) {
	// Strings that don't start with "!env " should pass through unchanged
	tests := []string{
		"hello",
		"!envNOSPACE",
		"env VALUE",
		"!secret VALUE",
		"",
	}
	for _, s := range tests {
		result, err := resolveYamlTags(s)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", s, err)
		}
		if result != s {
			t.Errorf("expected %q, got %v", s, result)
		}
	}
}

// Acceptance tests for the Terraform function

func TestResolveYamlTagsFunction_Basic(t *testing.T) {
	os.Setenv("TEST_RESOLVE_DB", "postgres://localhost:5432/db")
	defer os.Unsetenv("TEST_RESOLVE_DB")

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					data = {
						database = "!env TEST_RESOLVE_DB"
						name     = "myapp"
						port     = 5432
					}
				}
				output "test" {
					value = jsonencode(provider::utils::resolve_yaml_tags(local.data))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"database":"postgres://localhost:5432/db","name":"myapp","port":5432}`),
				),
			},
		},
	})
}

func TestResolveYamlTagsFunction_Nested(t *testing.T) {
	os.Setenv("TEST_RESOLVE_URL", "https://example.com")
	defer os.Unsetenv("TEST_RESOLVE_URL")

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					data = {
						config = {
							url = "!env TEST_RESOLVE_URL"
						}
					}
				}
				output "test" {
					value = jsonencode(provider::utils::resolve_yaml_tags(local.data))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"config":{"url":"https://example.com"}}`),
				),
			},
		},
	})
}

func TestResolveYamlTagsFunction_NoTags(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					data = {
						name = "hello"
						port = 8080
					}
				}
				output "test" {
					value = jsonencode(provider::utils::resolve_yaml_tags(local.data))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"name":"hello","port":8080}`),
				),
			},
		},
	})
}

func TestResolveYamlTagsFunction_UnsetEnvError(t *testing.T) {
	os.Unsetenv("TEST_RESOLVE_MISSING_XYZZY")

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					data = {
						val = "!env TEST_RESOLVE_MISSING_XYZZY"
					}
				}
				output "test" {
					value = jsonencode(provider::utils::resolve_yaml_tags(local.data))
				}
				`,
				ExpectError: regexp.MustCompile(`environment variable.*not set`),
			},
		},
	})
}

func TestResolveYamlTagsFunction_Null(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::resolve_yaml_tags(null) == null ? "null" : "not_null"
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "null"),
				),
			},
		},
	})
}

func TestResolveYamlTagsFunction_RoundTrip(t *testing.T) {
	os.Setenv("TEST_RESOLVE_RT", "resolved_value")
	defer os.Unsetenv("TEST_RESOLVE_RT")

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					decoded  = provider::utils::yaml_decode("name: myapp\nval: !env TEST_RESOLVE_RT\n")
					resolved = provider::utils::resolve_yaml_tags(local.decoded)
				}
				output "test" {
					value = jsonencode(local.resolved)
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"name":"myapp","val":"resolved_value"}`),
				),
			},
		},
	})
}
