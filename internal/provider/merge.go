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
	"reflect"
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
	return mergeMaps(src, dst, deduplicate, "", nil)
}

// listMerger merges a sequence of documents into one destination, caching the
// duplicate status of destination lists between documents, so that merging a
// document only checks the list items it changed instead of rescanning whole lists.
//
// The cache is only valid if the documents do not share maps or lists with each
// other or with the destination, which holds for values produced by yamlDecode
// (aliases are deep-copied) and convertDynamicToNative.
type listMerger struct {
	dst         any
	deduplicate bool
	cache       *dupCache
}

// newListMerger creates a listMerger merging into dst. The cache is only used
// with deduplicate, as list items are simply appended otherwise.
func newListMerger(dst any, deduplicate bool) *listMerger {
	m := &listMerger{dst: dst, deduplicate: deduplicate}
	if deduplicate {
		m.cache = newDupCache()
	}
	return m
}

// Merge merges src into the destination, see MergeMaps.
func (m *listMerger) Merge(src any) error {
	_, err := mergeMaps(src, m.dst, m.deduplicate, "", m.cache)
	if err != nil && m.cache != nil {
		// A failed merge can leave destination lists partially changed without
		// updating their cache entries, so start over with an empty cache
		m.cache = newDupCache()
	}
	return err
}

// dupCacheKey identifies a list by the map containing it and its key, as Go
// slices have no stable identity (append may reallocate them).
type dupCacheKey struct {
	container uintptr
	key       string
}

type dupCacheEntry struct {
	// container keeps the map alive, so that its address cannot be reused
	container any
	// n and first identify the cached slice, to detect lists replaced without
	// going through the cache
	n     int
	first *any
	dup   bool
}

// dupCache caches the duplicate status of destination lists during a merge run.
// A nil *dupCache is valid and disables caching.
type dupCache struct {
	entries map[dupCacheKey]dupCacheEntry
}

func newDupCache() *dupCache {
	return &dupCache{entries: make(map[dupCacheKey]dupCacheEntry)}
}

func newDupCacheKey(container any, key string) dupCacheKey {
	return dupCacheKey{container: reflect.ValueOf(container).Pointer(), key: key}
}

// hasDuplicates returns the cached duplicate status of the list stored at key
// in container, computing and caching it if needed.
func (c *dupCache) hasDuplicates(container any, key string, items []any) bool {
	if c == nil {
		return hasDuplicatesInList(items)
	}
	if len(items) == 0 {
		return false
	}
	if e, ok := c.entries[newDupCacheKey(container, key)]; ok && e.n == len(items) && e.first == &items[0] {
		return e.dup
	}
	dup := hasDuplicatesInList(items)
	c.set(container, key, items, dup)
	return dup
}

func (c *dupCache) set(container any, key string, items []any, dup bool) {
	if c == nil {
		return
	}
	k := newDupCacheKey(container, key)
	if len(items) == 0 {
		delete(c.entries, k)
		return
	}
	c.entries[k] = dupCacheEntry{container: container, n: len(items), first: &items[0], dup: dup}
}

func (c *dupCache) invalidate(container any, key string) {
	if c == nil {
		return
	}
	delete(c.entries, newDupCacheKey(container, key))
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

func mergeMaps(src, dst any, deduplicate bool, path string, cache *dupCache) (any, error) {
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
			cache.invalidate(dst, key)
			return
		}

		childPath := conflictPath(path, key)

		srcMap, srcIsMap := asMap(sValue)
		dstMap, dstIsMap := asMap(dValue)
		if srcIsMap && dstIsMap {
			merged, err := mergeMaps(srcMap, dstMap, deduplicate, childPath, cache)
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
						if len(sv) > 0 {
							cache.invalidate(dst, key)
						}
					} else if hasDuplicatesInList(sv) || cache.hasDuplicates(dst, key, dv) {
						// Concatenate without merging to preserve all duplicates
						merged := append(dv, sv...)
						mapSet(dst, key, merged)
						cache.set(dst, key, merged, true)
					} else {
						merged := dv
						mergedIndices, primsList, err := mergeListItemsIndexed(sv, &merged, deduplicate, childPath, cache)
						if err != nil {
							mergeErr = err
							return
						}
						mapSet(dst, key, merged)
						// Only merged items can have become duplicates of others
						if cache != nil {
							cache.set(dst, key, merged, hasDuplicatesInvolving(primsList, mergedIndices))
						}
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

// primitivesMatch reports whether two items' primitive values share at least one
// key and all shared keys have equal values, i.e. the items would merge.
func primitivesMatch(a, b map[string]any) bool {
	shared := false
	for k, v1 := range a {
		if v2, ok := b[k]; ok {
			shared = true
			if v1 != v2 {
				return false
			}
		}
	}
	return shared
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
				if primitivesMatch(primsList[pair[0]], primsList[pair[1]]) {
					return true
				}
			}
		}
	}

	return false
}

// hasDuplicatesInvolving checks if any dict item at the given indices matches another
// dict item, using the same matching logic as hasDuplicatesInList. If a list without
// duplicates had only the items at these indices modified, only such pairs can have
// become duplicates. primsList holds the primitive values of each list item (nil for
// non-dict items), as returned by mergeListItemsIndexed.
func hasDuplicatesInvolving(primsList []map[string]any, indices []int) bool {
	if len(indices) == 0 {
		return false
	}

	// Only index the (key, value) pairs of the modified items, as any matching
	// item must share at least one of them
	wanted := make(map[kvPair]bool)
	for _, i := range indices {
		for k, v := range primsList[i] {
			wanted[kvPair{key: k, value: v}] = true
		}
	}
	index := make(map[kvPair][]int, len(wanted))
	for j, prims := range primsList {
		for k, v := range prims {
			pair := kvPair{key: k, value: v}
			if wanted[pair] {
				index[pair] = append(index[pair], j)
			}
		}
	}

	checked := make(map[int]bool, len(indices))
	for _, i := range indices {
		if checked[i] {
			continue
		}
		checked[i] = true
		prims := primsList[i]
		candidates := make(map[int]bool)
		for k, v := range prims {
			for _, j := range index[kvPair{key: k, value: v}] {
				if j != i && !candidates[j] {
					candidates[j] = true
					if primitivesMatch(prims, primsList[j]) {
						return true
					}
				}
			}
		}
	}

	return false
}

// mergeListItemsIndexed merges source items into destination using an inverted index.
// It returns the indices of destination items that source items were merged into and
// the primitive values of all destination items after merging (nil for non-dict items).
func mergeListItemsIndexed(sourceItems []any, dst *[]any, deduplicate bool, path string, cache *dupCache) ([]int, []map[string]any, error) {
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

	var mergedIndices []int
	for _, srcItem := range sourceItems {
		// Appended items also need an entry in destPrimitives, as candidate
		// indices refer to positions in dst
		srcMapVal, isMap := asMap(srcItem)
		if !isMap {
			*dst = append(*dst, srcItem)
			destPrimitives = append(destPrimitives, nil)
			continue
		}

		srcPrims := extractPrimitives(srcMapVal)
		if len(srcPrims) == 0 {
			*dst = append(*dst, srcItem)
			destPrimitives = append(destPrimitives, srcPrims)
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
			if primitivesMatch(srcPrims, dp) {
				if _, err := mergeMaps(srcMapVal, (*dst)[ci], deduplicate, path, cache); err != nil {
					return nil, nil, err
				}
				// Update primitives cache after merge
				destPrimitives[ci] = extractPrimitives((*dst)[ci])
				mergedIndices = append(mergedIndices, ci)
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
	return mergedIndices, destPrimitives, nil
}

func MergeListItem(src any, dst *[]any, deduplicate bool) error {
	if srcMap, isMap := asMap(src); isMap {
		for i, item := range *dst {
			if dstMap, ok := asMap(item); ok {
				if itemsWouldMerge(srcMap, dstMap) {
					_, err := mergeMaps(srcMap, (*dst)[i], deduplicate, "", nil)
					return err
				}
			}
		}
	}
	*dst = append(*dst, src)
	return nil
}
