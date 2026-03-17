package provider

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestYamlMarshal_StringQuoting(t *testing.T) {
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
			expected: "secret_key: \"23211e010211\"\n",
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
				"yes_str":   "yes",
				"no_str":    "no",
			},
			expected: "false_str: \"false\"\nno_str: \"no\"\ntrue_str: \"true\"\nyes_str: \"yes\"\n",
		},
		{
			name: "null-like strings",
			input: map[string]interface{}{
				"null_str": "null",
				"tilde":    "~",
			},
			expected: "null_str: \"null\"\ntilde: \"~\"\n",
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
			name: "special YAML characters",
			input: map[string]interface{}{
				"colon":   "key: value",
				"bracket": "[list]",
				"brace":   "{map}",
				"hash":    "#comment",
			},
			expected: "brace: \"{map}\"\nbracket: \"[list]\"\ncolon: \"key: value\"\nhash: \"#comment\"\n",
		},
		{
			name: "hexadecimal patterns",
			input: map[string]interface{}{
				"hex1": "0x123",
				"hex2": "0X456",
			},
			expected: "hex1: \"0x123\"\nhex2: \"0X456\"\n",
		},
		{
			name: "octal patterns",
			input: map[string]interface{}{
				"oct1": "0o755",
				"oct2": "0O644",
			},
			expected: "oct1: \"0o755\"\noct2: \"0O644\"\n",
		},
		{
			name: "timestamp strings",
			input: map[string]interface{}{
				"expiration": "2030-01-01T00:00:00.000+00:00",
				"date_only":  "2030-01-01",
			},
			expected: "date_only: \"2030-01-01\"\nexpiration: \"2030-01-01T00:00:00.000+00:00\"\n",
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
			expected: `config:
    api_key: "23211e010211"
    enabled: true
    list:
        - "true"
        - 42
        - normal
    name: service
    null_value: null
    port: 8080
    ratio: "0.10"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := YamlMarshal(tt.input)
			if err != nil {
				t.Fatalf("YamlMarshal() error = %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("YamlMarshal() mismatch:\nGot:\n%s\nExpected:\n%s", string(result), tt.expected)
			}

			var unmarshaled map[string]interface{}
			if err := yaml.Unmarshal(result, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}
		})
	}
}

func TestYamlMarshal_TypePreservation(t *testing.T) {
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
		"string_timestamp": "2030-01-01T00:00:00.000+00:00",
	}

	result, err := YamlMarshal(input)
	if err != nil {
		t.Fatalf("YamlMarshal() error = %v", err)
	}

	var unmarshaled map[string]interface{}
	if err := yaml.Unmarshal(result, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	stringChecks := []string{"string_number", "string_bool", "string_decimal", "string_null", "string_timestamp"}
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

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"23211e010211", true},
		{".10", true},
		{"0.10", true},
		{"true", true},
		{"false", true},
		{"yes", true},
		{"no", true},
		{"null", true},
		{"~", true},
		{"12345", true},
		{"0x123", true},
		{"0o755", true},
		{"key: value", true},
		{"[list]", true},
		{"{map}", true},
		{"#comment", true},
		{"1.2e3", true},
		{"1.2E3", true},
		{"+123", true},
		{"-456", true},

		// Timestamps - should need quoting
		{"2030-01-01T00:00:00.000+00:00", true},
		{"2023-01-15T12:34:56Z", true},
		{"2023-01-15T12:34:56.789Z", true},
		{"2020-12-31T23:59:59+05:30", true},
		{"2030-01-01", true},
		{"2030-01-01 00:00:00", true},

		{"normal", false},
		{"hello world", false},
		{"some-value", false},
		{"some_value", false},
		{"CamelCase", false},
		{"value123text", false},
		{"text123", false},
		{"e10", false},
		{"E10", false},
		{"truthy", false},
		{"falsey", false},
		{"nullable", false},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := needsQuoting(tt.input)
			if result != tt.expected {
				t.Errorf("needsQuoting(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertToYamlNode_MapKeyOrdering(t *testing.T) {
	input := map[string]interface{}{
		"zebra":   1,
		"apple":   2,
		"banana":  3,
		"charlie": 4,
	}

	result1, err := YamlMarshal(input)
	if err != nil {
		t.Fatalf("YamlMarshal() error = %v", err)
	}

	result2, err := YamlMarshal(input)
	if err != nil {
		t.Fatalf("YamlMarshal() error = %v", err)
	}

	if string(result1) != string(result2) {
		t.Errorf("YamlMarshal() produced non-deterministic output:\n%s\nvs\n%s", result1, result2)
	}

	expected := "apple: 2\nbanana: 3\ncharlie: 4\nzebra: 1\n"
	if string(result1) != expected {
		t.Errorf("YamlMarshal() keys not sorted:\nGot:\n%s\nExpected:\n%s", result1, expected)
	}
}
