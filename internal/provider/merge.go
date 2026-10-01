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
	"sort"
)

// mapGet retrieves a value from either *OrderedMap or map[string]any.
func mapGet(m any, key string) (any, bool) {
	switch v := m.(type) {
	case *OrderedMap:
		return v.Get(key)
	case map[string]any:
		val, ok := v[key]
		return val, ok
	}
	return nil, false
}

// mapSet sets a value in either *OrderedMap or map[string]any.
func mapSet(m any, key string, value any) {
	switch v := m.(type) {
	case *OrderedMap:
		v.Set(key, value)
	case map[string]any:
		v[key] = value
	}
}

// mapDelete removes a key from either *OrderedMap or map[string]any.
func mapDelete(m any, key string) {
	switch v := m.(type) {
	case *OrderedMap:
		v.Delete(key)
	case map[string]any:
		delete(v, key)
	}
}

// mapForEach iterates over key-value pairs in either type.
// The callback receives each key and value. Iteration order is preserved for *OrderedMap.
func mapForEach(m any, fn func(key string, value any)) {
	switch v := m.(type) {
	case *OrderedMap:
		for _, e := range v.Entries() {
			fn(e.Key, e.Value)
		}
	case map[string]any:
		for k, val := range v {
			fn(k, val)
		}
	}
}

// mapLen returns the number of entries.
func mapLen(m any) int {
	switch v := m.(type) {
	case *OrderedMap:
		return v.Len()
	case map[string]any:
		return len(v)
	}
	return 0
}

// MergeMaps merges src into dst (both can be *OrderedMap or map[string]any).
// For *OrderedMap: existing keys update in-place (first-doc-wins ordering), new keys append.
// For map[string]any: standard unordered merge.
//
// Scalar values may differ in value or type between src and dst — src always wins.
// A map or list value conflicting in shape with the existing value at the same key
// (e.g. a map in one document and a list in another) returns an error, since that
// combination is never a valid merge.
func MergeMaps(src, dst any, deduplicate bool) (any, error) {
	return mergeMaps(src, dst, deduplicate, "")
}

// conflictPath builds a dotted key path used in shape-conflict error messages.
func conflictPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// shapeLabel returns a human-readable label for a value's merge-relevant shape.
func shapeLabel(v any) string {
	if _, ok := asMap(v); ok {
		return "map"
	}
	if _, ok := v.([]any); ok {
		return "list"
	}
	return "scalar"
}

func mergeMaps(src, dst any, deduplicate bool, path string) (any, error) {
	var mergeErr error
	mapForEach(src, func(key string, sValue any) {
		if mergeErr != nil {
			return
		}
		if sValue == nil {
			return
		}
		dValue, exists := mapGet(dst, key)
		if !exists || dValue == nil {
			mapSet(dst, key, sValue)
			return
		}

		childPath := conflictPath(path, key)

		srcMap, srcIsMap := asMap(sValue)
		dstMap, dstIsMap := asMap(dValue)
		if srcIsMap && dstIsMap {
			merged, err := mergeMaps(srcMap, dstMap, deduplicate, childPath)
			if err != nil {
				mergeErr = err
				return
			}
			mapSet(dst, key, merged)
			return
		}

		if sv, ok := sValue.([]any); ok {
			if dv, ok := dValue.([]any); ok {
				if deduplicate {
					if len(sv) == 0 || len(dv) == 0 {
						mapSet(dst, key, append(dv, sv...))
					} else if hasDuplicatesInList(sv) || hasDuplicatesInList(dv) {
						mapSet(dst, key, append(dv, sv...))
					} else {
						merged := dv
						if err := mergeListItemsIndexed(sv, &merged, deduplicate, childPath); err != nil {
							mergeErr = err
							return
						}
						mapSet(dst, key, merged)
					}
				} else {
					mapSet(dst, key, append(dv, sv...))
				}
				return
			}
		}

		if isPrimitive(sValue) && isPrimitive(dValue) {
			mapSet(dst, key, sValue)
			return
		}

		mergeErr = fmt.Errorf("conflicting types for attribute %q: %s vs %s", childPath, shapeLabel(dValue), shapeLabel(sValue))
	})
	return dst, mergeErr
}

// asMap checks if a value is a map type (*OrderedMap or map[string]any) and returns it.
func asMap(v any) (any, bool) {
	switch v.(type) {
	case *OrderedMap:
		return v, true
	case map[string]any:
		return v, true
	}
	return nil, false
}

// itemsWouldMerge checks if two map items would merge based on primitive field matching.
// Works with both *OrderedMap and map[string]any.
func itemsWouldMerge(item1, item2 any) bool {
	comparison := false
	mismatch := false

	mapForEach(item1, func(k string, v1 any) {
		if mismatch {
			return
		}
		if !isPrimitive(v1) {
			return
		}
		v2, ok := mapGet(item2, k)
		if !ok || !isPrimitive(v2) {
			return
		}
		comparison = true
		if v1 != v2 {
			mismatch = true
		}
	})
	if mismatch {
		return false
	}

	return comparison
}

// kvPair represents a key-value pair used as an inverted index key
type kvPair struct {
	key   string
	value any
}

// isPrimitive returns true if the value is not a map or slice
func isPrimitive(v any) bool {
	switch v.(type) {
	case map[string]any, *OrderedMap, []any:
		return false
	default:
		return true
	}
}

// extractPrimitives returns only primitive key-value pairs from a map-like value.
// The result is always map[string]any (used for inverted index lookups only).
func extractPrimitives(m any) map[string]any {
	size := mapLen(m)
	result := make(map[string]any, size)
	mapForEach(m, func(k string, v any) {
		if isPrimitive(v) {
			result[k] = v
		}
	})
	return result
}

// buildInvertedIndex builds a mapping from (key, value) pairs to item indices
func buildInvertedIndex(primsList []map[string]any) map[kvPair][]int {
	index := make(map[kvPair][]int)
	for i, prims := range primsList {
		for k, v := range prims {
			pair := kvPair{key: k, value: v}
			index[pair] = append(index[pair], i)
		}
	}
	return index
}

// hasDuplicatesInList checks if a list contains duplicate dict items using an inverted index
func hasDuplicatesInList(items []any) bool {
	// Only check dict items for duplicates, precompute primitives
	var indices []int
	var primsList []map[string]any
	for i, item := range items {
		if _, ok := asMap(item); ok {
			indices = append(indices, i)
			primsList = append(primsList, extractPrimitives(item))
		}
	}

	if len(indices) < 2 {
		return false
	}

	// Build inverted index
	index := buildInvertedIndex(primsList)

	// Check candidate pairs from buckets with 2+ entries
	type intPair [2]int
	checked := make(map[intPair]bool)
	for _, bucket := range index {
		if len(bucket) < 2 {
			continue
		}
		for bi := 0; bi < len(bucket); bi++ {
			for bj := bi + 1; bj < len(bucket); bj++ {
				pair := intPair{bucket[bi], bucket[bj]}
				if checked[pair] {
					continue
				}
				checked[pair] = true
				i, j := pair[0], pair[1]
				// Intersect primitive key sets, verify all shared keys match
				match := false
				allMatch := true
				for k, v1 := range primsList[i] {
					if v2, ok := primsList[j][k]; ok {
						match = true
						if v1 != v2 {
							allMatch = false
							break
						}
					}
				}
				if match && allMatch {
					return true
				}
			}
		}
	}

	return false
}

// mergeListItemsIndexed merges source items into destination using an inverted index
func mergeListItemsIndexed(sourceItems []any, dst *[]any, deduplicate bool, path string) error {
	// Build inverted index over destination's dict items
	destPrimitives := make([]map[string]any, len(*dst))
	for i, item := range *dst {
		if _, ok := asMap(item); ok {
			destPrimitives[i] = extractPrimitives(item)
		}
	}

	index := make(map[kvPair][]int)
	for i, prims := range destPrimitives {
		if prims == nil {
			continue
		}
		for k, v := range prims {
			pair := kvPair{key: k, value: v}
			index[pair] = append(index[pair], i)
		}
	}

	for _, srcItem := range sourceItems {
		srcMapVal, isMap := asMap(srcItem)
		if !isMap {
			*dst = append(*dst, srcItem)
			continue
		}

		srcPrims := extractPrimitives(srcMapVal)
		if len(srcPrims) == 0 {
			*dst = append(*dst, srcItem)
			continue
		}

		// Collect candidate dest indices
		candidateSet := make(map[int]bool)
		for k, v := range srcPrims {
			pair := kvPair{key: k, value: v}
			if indices, exists := index[pair]; exists {
				for _, idx := range indices {
					candidateSet[idx] = true
				}
			}
		}

		// Check candidates in destination order (first-match semantics)
		candidates := make([]int, 0, len(candidateSet))
		for idx := range candidateSet {
			candidates = append(candidates, idx)
		}
		sort.Ints(candidates)

		matched := false
		for _, ci := range candidates {
			dp := destPrimitives[ci]
			if dp == nil {
				continue
			}
			// Intersect primitive key sets, verify all shared keys match
			hasShared := false
			allMatch := true
			for k, sv := range srcPrims {
				if dv, ok := dp[k]; ok {
					hasShared = true
					if sv != dv {
						allMatch = false
						break
					}
				}
			}
			if hasShared && allMatch {
				if _, err := mergeMaps(srcMapVal, (*dst)[ci], deduplicate, path); err != nil {
					return err
				}
				// Update primitives cache after merge
				destPrimitives[ci] = extractPrimitives((*dst)[ci])
				matched = true
				break
			}
		}

		if !matched {
			// Append and update index so later source items can match
			newIdx := len(*dst)
			*dst = append(*dst, srcItem)
			destPrimitives = append(destPrimitives, srcPrims)
			for k, v := range srcPrims {
				pair := kvPair{key: k, value: v}
				index[pair] = append(index[pair], newIdx)
			}
		}
	}
	return nil
}

func MergeListItem(src any, dst *[]any, deduplicate bool) error {
	if srcMap, isMap := asMap(src); isMap {
		for i, item := range *dst {
			if dstMap, ok := asMap(item); ok {
				if itemsWouldMerge(srcMap, dstMap) {
					_, err := mergeMaps(srcMap, (*dst)[i], deduplicate, "")
					return err
				}
			}
		}
	}
	*dst = append(*dst, src)
	return nil
}
