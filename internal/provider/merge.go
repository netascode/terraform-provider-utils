package provider

import (
	"reflect"
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
						for i := range sValue.Len() {
							MergeListItem(sValue.Index(i).Interface(), &merged, deduplicate)
						}
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

		comparison = true
		if value.Interface() != item1Value.Interface() {
			return false // Early exit on mismatch
		}
	}

	return comparison
}

// hasDuplicatesInList checks if a list contains duplicate dict items using merge matching logic
func hasDuplicatesInList(listValue reflect.Value) bool {
	// Only check dict items for duplicates
	var dictItems []reflect.Value
	for i := range listValue.Len() {
		item := listValue.Index(i)
		if item.Kind() == reflect.Interface {
			item = item.Elem()
		}
		if item.Kind() == reflect.Map {
			dictItems = append(dictItems, item)
		}
	}

	if len(dictItems) < 2 {
		return false
	}

	// Check each dict against all subsequent dicts
	for i := 0; i < len(dictItems)-1; i++ {
		for j := i + 1; j < len(dictItems); j++ {
			if itemsWouldMerge(dictItems[i], dictItems[j]) {
				return true
			}
		}
	}

	return false
}

func MergeListItem(src any, dst *[]any, deduplicate bool) {
	srcValue := reflect.ValueOf(src)

	if srcValue.Kind() == reflect.Interface {
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() == reflect.Map {
		for i, item := range *dst {
			match := true
			comparison := false

			// Iterate over all source map keys and values
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

				x := reflect.ValueOf(item).MapIndex(sKey)
				if x.Kind() == reflect.Interface {
					x = x.Elem()
				}
				if sValue.Kind() == reflect.Map || sValue.Kind() == reflect.Slice {
					continue
				}
				if !x.IsValid() || !sValue.IsValid() {
					continue
				}
				comparison = true
				if sValue.Interface() != x.Interface() {
					match = false
				}
			}
			// Iterate over all dst map keys and values
			iter = reflect.ValueOf(item).MapRange()
			for iter.Next() {
				dKey := iter.Key()
				if dKey.Kind() == reflect.Interface {
					dKey = dKey.Elem()
				}
				dValue := iter.Value()
				if dValue.Kind() == reflect.Interface {
					dValue = dValue.Elem()
				}

				x := srcValue.MapIndex(dKey)
				if x.Kind() == reflect.Interface {
					x = x.Elem()
				}
				if dValue.Kind() == reflect.Map || dValue.Kind() == reflect.Slice {
					continue
				}
				if !x.IsValid() || !dValue.IsValid() {
					continue
				}
				comparison = true
				if dValue.Interface() != x.Interface() {
					match = false
				}
			}
			// Check if all primitive values have matched AND at least one comparison was done
			if match && comparison {
				MergeMaps(srcValue.Interface().(map[string]any), (*dst)[i].(map[string]any), deduplicate)
				return
			}
		}

	}
	t := append(*dst, src)
	*dst = t
}
