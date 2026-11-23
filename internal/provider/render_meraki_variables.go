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
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Computed variables: string values of template variables (at any level and
// nested within maps and lists) containing a template interpolation (${...})
// or directive (%{...}) are evaluated with the merged variables of the network
// or device before the templates are rendered. All variables are evaluated
// (eagerly), also those not used by a template.
//
// Expressions are marked (merakiExpr) before !env tags are resolved, so that
// only the strings of the model are evaluated, not resolved !env values or
// evaluated results. Every marked expression and every deferred merge
// (merakiDeferredMerge, a map merged with a computed value) is a node; a node
// depends on the nodes its variable references overlap with (e.g. a reference
// to `aps` depends on `aps[2].name`), so that expressions can build on each
// other in any order, and cycles are an error.

// merakiExpr is a variable expression marked for evaluation.
type merakiExpr struct {
	src string
}

// merakiDeferredMerge is a variable whose value is a merge of values of which
// at least one is computed, e.g. a computed map default and a map of the
// network overriding one of its keys. The layers (maps and computed values,
// lowest precedence first) are merged after the computed ones are evaluated,
// with the rules of mergeMerakiVariables. It is never modified, as it may be
// shared between networks and devices.
type merakiDeferredMerge struct {
	layers []any
}

// newMerakiDeferredMerge returns the deferred merge of base and overlay.
func newMerakiDeferredMerge(base, overlay any) *merakiDeferredMerge {
	layers := func(v any) []any {
		if d, ok := v.(*merakiDeferredMerge); ok {
			return d.layers
		}
		return []any{v}
	}
	b, o := layers(base), layers(overlay)
	all := make([]any, 0, len(b)+len(o))
	return &merakiDeferredMerge{layers: append(append(all, b...), o...)}
}

// isMerakiVariableExpression reports whether v is a string containing a
// template interpolation or directive.
func isMerakiVariableExpression(v any) bool {
	s, ok := v.(string)
	return ok && (strings.Contains(s, "${") || strings.Contains(s, "%{"))
}

// isMerakiComputed reports whether v is a computed value: a deferred merge, a
// marked expression or, unless marked is set, a variable expression string.
func isMerakiComputed(v any, marked bool) bool {
	switch v.(type) {
	case *merakiDeferredMerge, *merakiExpr:
		return true
	case string:
		return !marked && isMerakiVariableExpression(v)
	}
	return false
}

// merakiVariablesHaveExpressions reports whether any value within v (not yet
// marked) is computed.
func merakiVariablesHaveExpressions(v any) bool {
	switch val := v.(type) {
	case string:
		return isMerakiVariableExpression(val)
	case *merakiDeferredMerge, *merakiExpr:
		return true
	case *OrderedMap:
		for _, e := range val.Entries() {
			if merakiVariablesHaveExpressions(e.Value) {
				return true
			}
		}
	case map[string]any:
		for _, item := range val {
			if merakiVariablesHaveExpressions(item) {
				return true
			}
		}
	case []any:
		for _, item := range val {
			if merakiVariablesHaveExpressions(item) {
				return true
			}
		}
	}
	return false
}

// markMerakiVariables returns a copy of v (plain maps and lists) with variable
// expression strings replaced by merakiExpr, also within deferred merges, and
// whether there were any.
func markMerakiVariables(v any) (any, bool) {
	switch val := v.(type) {
	case string:
		if isMerakiVariableExpression(val) {
			return &merakiExpr{src: val}, true
		}
		return val, false
	case *merakiExpr:
		return val, true
	case *merakiDeferredMerge:
		layers := make([]any, len(val.layers))
		for i, layer := range val.layers {
			layers[i], _ = markMerakiVariables(layer)
		}
		return &merakiDeferredMerge{layers: layers}, true
	case *OrderedMap:
		return markMerakiVariables(orderedMapToPlainMap(val))
	case map[string]any:
		result := make(map[string]any, len(val))
		found := false
		for k, item := range val {
			m, f := markMerakiVariables(item)
			result[k] = m
			found = found || f
		}
		return result, found
	case []any:
		result := make([]any, len(val))
		found := false
		for i, item := range val {
			m, f := markMerakiVariables(item)
			result[i] = m
			found = found || f
		}
		return result, found
	default:
		return v, false
	}
}

// resolveMerakiVariableTags resolves the !env tags within marked variables
// (see resolveYamlTagsLenient), also within deferred merges; marked
// expressions are kept.
func resolveMerakiVariableTags(v any) any {
	switch val := v.(type) {
	case string:
		return resolveYamlTagsLenient(val)
	case *merakiExpr:
		return val
	case *merakiDeferredMerge:
		layers := make([]any, len(val.layers))
		for i, layer := range val.layers {
			layers[i] = resolveMerakiVariableTags(layer)
		}
		return &merakiDeferredMerge{layers: layers}
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[k] = resolveMerakiVariableTags(item)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = resolveMerakiVariableTags(item)
		}
		return result
	default:
		return v
	}
}

// merakiVariableExpression is a parsed variable expression.
type merakiVariableExpression struct {
	expr hclsyntax.Expression
	// refs are the static prefixes of the variable traversals of the
	// expression (e.g. ["wireless", "guest"] for `wireless.guest.ssid` and
	// ["aps"] for `aps[i].name`); for-expression symbols are not included
	refs [][]any
	// roots are the distinct root variable names of refs
	roots []string
	err   error
}

// merakiExpressionCache memoizes parsed variable expressions by their source,
// as many networks and devices share the same template defaults.
type merakiExpressionCache map[string]*merakiVariableExpression

func (c merakiExpressionCache) parse(src string) *merakiVariableExpression {
	if e, ok := c[src]; ok {
		return e
	}
	e := &merakiVariableExpression{}
	expr, diags := hclsyntax.ParseTemplate([]byte(src), "variable", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		e.err = fmt.Errorf("parsing template: %s", diags.Error())
	} else {
		e.expr = expr
		seen := make(map[string]bool)
		for _, traversal := range expr.Variables() {
			ref := staticTraversalPath(traversal)
			e.refs = append(e.refs, ref)
			e.roots = appendUniqueStrings(e.roots, seen, ref[0].(string))
		}
	}
	c[src] = e
	return e
}

// staticTraversalPath returns the root name and the following attribute names
// and literal indices of traversal, up to the first step that is not static.
func staticTraversalPath(traversal hcl.Traversal) []any {
	path := []any{traversal.RootName()}
	for _, step := range traversal[1:] {
		switch s := step.(type) {
		case hcl.TraverseAttr:
			path = append(path, s.Name)
		case hcl.TraverseIndex:
			key := s.Key
			if !key.IsKnown() || key.IsNull() {
				return path
			}
			switch key.Type() {
			case cty.String:
				path = append(path, key.AsString())
			case cty.Number:
				bf := key.AsBigFloat()
				i, acc := bf.Int64()
				if !bf.IsInt() || acc != big.Exact {
					return path
				}
				path = append(path, int(i))
			default:
				return path
			}
		default:
			return path
		}
	}
	return path
}

// merakiPathsOverlap reports whether one path is a prefix of the other. Keys
// are compared as text, so that e.g. a reference `x["0"]` conservatively
// overlaps the list item `x[0]`.
func merakiPathsOverlap(a, b []any) bool {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if fmt.Sprint(a[i]) != fmt.Sprint(b[i]) {
			return false
		}
	}
	return true
}

// formatMerakiVariablePath formats a variable path for error messages, e.g.
// `wireless.guest.ssid` or `aps[2].name`.
func formatMerakiVariablePath(path []any) string {
	var b strings.Builder
	for i, p := range path {
		switch v := p.(type) {
		case int:
			fmt.Fprintf(&b, "[%d]", v)
		default:
			if i > 0 {
				b.WriteByte('.')
			}
			fmt.Fprint(&b, v)
		}
	}
	return b.String()
}

// evaluateMerakiVariables evaluates the variable expressions within vars
// (plain maps and lists, possibly with deferred merges) and returns the
// evaluated variables and whether vars contained any expressions. If resolve
// is set, !env tags are resolved (leniently) first; expressions are marked
// before, so that resolved values are never evaluated. vars is not modified;
// without expressions and resolve, vars itself is returned.
//
// An expression consisting of a single interpolation keeps the type of its
// value (e.g. a list), like in model templates, and `$${` and `%%{` escape a
// literal `${` and `%{`. Evaluation errors and cycles are returned at once,
// one per failing expression (expressions depending on a failing one are not
// reported).
func evaluateMerakiVariables(vars map[string]any, cache merakiExpressionCache, resolve bool) (map[string]any, bool, error) {
	markedValue, found := markMerakiVariables(vars)
	if !found {
		if resolve {
			return resolveYamlTagsLenient(vars).(map[string]any), false, nil
		}
		return vars, false, nil
	}
	marked := markedValue.(map[string]any)
	if resolve {
		marked = resolveMerakiVariableTags(marked).(map[string]any)
	}
	e := &merakiEvaluator{root: marked, cache: cache}
	e.evaluate(nil)
	if len(e.errs) > 0 {
		return nil, true, errors.Join(e.errs...)
	}
	return marked, true, nil
}

// merakiEvaluator evaluates the marked expressions and deferred merges of a
// variables tree in place.
type merakiEvaluator struct {
	root  map[string]any
	cache merakiExpressionCache
	errs  []error
}

// merakiNode is a marked expression or deferred merge to evaluate.
type merakiNode struct {
	path  []any
	value any // *merakiExpr or *merakiDeferredMerge
	// refs are the variable references the node depends on
	refs [][]any
}

// collectNodes appends the nodes within v (at path) to nodes, in a
// deterministic order (map keys sorted). Deferred merges are not descended
// into: their content is evaluated when the deferred merge is.
func (e *merakiEvaluator) collectNodes(v any, path []any, nodes []*merakiNode) []*merakiNode {
	switch val := v.(type) {
	case *merakiExpr:
		node := &merakiNode{path: append([]any(nil), path...), value: val}
		node.refs = e.cache.parse(val.src).refs
		nodes = append(nodes, node)
	case *merakiDeferredMerge:
		node := &merakiNode{path: append([]any(nil), path...), value: val}
		for _, layer := range val.layers {
			if x, ok := layer.(*merakiExpr); ok {
				node.refs = append(node.refs, e.cache.parse(x.src).refs...)
				continue
			}
			// References of the expressions within a map layer to the
			// variable itself are resolved after the merge
			for _, ref := range e.innerRefs(layer) {
				if len(ref) >= len(node.path) && merakiPathsOverlap(ref, node.path) {
					continue
				}
				node.refs = append(node.refs, ref)
			}
		}
		nodes = append(nodes, node)
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			nodes = e.collectNodes(val[k], append(path, k), nodes)
		}
	case []any:
		for i, item := range val {
			nodes = e.collectNodes(item, append(path, i), nodes)
		}
	}
	return nodes
}

// innerRefs returns the references of all expressions within v, also within
// deferred merges.
func (e *merakiEvaluator) innerRefs(v any) [][]any {
	switch val := v.(type) {
	case *merakiExpr:
		return e.cache.parse(val.src).refs
	case *merakiDeferredMerge:
		var refs [][]any
		for _, layer := range val.layers {
			refs = append(refs, e.innerRefs(layer)...)
		}
		return refs
	case map[string]any:
		var refs [][]any
		for _, item := range val {
			refs = append(refs, e.innerRefs(item)...)
		}
		return refs
	case []any:
		var refs [][]any
		for _, item := range val {
			refs = append(refs, e.innerRefs(item)...)
		}
		return refs
	}
	return nil
}

// dependsOn reports whether node a depends on node b. A reference to a map
// containing a (e.g. `lookup(wireless, key)` within `wireless`) depends on
// the other nodes within the map, but not on a itself, which is null when a
// is evaluated; a reference to a itself or within it is a cycle.
func dependsOn(a, b *merakiNode) bool {
	for _, ref := range a.refs {
		if !merakiPathsOverlap(ref, b.path) {
			continue
		}
		if a == b && len(ref) < len(a.path) {
			continue
		}
		return true
	}
	return false
}

// evaluate evaluates the nodes within the value at path (the whole tree if
// path is empty) in the order of their dependencies.
func (e *merakiEvaluator) evaluate(path []any) {
	var value any = e.root
	if len(path) > 0 {
		value = e.valueAt(path)
	}
	nodes := e.collectNodes(value, path, nil)
	if len(nodes) == 0 {
		return
	}
	byRoot := make(map[string][]int)
	for i, node := range nodes {
		root := fmt.Sprint(node.path[0])
		byRoot[root] = append(byRoot[root], i)
	}
	deps := make([][]int, len(nodes))
	for i, node := range nodes {
		seen := make(map[int]bool)
		for _, ref := range node.refs {
			for _, j := range byRoot[fmt.Sprint(ref[0])] {
				if !seen[j] && dependsOn(node, nodes[j]) {
					seen[j] = true
					deps[i] = append(deps[i], j)
				}
			}
		}
	}

	const (
		pending = iota
		visiting
		done
		failed
	)
	state := make([]int, len(nodes))
	var stack []int
	var visit func(i int) bool
	visit = func(i int) bool {
		switch state[i] {
		case done:
			return true
		case failed:
			return false
		case visiting:
			start := 0
			for k, j := range stack {
				if j == i {
					start = k
					break
				}
			}
			chain := make([]string, 0, len(stack)-start+1)
			for _, j := range stack[start:] {
				chain = append(chain, formatMerakiVariablePath(nodes[j].path))
			}
			chain = append(chain, formatMerakiVariablePath(nodes[i].path))
			e.errs = append(e.errs, fmt.Errorf("variable cycle: %s", strings.Join(chain, " -> ")))
			return false
		}
		state[i] = visiting
		stack = append(stack, i)
		ok := true
		for _, j := range deps[i] {
			if !visit(j) {
				ok = false
			}
		}
		stack = stack[:len(stack)-1]
		if !ok || !e.evaluateNode(nodes[i]) {
			state[i] = failed
			return false
		}
		state[i] = done
		return true
	}
	for i := range nodes {
		visit(i)
	}
}

// evaluateNode evaluates a node and stores its value in the tree. It returns
// false (and records the error) if the evaluation fails.
func (e *merakiEvaluator) evaluateNode(node *merakiNode) bool {
	switch val := node.value.(type) {
	case *merakiExpr:
		value, err := e.evaluateExpression(val, node.path)
		if err != nil {
			e.errs = append(e.errs, err)
			return false
		}
		e.setAt(node.path, value)
		return true
	case *merakiDeferredMerge:
		var acc any
		for _, layer := range val.layers {
			if x, ok := layer.(*merakiExpr); ok {
				value, err := e.evaluateExpression(x, node.path)
				if err != nil {
					e.errs = append(e.errs, err)
					return false
				}
				layer = value
			}
			merged, err := mergeMerakiLayer(acc, layer, node.path)
			if err != nil {
				e.errs = append(e.errs, err)
				return false
			}
			acc = merged
		}
		e.setAt(node.path, acc)
		// Evaluate the expressions of the map layers that the merge kept
		errs := len(e.errs)
		e.evaluate(node.path)
		return len(e.errs) == errs
	}
	return true
}

// mergeMerakiLayer merges the next layer of a deferred merge (an evaluated
// value or a map with marked expressions) onto the merged lower layers, with
// the rules of mergeMerakiVariables.
func mergeMerakiLayer(acc, layer any, path []any) (any, error) {
	if acc == nil || layer == nil {
		return layer, nil
	}
	accMap, accIsMap := normalizeToMapStringAny(acc)
	layerMap, layerIsMap := normalizeToMapStringAny(layer)
	if accIsMap && layerIsMap {
		merged, err := mergeMerakiVariablesMode(accMap, layerMap, formatMerakiVariablePath(path), true)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", formatMerakiVariablePath(path), err)
		}
		return merged, nil
	}
	if accIsMap != layerIsMap {
		return nil, fmt.Errorf("variable %q: conflicting types of evaluated values (%s vs %s)",
			formatMerakiVariablePath(path), shapeLabel(acc), shapeLabel(layer))
	}
	return layer, nil
}

// evaluateExpression evaluates a marked expression of the variable at path
// with the current values of the variables it refers to. Expressions and
// deferred merges not evaluated yet are their source text and null in the
// variables, and so is the variable at path itself.
func (e *merakiEvaluator) evaluateExpression(x *merakiExpr, path []any) (any, error) {
	name := formatMerakiVariablePath(path)
	parsed := e.cache.parse(x.src)
	if parsed.err != nil {
		return nil, fmt.Errorf("variable %q: %w", name, parsed.err)
	}
	ctxVars := make(map[string]cty.Value, len(parsed.roots))
	for _, root := range parsed.roots {
		value, ok := e.root[root]
		if !ok {
			continue
		}
		placeholder := merakiPlaceholderValue(value, []any{root}, path)
		ctyValue, err := nativeToCty(placeholder)
		if err != nil {
			return nil, fmt.Errorf("variable %q: converting variable %q: %w", name, root, err)
		}
		ctxVars[root] = ctyValue
	}
	value, diags := parsed.expr.Value(&hcl.EvalContext{Variables: ctxVars, Functions: templateFunctions})
	if diags.HasErrors() {
		return nil, fmt.Errorf("variable %q: evaluating template: %s", name, diags.Error())
	}
	return ctyToNative(value), nil
}

// merakiPlaceholderValue returns a plain copy of v (at path) for evaluation,
// with marked expressions replaced by their source text, deferred merges by
// null and the value at self by null.
func merakiPlaceholderValue(v any, path, self []any) any {
	if len(path) == len(self) && merakiPathsOverlap(path, self) {
		return nil
	}
	switch val := v.(type) {
	case *merakiExpr:
		return val.src
	case *merakiDeferredMerge:
		return nil
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[k] = merakiPlaceholderValue(item, append(path[:len(path):len(path)], k), self)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = merakiPlaceholderValue(item, append(path[:len(path):len(path)], i), self)
		}
		return result
	default:
		return v
	}
}

// valueAt returns the value at path within the tree.
func (e *merakiEvaluator) valueAt(path []any) any {
	var value any = e.root
	for _, p := range path {
		switch c := value.(type) {
		case map[string]any:
			value = c[p.(string)]
		case []any:
			value = c[p.(int)]
		default:
			return nil
		}
	}
	return value
}

// setAt replaces the value at path within the tree.
func (e *merakiEvaluator) setAt(path []any, value any) {
	if len(path) == 0 {
		return
	}
	switch c := e.valueAt(path[:len(path)-1]).(type) {
	case map[string]any:
		c[path[len(path)-1].(string)] = value
	case []any:
		c[path[len(path)-1].(int)] = value
	}
}

// merakiValuesAt returns the keys of structure with their values in values:
// maps in both are narrowed recursively to the keys of structure, every other
// value of structure is replaced by the value in values as a whole. It gives
// the evaluated values of the rendered variables of a device, whose keys are
// determined from the unevaluated variables.
func merakiValuesAt(structure, values map[string]any) map[string]any {
	result := make(map[string]any, len(structure))
	for k, s := range structure {
		v, ok := values[k]
		if !ok {
			result[k] = s
			continue
		}
		if sm, ok := normalizeToMapStringAny(s); ok {
			if vm, ok := normalizeToMapStringAny(v); ok {
				result[k] = merakiValuesAt(sm, vm)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// merakiVariablesChanged returns the parts of target that differ from base:
// keys missing in base and, for maps in both, recursively the changed keys;
// other values are compared as a whole. null values of target are skipped, as
// they are removed from the rendered model. It gives the variables of the
// network whose computed value differs for a device, e.g. because they refer
// to a variable the device overrides.
func merakiVariablesChanged(base, target map[string]any) map[string]any {
	result := make(map[string]any)
	for k, t := range target {
		if t == nil {
			continue
		}
		b, ok := base[k]
		if !ok {
			result[k] = t
			continue
		}
		if bm, ok := normalizeToMapStringAny(b); ok {
			if tm, ok := normalizeToMapStringAny(t); ok {
				if changed := merakiVariablesChanged(bm, tm); len(changed) > 0 {
					result[k] = changed
				}
				continue
			}
		}
		if !reflect.DeepEqual(orderedMapToPlainMap(b), orderedMapToPlainMap(t)) {
			result[k] = t
		}
	}
	return result
}
