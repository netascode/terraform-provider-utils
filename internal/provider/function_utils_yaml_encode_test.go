package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestYamlEncodeFunction_SimpleMap(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode({b = "two", a = "one"})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "a: one\nb: two\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_NestedMap(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode({
						parent = {
							child = "value"
						}
					})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "parent:\n  child: value\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_List(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode(["a", "b", "c"])
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "  - a\n  - b\n  - c\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_DataTypes(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode({
						bool_val   = true
						null_val   = null
						number_val = 42
						string_val = "hello"
					})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "bool_val: true\nnull_val: null\nnumber_val: 42\nstring_val: hello\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_StringQuoting(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test_bool_string" {
					value = provider::utils::yaml_encode({val = "true"})
				}
				output "test_num_string" {
					value = provider::utils::yaml_encode({val = "123"})
				}
				output "test_null_string" {
					value = provider::utils::yaml_encode({val = "null"})
				}
				output "test_empty_string" {
					value = provider::utils::yaml_encode({val = ""})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test_bool_string", "val: \"true\"\n"),
					resource.TestCheckOutput("test_num_string", "val: \"123\"\n"),
					resource.TestCheckOutput("test_null_string", "val: \"null\"\n"),
					resource.TestCheckOutput("test_empty_string", "val: \"\"\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_Null(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode(null)
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "null\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_NestedListOfMaps(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode({
						servers = [
							{
								name = "web"
								port = 8080
							},
							{
								name = "db"
								port = 5432
							}
						]
					})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "servers:\n  - name: web\n    port: 8080\n  - name: db\n    port: 5432\n"),
				),
			},
		},
	})
}

func TestYamlEncodeFunction_MapKeySort(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_encode({z = 3, a = 1, m = 2})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "a: 1\nm: 2\nz: 3\n"),
				),
			},
		},
	})
}

// TestYamlEncode_UnitFormats tests the yamlEncode helper directly for exact output verification
func TestYamlEncode_UnitFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple_map",
			input:    map[string]any{"b": "two", "a": "one"},
			expected: "a: one\nb: two\n",
		},
		{
			name:     "nested_map",
			input:    map[string]any{"parent": map[string]any{"child": "value"}},
			expected: "parent:\n  child: value\n",
		},
		{
			name:     "list",
			input:    []any{"a", "b", "c"},
			expected: "  - a\n  - b\n  - c\n",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "null\n",
		},
		{
			name:     "map_key_sort",
			input:    map[string]any{"z": 3, "a": 1, "m": 2},
			expected: "a: 1\nm: 2\nz: 3\n",
		},
		{
			name:     "float",
			input:    map[string]any{"val": 42.5},
			expected: "val: 42.5\n",
		},
		{
			name:  "nested_list_of_maps",
			input: map[string]any{"servers": []any{map[string]any{"name": "web", "port": 8080}, map[string]any{"name": "db", "port": 5432}}},
			expected: "servers:\n  - name: web\n    port: 8080\n  - name: db\n    port: 5432\n",
		},
		{
			name:     "string_quoting_bool",
			input:    map[string]any{"val": "true"},
			expected: "val: \"true\"\n",
		},
		{
			name:     "string_quoting_number",
			input:    map[string]any{"val": "123"},
			expected: "val: \"123\"\n",
		},
		{
			name:     "string_quoting_null",
			input:    map[string]any{"val": "null"},
			expected: "val: \"null\"\n",
		},
		{
			name:     "string_quoting_empty",
			input:    map[string]any{"val": ""},
			expected: "val: \"\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yamlEncode(tt.input)
			if err != nil {
				t.Fatalf("yamlEncode() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("yamlEncode() mismatch:\nGot:      %q\nExpected: %q", result, tt.expected)
			}
		})
	}
}
