package provider

import (
	"reflect"
	"sort"
)

func MergeMaps(src, dst map[string]any, deduplicate bool) map[string]any {
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst)

	// Iterate over source map keys and values
	iter := srcValue.MapRange()
	for iter.Next() {
		sKey := iter.Key()
		if sKey.Kind() == reflect.Interface {
			sKey = sKey.Elem()
		}
		sValue := iter.Value()
		if sValue.Kind() == reflect.Interface {
			sValue = sValue.Elem()
		}

		dValue := dstValue.MapIndex(sKey)
		if !dValue.IsValid() || dValue.IsZero() {
			dstValue.SetMapIndex(sKey, sValue)
		} else if sValue.Kind() == reflect.Map {
			dValue = reflect.ValueOf(dValue.Interface())
			if dValue.Kind() == reflect.Map {
				dstValue.SetMapIndex(sKey, reflect.ValueOf(MergeMaps(sValue.Interface().(map[string]any), dValue.Interface().(map[string]any), deduplicate)))
			}
		} else if sValue.Kind() == reflect.Slice {
			dValue = reflect.ValueOf(dValue.Interface())
			if dValue.Kind() == reflect.Slice {
				if deduplicate {
					// Skip empty lists
					if sValue.Len() == 0 || dValue.Len() == 0 {
						dstValue.SetMapIndex(sKey, reflect.AppendSlice(dValue, sValue))
					} else if hasDuplicatesInList(sValue) || hasDuplicatesInList(dValue) {
						// Check if either source or destination list has duplicates
						// If duplicates exist, skip merging to preserve them
						// Concatenate without merging to preserve all duplicates
						dstValue.SetMapIndex(sKey, reflect.AppendSlice(dValue, sValue))
					} else {
						// No duplicates: merge matching items across files
						merged := dValue.Interface().([]any)
						srcSlice := make([]any, sValue.Len())
						for i := range sValue.Len() {
							srcSlice[i] = sValue.Index(i).Interface()
						}
						mergeListItemsIndexed(srcSlice, &merged, deduplicate)
						dstValue.SetMapIndex(sKey, reflect.ValueOf(merged))
					}
				} else {
					// Simple append (original behavior)
					dstValue.SetMapIndex(sKey, reflect.AppendSlice(dValue, sValue))
				}
			}
		} else if sValue.Kind() != reflect.Invalid && !(sValue.Kind() == reflect.String && sValue.IsZero()) {
			// Else we have primitive type - add/replace dst value
			dstValue.SetMapIndex(sKey, sValue)
		}
	}
	return dstValue.Interface().(map[string]any)
}

// itemsWouldMerge checks if two map items would merge based on primitive field matching
func itemsWouldMerge(item1, item2 reflect.Value) bool {
	if item1.Kind() == reflect.Interface {
		item1 = item1.Elem()
	}
	if item2.Kind() == reflect.Interface {
		item2 = item2.Elem()
	}

	if item1.Kind() != reflect.Map || item2.Kind() != reflect.Map {
		return false
	}

	comparison := false

	// Check item1 primitive fields against item2
	iter := item1.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() == reflect.Interface {
			key = key.Elem()
		}
		value := iter.Value()
		if value.Kind() == reflect.Interface {
			value = value.Elem()
		}

		if value.Kind() == reflect.Map || value.Kind() == reflect.Slice {
			continue
		}

		item2Value := item2.MapIndex(key)
		if item2Value.Kind() == reflect.Interface {
			item2Value = item2Value.Elem()
		}

		if !item2Value.IsValid() {
			continue
		}

		if item2Value.Kind() == reflect.Map || item2Value.Kind() == reflect.Slice {
			continue
		}

		comparison = true
		if value.Interface() != item2Value.Interface() {
			return false // Early exit on mismatch
		}
	}

	// Check item2 primitive fields against item1
	iter = item2.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() == reflect.Interface {
			key = key.Elem()
		}
		value := iter.Value()
		if value.Kind() == reflect.Interface {
			value = value.Elem()
		}

		if value.Kind() == reflect.Map || value.Kind() == reflect.Slice {
			continue
		}

		item1Value := item1.MapIndex(key)
		if item1Value.Kind() == reflect.Interface {
			item1Value = item1Value.Elem()
		}

		if !item1Value.IsValid() {
			continue
		}

		if item1Value.Kind() == reflect.Map || item1Value.Kind() == reflect.Slice {
			continue
		}

		comparison = true
		if value.Interface() != item1Value.Interface() {
			return false // Early exit on mismatch
		}
	}

	return comparison
}

// kvPair represents a key-value pair used as an inverted index key
type kvPair struct {
	key   string
	value any
}

// extractPrimitives returns only primitive (non-map, non-slice) key-value pairs from a map
func extractPrimitives(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map || rv.Kind() == reflect.Slice {
			continue
		}
		result[k] = v
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
func hasDuplicatesInList(listValue reflect.Value) bool {
	// Only check dict items for duplicates, precompute primitives
	var dictItems []map[string]any
	var primsList []map[string]any
	for i := range listValue.Len() {
		item := listValue.Index(i)
		if item.Kind() == reflect.Interface {
			item = item.Elem()
		}
		if item.Kind() == reflect.Map {
			m := item.Interface().(map[string]any)
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
	srcValue := reflect.ValueOf(src)

	if srcValue.Kind() == reflect.Interface {
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() == reflect.Map {
		for i, item := range *dst {
			itemValue := reflect.ValueOf(item)
			if itemValue.Kind() == reflect.Interface {
				itemValue = itemValue.Elem()
			}
			if itemValue.Kind() != reflect.Map {
				continue
			}

			if itemsWouldMerge(srcValue, itemValue) {
				MergeMaps(srcValue.Interface().(map[string]any), (*dst)[i].(map[string]any), deduplicate)
				return
			}
		}

	}
	t := append(*dst, src)
	*dst = t
}
