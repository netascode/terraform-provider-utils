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
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestNativeToCty_String(t *testing.T) {
	v, err := nativeToCty("hello")
	if err != nil {
		t.Fatal(err)
	}
	if v.Type() != cty.String || v.AsString() != "hello" {
		t.Fatalf("expected string 'hello', got %v", v)
	}
}

func TestNativeToCty_Bool(t *testing.T) {
	v, err := nativeToCty(true)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type() != cty.Bool || !v.True() {
		t.Fatalf("expected true, got %v", v)
	}
}

func TestNativeToCty_Int(t *testing.T) {
	v, err := nativeToCty(42)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type() != cty.Number {
		t.Fatalf("expected number, got %v", v.Type())
	}
	bf := v.AsBigFloat()
	i, _ := bf.Int64()
	if i != 42 {
		t.Fatalf("expected 42, got %d", i)
	}
}

func TestNativeToCty_Float(t *testing.T) {
	v, err := nativeToCty(3.14)
	if err != nil {
		t.Fatal(err)
	}
	if v.Type() != cty.Number {
		t.Fatalf("expected number, got %v", v.Type())
	}
	f, _ := v.AsBigFloat().Float64()
	if f != 3.14 {
		t.Fatalf("expected 3.14, got %f", f)
	}
}

func TestNativeToCty_Nil(t *testing.T) {
	v, err := nativeToCty(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsNull() {
		t.Fatal("expected null")
	}
}

func TestNativeToCty_Map(t *testing.T) {
	v, err := nativeToCty(map[string]any{
		"name": "test",
		"port": 443,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Type().IsObjectType() {
		t.Fatalf("expected object, got %v", v.Type())
	}
	name := v.GetAttr("name")
	if name.AsString() != "test" {
		t.Fatalf("expected 'test', got %v", name)
	}
}

func TestNativeToCty_EmptyMap(t *testing.T) {
	v, err := nativeToCty(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Type().IsObjectType() || v.LengthInt() != 0 {
		t.Fatalf("expected empty object, got %v", v)
	}
}

func TestNativeToCty_Slice(t *testing.T) {
	v, err := nativeToCty([]any{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Type().IsTupleType() {
		t.Fatalf("expected tuple, got %v", v.Type())
	}
	if v.LengthInt() != 3 {
		t.Fatalf("expected length 3, got %d", v.LengthInt())
	}
}

func TestNativeToCty_EmptySlice(t *testing.T) {
	v, err := nativeToCty([]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Type().IsTupleType() || v.LengthInt() != 0 {
		t.Fatalf("expected empty tuple, got %v", v)
	}
}

func TestNativeToCty_NestedStructure(t *testing.T) {
	v, err := nativeToCty(map[string]any{
		"devices": []any{
			map[string]any{
				"name": "switch1",
				"port": 443,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	devices := v.GetAttr("devices")
	it := devices.ElementIterator()
	it.Next()
	_, device := it.Element()
	name := device.GetAttr("name")
	if name.AsString() != "switch1" {
		t.Fatalf("expected 'switch1', got %v", name)
	}
}

func TestRenderHCLTemplate_SimpleInterpolation(t *testing.T) {
	vars := map[string]cty.Value{
		"name": cty.StringVal("switch1"),
	}
	result, err := renderHCLTemplate("hostname: ${name}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hostname: switch1" {
		t.Fatalf("expected 'hostname: switch1', got %q", result)
	}
}

func TestRenderHCLTemplate_NestedAccess(t *testing.T) {
	vars := map[string]cty.Value{
		"device": cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("leaf1"),
			"ip":   cty.StringVal("10.0.0.1"),
		}),
	}
	result, err := renderHCLTemplate("name=${device.name} ip=${device.ip}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "name=leaf1 ip=10.0.0.1" {
		t.Fatalf("expected 'name=leaf1 ip=10.0.0.1', got %q", result)
	}
}

func TestRenderHCLTemplate_ForLoop(t *testing.T) {
	vars := map[string]cty.Value{
		"items": cty.TupleVal([]cty.Value{
			cty.StringVal("a"),
			cty.StringVal("b"),
			cty.StringVal("c"),
		}),
	}
	tmpl := "%{ for item in items }${item}%{ endfor }"
	result, err := renderHCLTemplate(tmpl, vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "abc" {
		t.Fatalf("expected 'abc', got %q", result)
	}
}

func TestRenderHCLTemplate_IfConditional(t *testing.T) {
	vars := map[string]cty.Value{
		"enabled": cty.True,
	}
	tmpl := "%{ if enabled }yes%{ else }no%{ endif }"
	result, err := renderHCLTemplate(tmpl, vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "yes" {
		t.Fatalf("expected 'yes', got %q", result)
	}
}

func TestRenderHCLTemplate_ContainsFunction(t *testing.T) {
	vars := map[string]cty.Value{
		"groups": cty.ListVal([]cty.Value{
			cty.StringVal("spines"),
			cty.StringVal("leaves"),
		}),
	}
	tmpl := "%{ if contains(groups, \"spines\") }match%{ else }no%{ endif }"
	result, err := renderHCLTemplate(tmpl, vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "match" {
		t.Fatalf("expected 'match', got %q", result)
	}
}

func TestRenderHCLTemplate_LiteralString(t *testing.T) {
	result, err := renderHCLTemplate("no interpolation here", map[string]cty.Value{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "no interpolation here" {
		t.Fatalf("expected literal string, got %q", result)
	}
}

func TestRenderHCLTemplate_MultipleVars(t *testing.T) {
	vars := map[string]cty.Value{
		"host": cty.StringVal("10.0.0.1"),
		"port": cty.NumberIntVal(443),
	}
	result, err := renderHCLTemplate("${host}:${port}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "10.0.0.1:443" {
		t.Fatalf("expected '10.0.0.1:443', got %q", result)
	}
}

func TestRenderHCLTemplate_IndexAccess(t *testing.T) {
	vars := map[string]cty.Value{
		"devices": cty.TupleVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"name": cty.StringVal("switch1"),
			}),
			cty.ObjectVal(map[string]cty.Value{
				"name": cty.StringVal("switch2"),
			}),
		}),
	}
	result, err := renderHCLTemplate("${devices[0].name}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if result != "switch1" {
		t.Fatalf("expected 'switch1', got %q", result)
	}
}

func TestRenderHCLTemplate_ErrorInvalidVar(t *testing.T) {
	_, err := renderHCLTemplate("${nonexistent}", map[string]cty.Value{})
	if err == nil {
		t.Fatal("expected error for nonexistent variable")
	}
}

func TestRenderHCLTemplate_NativeToCtyIntegration(t *testing.T) {
	native := map[string]any{
		"GLOBAL": map[string]any{
			"devices": []any{
				map[string]any{"name": "leaf1", "ip": "10.0.0.1"},
				map[string]any{"name": "leaf2", "ip": "10.0.0.2"},
			},
		},
		"hostname": "spine1",
	}

	ctyVars := make(map[string]cty.Value)
	for k, v := range native {
		ctyVal, err := nativeToCty(v)
		if err != nil {
			t.Fatalf("converting %q: %v", k, err)
		}
		ctyVars[k] = ctyVal
	}

	result, err := renderHCLTemplate("${hostname} peers with ${GLOBAL.devices[0].name}", ctyVars)
	if err != nil {
		t.Fatal(err)
	}
	expected := "spine1 peers with leaf1"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}
