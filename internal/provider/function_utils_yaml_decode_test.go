package provider

import (
	"math"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// toNativeMap recursively converts *OrderedMap to map[string]any for test assertions.
func toNativeMap(v any) any {
	switch val := v.(type) {
	case *OrderedMap:
		result := make(map[string]any, val.Len())
		for _, e := range val.Entries() {
			result[e.Key] = toNativeMap(e.Value)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = toNativeMap(item)
		}
		return result
	default:
		return v
	}
}

// Unit tests for the yamlDecode helper

func TestYamlDecode_SimpleMap(t *testing.T) {
	result, err := yamlDecode("a: one\nb: two\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := toNativeMap(result).(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["a"] != "one" || m["b"] != "two" {
		t.Errorf("unexpected result: %v", m)
	}
}

func TestYamlDecode_NestedMap(t *testing.T) {
	result, err := yamlDecode("parent:\n  child: value\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	parent := m["parent"].(map[string]any)
	if parent["child"] != "value" {
		t.Errorf("unexpected result: %v", parent)
	}
}

func TestYamlDecode_List(t *testing.T) {
	result, err := yamlDecode("- a\n- b\n- c\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(list) != 3 || list[0] != "a" || list[1] != "b" || list[2] != "c" {
		t.Errorf("unexpected result: %v", list)
	}
}

func TestYamlDecode_DataTypes(t *testing.T) {
	input := "bool_val: true\nfloat_val: 3.14\nnull_val: null\nnumber_val: 42\nstring_val: hello\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)

	if m["bool_val"] != true {
		t.Errorf("bool_val: expected true, got %v (%T)", m["bool_val"], m["bool_val"])
	}
	if m["number_val"] != 42 {
		t.Errorf("number_val: expected 42, got %v (%T)", m["number_val"], m["number_val"])
	}
	if m["float_val"] != 3.14 {
		t.Errorf("float_val: expected 3.14, got %v (%T)", m["float_val"], m["float_val"])
	}
	if m["string_val"] != "hello" {
		t.Errorf("string_val: expected hello, got %v (%T)", m["string_val"], m["string_val"])
	}
	if m["null_val"] != nil {
		t.Errorf("null_val: expected nil, got %v (%T)", m["null_val"], m["null_val"])
	}
}

func TestYamlDecode_UnknownTagPreservation(t *testing.T) {
	result, err := yamlDecode("val: !env ABC\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	if m["val"] != "!env ABC" {
		t.Errorf("expected '!env ABC', got %v", m["val"])
	}
}

func TestYamlDecode_MultipleUnknownTags(t *testing.T) {
	input := "db: !env DATABASE_URL\nkey: !secret s3cr3t\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	if m["db"] != "!env DATABASE_URL" {
		t.Errorf("db: expected '!env DATABASE_URL', got %v", m["db"])
	}
	if m["key"] != "!secret s3cr3t" {
		t.Errorf("key: expected '!secret s3cr3t', got %v", m["key"])
	}
}

func TestYamlDecode_StandardTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		key      string
		expected any
	}{
		{"str", "val: !!str 123\n", "val", "123"},
		{"int", "val: !!int 42\n", "val", 42},
		{"float", "val: !!float 3.14\n", "val", 3.14},
		{"bool", "val: !!bool true\n", "val", true},
		{"null", "val: !!null null\n", "val", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yamlDecode(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := toNativeMap(result).(map[string]any)
			if m[tt.key] != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, m[tt.key], m[tt.key])
			}
		})
	}
}

func TestYamlDecode_TagOnNonScalarError(t *testing.T) {
	_, err := yamlDecode("val: !custom\n  key: value\n")
	if err == nil {
		t.Fatal("expected error for tag on non-scalar value")
	}
	if matched, _ := regexp.MatchString(`unsupported tag.*non-scalar`, err.Error()); !matched {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestYamlDecode_EmptyDocument(t *testing.T) {
	result, err := yamlDecode("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestYamlDecode_NullDocument(t *testing.T) {
	result, err := yamlDecode("null\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestYamlDecode_MultiDocumentError(t *testing.T) {
	_, err := yamlDecode("---\na: 1\n---\nb: 2\n")
	if err == nil {
		t.Fatal("expected error for multiple documents")
	}
	if matched, _ := regexp.MatchString(`multiple YAML documents`, err.Error()); !matched {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestYamlDecode_Anchors(t *testing.T) {
	result, err := yamlDecode("anchor: &myval hello\nalias: *myval\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	if m["anchor"] != "hello" {
		t.Errorf("anchor: expected 'hello', got %v", m["anchor"])
	}
	if m["alias"] != "hello" {
		t.Errorf("alias: expected 'hello', got %v", m["alias"])
	}
}

func TestYamlDecode_AliasDeepCopy(t *testing.T) {
	// Two aliases referencing the same anchored map must produce independent copies.
	// Mutating one must not affect the other or the original anchor value.
	input := "base: &base\n  key1: val1\n  key2: val2\ncopy1: *base\ncopy2: *base\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	base := m["base"].(map[string]any)
	copy1 := m["copy1"].(map[string]any)
	copy2 := m["copy2"].(map[string]any)

	// Verify initial values match
	if copy1["key1"] != "val1" || copy1["key2"] != "val2" {
		t.Fatalf("copy1 has wrong initial values: %v", copy1)
	}
	if copy2["key1"] != "val1" || copy2["key2"] != "val2" {
		t.Fatalf("copy2 has wrong initial values: %v", copy2)
	}

	// Mutate copy1 — must not affect copy2 or base
	copy1["key1"] = "mutated"
	if copy2["key1"] != "val1" {
		t.Errorf("mutating copy1 affected copy2: copy2[key1] = %v, want 'val1'", copy2["key1"])
	}
	if base["key1"] != "val1" {
		t.Errorf("mutating copy1 affected base: base[key1] = %v, want 'val1'", base["key1"])
	}
}

func TestYamlDecode_ListOfMaps(t *testing.T) {
	input := "items:\n  - name: a\n    val: 1\n  - name: b\n    val: 2\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	items := m["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	item0 := items[0].(map[string]any)
	if item0["name"] != "a" || item0["val"] != 1 {
		t.Errorf("item[0]: unexpected %v", item0)
	}
}

func TestYamlDecode_InfinityAndNaN(t *testing.T) {
	input := "inf_val: .inf\nneginf_val: -.inf\nnan_val: .nan\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)

	infVal, ok := m["inf_val"].(float64)
	if !ok || !math.IsInf(infVal, 1) {
		t.Errorf("inf_val: expected +Inf, got %v", m["inf_val"])
	}
	negInfVal, ok := m["neginf_val"].(float64)
	if !ok || !math.IsInf(negInfVal, -1) {
		t.Errorf("neginf_val: expected -Inf, got %v", m["neginf_val"])
	}
	nanVal, ok := m["nan_val"].(float64)
	if !ok || !math.IsNaN(nanVal) {
		t.Errorf("nan_val: expected NaN, got %v", m["nan_val"])
	}
}

func TestYamlDecode_MergeKey(t *testing.T) {
	input := "defaults: &defaults\n  adapter: postgres\n  host: localhost\nproduction:\n  <<: *defaults\n  database: mydb\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	prod := m["production"].(map[string]any)
	if prod["adapter"] != "postgres" {
		t.Errorf("expected 'postgres', got %v", prod["adapter"])
	}
	if prod["host"] != "localhost" {
		t.Errorf("expected 'localhost', got %v", prod["host"])
	}
	if prod["database"] != "mydb" {
		t.Errorf("expected 'mydb', got %v", prod["database"])
	}
}

func TestYamlDecode_LiteralBlock(t *testing.T) {
	input := "val: |\n  line1\n  line2\n"
	result, err := yamlDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := toNativeMap(result).(map[string]any)
	expected := "line1\nline2\n"
	if m["val"] != expected {
		t.Errorf("expected %q, got %q", expected, m["val"])
	}
}

// Acceptance tests for the Terraform function

func TestYamlDecodeFunction_SimpleMap(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = jsonencode(provider::utils::yaml_decode("a: one\nb: two\n"))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"a":"one","b":"two"}`),
				),
			},
		},
	})
}

func TestYamlDecodeFunction_UnknownTag(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = jsonencode(provider::utils::yaml_decode("val: !env ABC\n"))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"val":"!env ABC"}`),
				),
			},
		},
	})
}

func TestYamlDecodeFunction_DataTypes(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = jsonencode(provider::utils::yaml_decode("bool_val: true\nnumber_val: 42\nstring_val: hello\n"))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `{"bool_val":true,"number_val":42,"string_val":"hello"}`),
				),
			},
		},
	})
}

func TestYamlDecodeFunction_List(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = jsonencode(provider::utils::yaml_decode("- a\n- b\n- c\n"))
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", `["a","b","c"]`),
				),
			},
		},
	})
}

func TestYamlDecodeFunction_TagOnNonScalarError(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_decode("val: !custom\n  key: value\n")
				}
				`,
				ExpectError: regexp.MustCompile(`unsupported tag.*non-scalar`),
			},
		},
	})
}

func TestYamlDecodeFunction_MultiDocumentError(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::utils::yaml_decode("---\na: 1\n---\nb: 2\n")
				}
				`,
				ExpectError: regexp.MustCompile(`multiple YAML documents`),
			},
		},
	})
}
