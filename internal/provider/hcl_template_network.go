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
	"regexp"
	"strconv"
	"strings"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// deviceTemplateFunctions is the function map of render_device_configs
// templates: templateFunctions plus functions for the device data models
// (nxos, iosxe, iosxr), which do not apply to other models (e.g. Meraki, whose
// VLAN lists are strings in CLI notation). Built once and only read.
var deviceTemplateFunctions = func() map[string]function.Function {
	funcs := make(map[string]function.Function, len(templateFunctions)+2)
	for name, f := range templateFunctions {
		funcs[name] = f
	}
	funcs["parse_vlans"] = parseVlansFunc
	funcs["expand_interfaces"] = expandInterfacesFunc
	return funcs
}()

// listParts returns the comma-separated parts of a value: a number, a string
// or a list of numbers and strings (e.g. [10, "20-29,40"] → "10", "20-29", "40").
// Whitespace around parts is removed, empty parts and null list elements are
// skipped.
func listParts(v cty.Value) ([]string, error) {
	if v.IsNull() {
		return nil, nil
	}
	if !v.IsWhollyKnown() {
		return nil, fmt.Errorf("value is not known")
	}
	ty := v.Type()
	if ty == cty.String || ty == cty.Number {
		return valueParts(v)
	}
	if !ty.IsListType() && !ty.IsTupleType() && !ty.IsSetType() {
		return nil, fmt.Errorf("must be a number, a string or a list of numbers and strings, not %s", ty.FriendlyName())
	}
	var parts []string
	i := 0
	for it := v.ElementIterator(); it.Next(); i++ {
		_, e := it.Element()
		if e.IsNull() {
			continue
		}
		if e.Type() != cty.String && e.Type() != cty.Number {
			return nil, fmt.Errorf("element %d: must be a number or a string, not %s", i, e.Type().FriendlyName())
		}
		p, err := valueParts(e)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		parts = append(parts, p...)
	}
	return parts, nil
}

// valueParts returns the comma-separated parts of a known, non-null string or
// number.
func valueParts(v cty.Value) ([]string, error) {
	s := ""
	if v.Type() == cty.Number {
		bf := v.AsBigFloat()
		n, acc := bf.Int64()
		if !bf.IsInt() || acc != big.Exact {
			return nil, fmt.Errorf("%s is not a whole number", bf.Text('f', -1))
		}
		s = strconv.FormatInt(n, 10)
	} else {
		s = v.AsString()
	}
	var parts []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return parts, nil
}

var (
	vlanIDRe    = regexp.MustCompile(`^\d+$`)
	vlanRangeRe = regexp.MustCompile(`^(\d+)\s*-\s*(\d+)$`)
)

// parseVlansFunc converts VLANs in CLI notation to the `ids` and `ranges`
// object of the device data models, e.g. [10, "20-29,40"] →
// {ids = [10, 40], ranges = [{from = 20, to = 29}]}. Lists without IDs or
// ranges are left out of the result (an empty input returns an empty object),
// null returns null.
var parseVlansFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "vlans", Type: cty.DynamicPseudoType, AllowNull: true, AllowDynamicType: true},
	},
	Type: function.StaticReturnType(cty.DynamicPseudoType),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		if args[0].IsNull() {
			return cty.NullVal(cty.DynamicPseudoType), nil
		}
		parts, err := listParts(args[0])
		if err != nil {
			return cty.NilVal, err
		}
		var ids, ranges []cty.Value
		for _, p := range parts {
			if m := vlanRangeRe.FindStringSubmatch(p); m != nil {
				from, errFrom := strconv.ParseInt(m[1], 10, 64)
				to, errTo := strconv.ParseInt(m[2], 10, 64)
				if errFrom != nil || errTo != nil {
					return cty.NilVal, fmt.Errorf("invalid VLAN range %q: VLAN IDs must be between 1 and 4094", p)
				}
				if err := validateVlanRange(from, to); err != nil {
					return cty.NilVal, fmt.Errorf("invalid VLAN range %q: %w", p, err)
				}
				ranges = append(ranges, cty.ObjectVal(map[string]cty.Value{
					"from": cty.NumberIntVal(from),
					"to":   cty.NumberIntVal(to),
				}))
				continue
			}
			if !vlanIDRe.MatchString(p) {
				return cty.NilVal, fmt.Errorf("invalid VLAN %q: must be an ID or a range (e.g. 10 or \"20-29\")", p)
			}
			id, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return cty.NilVal, fmt.Errorf("invalid VLAN %q: VLAN IDs must be between 1 and 4094", p)
			}
			if err := validateVlanID(id); err != nil {
				return cty.NilVal, fmt.Errorf("invalid VLAN %q: %w", p, err)
			}
			ids = append(ids, cty.NumberIntVal(id))
		}
		result := map[string]cty.Value{}
		if len(ids) > 0 {
			result["ids"] = cty.ListVal(ids)
		}
		if len(ranges) > 0 {
			result["ranges"] = cty.ListVal(ranges)
		}
		return cty.ObjectVal(result), nil
	},
})

var interfaceRangeRe = regexp.MustCompile(`^(.*?)(\d+)\s*-\s*(\d+)$`)

// maxInterfaceRange limits the size of a single interface range, so that a typo
// (e.g. "1/1-10000") fails instead of rendering thousands of interfaces.
const maxInterfaceRange = 1024

// expandInterfacesFunc expands interface ranges in CLI notation to a list of
// interface IDs, e.g. ["1/10", "1/20-23"] → ["1/10", "1/20", "1/21", "1/22", "1/23"].
// The range applies to the last number of an ID (e.g. "1/1/1-4" for breakout
// ports, "10-12" for port-channels). Order is kept, null returns an empty list.
var expandInterfacesFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "interfaces", Type: cty.DynamicPseudoType, AllowNull: true, AllowDynamicType: true},
	},
	Type: function.StaticReturnType(cty.List(cty.String)),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		parts, err := listParts(args[0])
		if err != nil {
			return cty.NilVal, err
		}
		var ids []cty.Value
		for _, p := range parts {
			m := interfaceRangeRe.FindStringSubmatch(p)
			if m == nil {
				if strings.Contains(p, "-") {
					return cty.NilVal, fmt.Errorf("invalid interface range %q: only the last number can be a range (e.g. \"1/20-23\")", p)
				}
				ids = append(ids, cty.StringVal(p))
				continue
			}
			prefix := m[1]
			if prefix != "" && !strings.HasSuffix(prefix, "/") {
				return cty.NilVal, fmt.Errorf("invalid interface range %q: only the last number can be a range (e.g. \"1/20-23\")", p)
			}
			from, errFrom := strconv.ParseInt(m[2], 10, 64)
			to, errTo := strconv.ParseInt(m[3], 10, 64)
			if errFrom != nil || errTo != nil {
				return cty.NilVal, fmt.Errorf("invalid interface range %q", p)
			}
			if from > to {
				return cty.NilVal, fmt.Errorf("invalid interface range %q: start must not be greater than end", p)
			}
			if to-from >= maxInterfaceRange {
				return cty.NilVal, fmt.Errorf("invalid interface range %q: more than %d interfaces", p, maxInterfaceRange)
			}
			for i := from; i <= to; i++ {
				ids = append(ids, cty.StringVal(prefix+strconv.FormatInt(i, 10)))
			}
		}
		if len(ids) == 0 {
			return cty.ListValEmpty(cty.String), nil
		}
		return cty.ListVal(ids), nil
	},
})
