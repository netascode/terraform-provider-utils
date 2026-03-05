package provider

import (
	"sort"
)

func MergeMaps(src, dst map[string]any, deduplicate bool) map[string]any {
	for key, sValue := range src {
		if sValue == nil {
			// nil source values delete nil destination keys (matching reflect.SetMapIndex behavior)
			if dValue, exists := dst[key]; exists && dValue == nil {
				delete(dst, key)
			}
			continue
		}
		dValue, exists := dst[key]
		if !exists || dValue == nil {
			dst[key] = sValue
		} else {
			switch sv := sValue.(type) {
			case map[string]any:
				if dv, ok := dValue.(map[string]any); ok {
					dst[key] = MergeMaps(sv, dv, deduplicate)
				}
			case []any:
				if dv, ok := dValue.([]any); ok {
					if deduplicate {
						if len(sv) == 0 || len(dv) == 0 {
							dst[key] = append(dv, sv...)
						} else if hasDuplicatesInList(sv) || hasDuplicatesInList(dv) {
							dst[key] = append(dv, sv...)
						} else {
							merged := dv
							mergeListItemsIndexed(sv, &merged, deduplicate)
							dst[key] = merged
						}
					} else {
						dst[key] = append(dv, sv...)
					}
				}
			default:
				if s, ok := sValue.(string); ok && s == "" {
					continue
				}
				dst[key] = sValue
			}
		}
	}
	return dst
}

// itemsWouldMerge checks if two map items would merge based on primitive field matching
func itemsWouldMerge(item1, item2 map[string]any) bool {
	comparison := false

	// Check item1 primitive fields against item2
	for k, v1 := range item1 {
		if !isPrimitive(v1) {
			continue
		}
		v2, ok := item2[k]
		if !ok {
			continue
		}
		if !isPrimitive(v2) {
			continue
		}
		comparison = true
		if v1 != v2 {
			return false
		}
	}

	// Check item2 primitive fields against item1
	for k, v2 := range item2 {
		if !isPrimitive(v2) {
			continue
		}
		v1, ok := item1[k]
		if !ok {
			continue
		}
		if !isPrimitive(v1) {
			continue
		}
		comparison = true
		if v1 != v2 {
			return false
		}
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
	case map[string]any, []any:
		return false
	default:
		return true
	}
}

// extractPrimitives returns only primitive (non-map, non-slice) key-value pairs from a map
func extractPrimitives(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if isPrimitive(v) {
			result[k] = v
		}
	}
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
	var dictItems []map[string]any
	var primsList []map[string]any
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			dictItems = append(dictItems, m)
			primsList = append(primsList, extractPrimitives(m))
		}
	}

	if len(dictItems) < 2 {
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
func mergeListItemsIndexed(sourceItems []any, dst *[]any, deduplicate bool) {
	// Build inverted index over destination's dict items
	destPrimitives := make([]map[string]any, len(*dst))
	for i, item := range *dst {
		if m, ok := item.(map[string]any); ok {
			destPrimitives[i] = extractPrimitives(m)
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
		srcMap, ok := srcItem.(map[string]any)
		if !ok {
			*dst = append(*dst, srcItem)
			continue
		}

		srcPrims := extractPrimitives(srcMap)
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
				MergeMaps(srcMap, (*dst)[ci].(map[string]any), deduplicate)
				// Update primitives cache after merge
				destPrimitives[ci] = extractPrimitives((*dst)[ci].(map[string]any))
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
}

func MergeListItem(src any, dst *[]any, deduplicate bool) {
	if srcMap, ok := src.(map[string]any); ok {
		for i, item := range *dst {
			if dstMap, ok := item.(map[string]any); ok {
				if itemsWouldMerge(srcMap, dstMap) {
					MergeMaps(srcMap, (*dst)[i].(map[string]any), deduplicate)
					return
				}
			}
		}
	}
	*dst = append(*dst, src)
}
