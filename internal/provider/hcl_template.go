package provider

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// renderHCLTemplate parses and evaluates an HCL template string with the given variables.
// This uses the same HCL library that Terraform's templatestring() uses internally.
func renderHCLTemplate(tmpl string, vars map[string]cty.Value) (string, error) {
	expr, diags := hclsyntax.ParseTemplate([]byte(tmpl), "template", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return "", fmt.Errorf("parsing template: %s", diags.Error())
	}

	ctx := &hcl.EvalContext{
		Variables: vars,
		Functions: hclTemplateFunctions(),
	}

	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return "", fmt.Errorf("evaluating template: %s", diags.Error())
	}

	if val.Type() != cty.String {
		return "", fmt.Errorf("template result is %s, not string", val.Type().FriendlyName())
	}

	return val.AsString(), nil
}

// renderHCLTemplateValue evaluates an HCL template string and returns a native Go value.
// Unlike renderHCLTemplate, this preserves the original cty type when possible:
// - Pure expressions like "${count}" may return int, bool, etc.
// - Interpolated strings like "prefix-${name}" always return string.
func renderHCLTemplateValue(tmpl string, vars map[string]cty.Value) (any, error) {
	expr, diags := hclsyntax.ParseTemplate([]byte(tmpl), "template", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing template: %s", diags.Error())
	}

	ctx := &hcl.EvalContext{
		Variables: vars,
		Functions: hclTemplateFunctions(),
	}

	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("evaluating template: %s", diags.Error())
	}

	return ctyToNative(val), nil
}

// ctyToNative converts a cty.Value to a native Go value.
func ctyToNative(val cty.Value) any {
	if val.IsNull() {
		return nil
	}
	ty := val.Type()
	switch {
	case ty == cty.String:
		return val.AsString()
	case ty == cty.Number:
		bf := val.AsBigFloat()
		if bf.IsInt() {
			if i, acc := bf.Int64(); acc == big.Exact {
				return i
			}
		}
		f, _ := bf.Float64()
		return f
	case ty == cty.Bool:
		return val.True()
	default:
		// For complex types, fall back to string representation
		return val.AsString()
	}
}

// nativeToCty converts a Go native value (from convertDynamicToNative) to a cty.Value.
func nativeToCty(val any) (cty.Value, error) {
	if val == nil {
		return cty.NullVal(cty.DynamicPseudoType), nil
	}

	switch v := val.(type) {
	case string:
		return cty.StringVal(v), nil
	case bool:
		return cty.BoolVal(v), nil
	case int:
		return cty.NumberIntVal(int64(v)), nil
	case int64:
		return cty.NumberIntVal(v), nil
	case float64:
		return cty.NumberFloatVal(v), nil
	case *big.Float:
		f, _ := v.Float64()
		return cty.NumberFloatVal(f), nil
	case map[string]any:
		if len(v) == 0 {
			return cty.EmptyObjectVal, nil
		}
		attrs := make(map[string]cty.Value, len(v))
		for k, val := range v {
			ctyVal, err := nativeToCty(val)
			if err != nil {
				return cty.NilVal, fmt.Errorf("key %q: %w", k, err)
			}
			attrs[k] = ctyVal
		}
		return cty.ObjectVal(attrs), nil
	case []any:
		if len(v) == 0 {
			return cty.EmptyTupleVal, nil
		}
		elems := make([]cty.Value, len(v))
		for i, val := range v {
			ctyVal, err := nativeToCty(val)
			if err != nil {
				return cty.NilVal, fmt.Errorf("index %d: %w", i, err)
			}
			elems[i] = ctyVal
		}
		return cty.TupleVal(elems), nil
	default:
		return cty.NilVal, fmt.Errorf("unsupported type %T", val)
	}
}

// hclTemplateFunctions returns the function map for HCL template evaluation.
// Includes go-cty stdlib functions, HCL try/can, and manually implemented extras.
func hclTemplateFunctions() map[string]function.Function {
	return map[string]function.Function{
		// go-cty stdlib: numbers
		"abs":      stdlib.AbsoluteFunc,
		"ceil":     stdlib.CeilFunc,
		"floor":    stdlib.FloorFunc,
		"log":      stdlib.LogFunc,
		"max":      stdlib.MaxFunc,
		"min":      stdlib.MinFunc,
		"parseint": stdlib.ParseIntFunc,
		"pow":      stdlib.PowFunc,
		"signum":   stdlib.SignumFunc,

		// go-cty stdlib: strings
		"chomp":        stdlib.ChompFunc,
		"format":       stdlib.FormatFunc,
		"formatlist":   stdlib.FormatListFunc,
		"indent":       stdlib.IndentFunc,
		"join":         stdlib.JoinFunc,
		"lower":        stdlib.LowerFunc,
		"regex":        stdlib.RegexFunc,
		"regexall":     stdlib.RegexAllFunc,
		"regexreplace": stdlib.RegexReplaceFunc,
		"replace":      stdlib.ReplaceFunc,
		"sort":         stdlib.SortFunc,
		"split":        stdlib.SplitFunc,
		"strlen":       stdlib.StrlenFunc,
		"substr":       stdlib.SubstrFunc,
		"title":        stdlib.TitleFunc,
		"trim":         stdlib.TrimFunc,
		"trimprefix":   stdlib.TrimPrefixFunc,
		"trimsuffix":   stdlib.TrimSuffixFunc,
		"trimspace":    stdlib.TrimSpaceFunc,
		"upper":        stdlib.UpperFunc,

		// go-cty stdlib: collections
		"chunklist":  stdlib.ChunklistFunc,
		"coalesce":   stdlib.CoalesceFunc,
		"compact":    stdlib.CompactFunc,
		"concat":     stdlib.ConcatFunc,
		"contains":   stdlib.ContainsFunc,
		"distinct":   stdlib.DistinctFunc,
		"element":    stdlib.ElementFunc,
		"flatten":    stdlib.FlattenFunc,
		"index":      stdlib.IndexFunc,
		"keys":       stdlib.KeysFunc,
		"length":     stdlib.LengthFunc,
		"lookup":     stdlib.LookupFunc,
		"merge":      stdlib.MergeFunc,
		"range":      stdlib.RangeFunc,
		"reverse":    stdlib.ReverseListFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setsubtract":     stdlib.SetSubtractFunc,
		"setunion":        stdlib.SetUnionFunc,
		"slice":           stdlib.SliceFunc,
		"values":          stdlib.ValuesFunc,
		"zipmap":          stdlib.ZipmapFunc,

		// go-cty stdlib: encoding
		"csvdecode":  stdlib.CSVDecodeFunc,
		"jsondecode": stdlib.JSONDecodeFunc,
		"jsonencode": stdlib.JSONEncodeFunc,

		// go-cty stdlib: date/time
		"formatdate": stdlib.FormatDateFunc,
		"timeadd":    stdlib.TimeAddFunc,

		// go-cty stdlib: general
		"coalescelist": stdlib.CoalesceListFunc,

		// HCL ext: try/can
		"try": tryfunc.TryFunc,
		"can": tryfunc.CanFunc,

		// Manually implemented
		"tostring":    toStringFunc,
		"tonumber":    toNumberFunc,
		"tobool":      toBoolFunc,
		"startswith":  startsWithFunc,
		"endswith":    endsWithFunc,
		"strcontains": strContainsFunc,
		"alltrue":     allTrueFunc,
		"anytrue":     anyTrueFunc,
		"one":         oneFunc,
		"sum":         sumFunc,
		"base64encode": base64EncodeFunc,
		"base64decode": base64DecodeFunc,
	}
}

// Manually implemented functions

var toStringFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "v", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		v := args[0]
		if v.IsNull() {
			return cty.NullVal(cty.String), nil
		}
		if v.Type() == cty.String {
			return v, nil
		}
		if v.Type() == cty.Number {
			bf := v.AsBigFloat()
			if bf.IsInt() {
				i, _ := bf.Int64()
				return cty.StringVal(fmt.Sprintf("%d", i)), nil
			}
			f, _ := bf.Float64()
			return cty.StringVal(fmt.Sprintf("%g", f)), nil
		}
		if v.Type() == cty.Bool {
			if v.True() {
				return cty.StringVal("true"), nil
			}
			return cty.StringVal("false"), nil
		}
		return cty.NilVal, fmt.Errorf("cannot convert %s to string", v.Type().FriendlyName())
	},
})

var toNumberFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "v", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.Number),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		v := args[0]
		if v.IsNull() {
			return cty.NullVal(cty.Number), nil
		}
		if v.Type() == cty.Number {
			return v, nil
		}
		if v.Type() == cty.String {
			s := v.AsString()
			bf, _, err := big.ParseFloat(s, 10, 512, big.ToNearestEven)
			if err != nil {
				return cty.NilVal, fmt.Errorf("cannot convert %q to number: %s", s, err)
			}
			return cty.NumberVal(bf), nil
		}
		return cty.NilVal, fmt.Errorf("cannot convert %s to number", v.Type().FriendlyName())
	},
})

var toBoolFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "v", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		v := args[0]
		if v.IsNull() {
			return cty.NullVal(cty.Bool), nil
		}
		if v.Type() == cty.Bool {
			return v, nil
		}
		if v.Type() == cty.String {
			switch v.AsString() {
			case "true":
				return cty.True, nil
			case "false":
				return cty.False, nil
			default:
				return cty.NilVal, fmt.Errorf("cannot convert %q to bool; must be \"true\" or \"false\"", v.AsString())
			}
		}
		return cty.NilVal, fmt.Errorf("cannot convert %s to bool", v.Type().FriendlyName())
	},
})

var startsWithFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
		{Name: "prefix", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		str := args[0].AsString()
		prefix := args[1].AsString()
		return cty.BoolVal(strings.HasPrefix(str, prefix)), nil
	},
})

var endsWithFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
		{Name: "suffix", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		str := args[0].AsString()
		suffix := args[1].AsString()
		return cty.BoolVal(strings.HasSuffix(str, suffix)), nil
	},
})

var strContainsFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
		{Name: "substr", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		str := args[0].AsString()
		substr := args[1].AsString()
		return cty.BoolVal(strings.Contains(str, substr)), nil
	},
})

var allTrueFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "list", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		coll := args[0]
		if !coll.CanIterateElements() {
			return cty.NilVal, fmt.Errorf("argument must be a list or set")
		}
		for it := coll.ElementIterator(); it.Next(); {
			_, v := it.Element()
			if v.IsNull() || !v.IsKnown() {
				return cty.False, nil
			}
			if v.Type() == cty.Bool && v.False() {
				return cty.False, nil
			}
		}
		return cty.True, nil
	},
})

var anyTrueFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "list", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		coll := args[0]
		if !coll.CanIterateElements() {
			return cty.NilVal, fmt.Errorf("argument must be a list or set")
		}
		for it := coll.ElementIterator(); it.Next(); {
			_, v := it.Element()
			if !v.IsNull() && v.IsKnown() && v.Type() == cty.Bool && v.True() {
				return cty.True, nil
			}
		}
		return cty.False, nil
	},
})

var oneFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "list", Type: cty.DynamicPseudoType},
	},
	Type: func(args []cty.Value) (cty.Type, error) {
		return cty.DynamicPseudoType, nil
	},
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		coll := args[0]
		if !coll.CanIterateElements() {
			return cty.NilVal, fmt.Errorf("argument must be a list or set")
		}
		if coll.LengthInt() == 0 {
			return cty.NullVal(cty.DynamicPseudoType), nil
		}
		if coll.LengthInt() != 1 {
			return cty.NilVal, fmt.Errorf("must be a list or set with either zero or one elements")
		}
		it := coll.ElementIterator()
		it.Next()
		_, v := it.Element()
		return v, nil
	},
})

var sumFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "list", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.Number),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		coll := args[0]
		if !coll.CanIterateElements() {
			return cty.NilVal, fmt.Errorf("argument must be a list or set of numbers")
		}
		sum := new(big.Float)
		for it := coll.ElementIterator(); it.Next(); {
			_, v := it.Element()
			if v.Type() != cty.Number {
				return cty.NilVal, fmt.Errorf("argument must be a list or set of numbers")
			}
			sum.Add(sum, v.AsBigFloat())
		}
		return cty.NumberVal(sum), nil
	},
})

var base64EncodeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.StringVal(base64.StdEncoding.EncodeToString([]byte(args[0].AsString()))), nil
	},
})

var base64DecodeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "str", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		decoded, err := base64.StdEncoding.DecodeString(args[0].AsString())
		if err != nil {
			return cty.NilVal, fmt.Errorf("failed to decode base64: %s", err)
		}
		return cty.StringVal(string(decoded)), nil
	},
})
