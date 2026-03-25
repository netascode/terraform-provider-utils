package provider

import (
	"testing"
)

func TestYamlEncode_StringQuoting(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name: "scientific notation pattern - issue #155",
			input: map[string]interface{}{
				"secret_key": "23211e010211",
			},
			expected: "secret_key: 23211e010211\n",
		},
		{
			name: "leading decimal point",
			input: map[string]interface{}{
				"value": ".10",
			},
			expected: "value: \".10\"\n",
		},
		{
			name: "decimal number string",
			input: map[string]interface{}{
				"value": "0.10",
			},
			expected: "value: \"0.10\"\n",
		},
		{
			name: "boolean-like strings",
			input: map[string]interface{}{
				"true_str":  "true",
				"false_str": "false",
			},
			expected: "false_str: \"false\"\ntrue_str: \"true\"\n",
		},
		{
			name: "null-like strings",
			input: map[string]interface{}{
				"null_str": "null",
			},
			expected: "null_str: \"null\"\n",
		},
		{
			name: "numeric strings",
			input: map[string]interface{}{
				"number": "12345",
			},
			expected: "number: \"12345\"\n",
		},
		{
			name: "actual numbers - no quoting",
			input: map[string]interface{}{
				"int_val":   42,
				"float_val": 3.14,
			},
			expected: "float_val: 3.14\nint_val: 42\n",
		},
		{
			name: "actual booleans - no quoting",
			input: map[string]interface{}{
				"bool_true":  true,
				"bool_false": false,
			},
			expected: "bool_false: false\nbool_true: true\n",
		},
		{
			name: "normal strings - no quoting",
			input: map[string]interface{}{
				"name":        "alice",
				"description": "some text here",
			},
			expected: "description: some text here\nname: alice\n",
		},
		{
			name: "nested structures with mixed types",
			input: map[string]interface{}{
				"config": map[string]interface{}{
					"api_key":    "23211e010211",
					"port":       8080,
					"enabled":    true,
					"name":       "service",
					"ratio":      "0.10",
					"list":       []interface{}{"true", 42, "normal"},
					"null_value": nil,
				},
			},
			expected: "config:\n  api_key: 23211e010211\n  enabled: true\n  list:\n    - \"true\"\n    - 42\n    - normal\n  name: service\n  null_value: null\n  port: 8080\n  ratio: \"0.10\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yamlEncode(tt.input)
			if err != nil {
				t.Fatalf("yamlEncode() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("yamlEncode() mismatch:\nGot:\n%s\nExpected:\n%s", result, tt.expected)
			}

			// Verify round-trip: the output can be decoded back
			_, err = yamlDecode(result)
			if err != nil {
				t.Fatalf("Failed to decode yamlEncode result: %v", err)
			}
		})
	}
}

func TestYamlEncode_TypePreservation(t *testing.T) {
	input := map[string]interface{}{
		"string_number":    "23211e010211",
		"actual_number":    23211,
		"string_bool":      "true",
		"actual_bool":      true,
		"string_decimal":   "0.10",
		"actual_decimal":   0.10,
		"string_null":      "null",
		"actual_null":      nil,
		"normal_string":    "hello",
		"empty_string":     "",
		"string_with_dash": "some-value",
	}

	result, err := yamlEncode(input)
	if err != nil {
		t.Fatalf("yamlEncode() error = %v", err)
	}

	decoded, err := yamlDecode(result)
	if err != nil {
		t.Fatalf("Failed to decode yamlEncode result: %v", err)
	}

	unmarshaled, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("Expected decoded result to be map[string]any, got %T", decoded)
	}

	stringChecks := []string{"string_number", "string_bool", "string_decimal", "string_null"}
	for _, key := range stringChecks {
		if _, ok := unmarshaled[key].(string); !ok {
			t.Errorf("Expected %s to be string, got %T", key, unmarshaled[key])
		}
	}

	if _, ok := unmarshaled["actual_number"].(int); !ok {
		t.Errorf("Expected actual_number to be int, got %T", unmarshaled["actual_number"])
	}
	if _, ok := unmarshaled["actual_bool"].(bool); !ok {
		t.Errorf("Expected actual_bool to be bool, got %T", unmarshaled["actual_bool"])
	}
	if unmarshaled["actual_null"] != nil {
		t.Errorf("Expected actual_null to be nil, got %v", unmarshaled["actual_null"])
	}
}

func TestYamlEncode_MapKeyOrdering(t *testing.T) {
	input := map[string]interface{}{
		"zebra":   1,
		"apple":   2,
		"banana":  3,
		"charlie": 4,
	}

	result1, err := yamlEncode(input)
	if err != nil {
		t.Fatalf("yamlEncode() error = %v", err)
	}

	result2, err := yamlEncode(input)
	if err != nil {
		t.Fatalf("yamlEncode() error = %v", err)
	}

	if result1 != result2 {
		t.Errorf("yamlEncode() produced non-deterministic output:\n%s\nvs\n%s", result1, result2)
	}

	expected := "apple: 2\nbanana: 3\ncharlie: 4\nzebra: 1\n"
	if result1 != expected {
		t.Errorf("yamlEncode() keys not sorted:\nGot:\n%s\nExpected:\n%s", result1, expected)
	}
}
