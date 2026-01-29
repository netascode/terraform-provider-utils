package provider

import (
	"reflect"
)

func MergeMaps(src, dst map[string]any) map[string]any {
	return MergeMapsWithDepth(src, dst, 0)
}

// MergeMapsWithDepth merges maps with recursion depth tracking for security
func MergeMapsWithDepth(src, dst map[string]any, depth int) map[string]any {
	// Security control: prevent stack overflow from deep recursion
	if depth > 100 {
		panic("maximum recursion depth exceeded (100 levels) during map merge")
	}

	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst)

	// iterate over source map keys and values
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
				dstValue.SetMapIndex(sKey, reflect.ValueOf(MergeMapsWithDepth(sValue.Interface().(map[string]any), dValue.Interface().(map[string]any), depth+1)))
			}
		} else if sValue.Kind() == reflect.Slice {
			dValue = reflect.ValueOf(dValue.Interface())
			if dValue.Kind() == reflect.Slice {
				dstValue.SetMapIndex(sKey, reflect.AppendSlice(dValue, sValue))
			}
		} else if sValue.Kind() != reflect.Invalid && !(sValue.Kind() == reflect.String && sValue.IsZero()) {
			// else we have primitive type -> add/replace dst value
			dstValue.SetMapIndex(sKey, sValue)
		}
	}
	return dstValue.Interface().(map[string]any)
}

func MergeListItem(src any, dst *[]any) {
	srcValue := reflect.ValueOf(src)

	if srcValue.Kind() == reflect.Interface {
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() == reflect.Map {
		for i, item := range *dst {
			// Check if the destination item is also a map before trying to compare
			itemValue := reflect.ValueOf(item)
			if itemValue.Kind() == reflect.Interface {
				itemValue = itemValue.Elem()
			}
			// Skip this item if it's not a map - can't merge map with non-map
			if itemValue.Kind() != reflect.Map {
				continue
			}

			match := true
			comparison := false
			unique_source := false
			unique_dest := false
			// iterate over all source map keys and values
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

				x := itemValue.MapIndex(sKey)
				if x.Kind() == reflect.Interface {
					x = x.Elem()
				}
				if sValue.Kind() == reflect.Map || sValue.Kind() == reflect.Slice {
					continue
				}
				if !x.IsValid() || !sValue.IsValid() {
					unique_source = true
					continue
				}
				comparison = true
				if sValue.Interface() != x.Interface() {
					match = false
				}
			}
			// iterate over all dst map keys and values
			iter = itemValue.MapRange()
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
					unique_dest = true
					continue
				}
				comparison = true
				if dValue.Interface() != x.Interface() {
					match = false
				}
			}
			// Check if all primitive values have matched AND at least one comparison was done
			if match && comparison && !(unique_source && unique_dest) {
				MergeMapsWithDepth(srcValue.Interface().(map[string]any), (*dst)[i].(map[string]any), 0)
				return
			}
		}

	}
	t := append(*dst, src)
	*dst = t
}

func DeduplicateListItems(data map[string]any) map[string]any {
	return DeduplicateListItemsWithDepth(data, 0)
}

// DeduplicateListItemsWithDepth deduplicates list items with recursion depth tracking
func DeduplicateListItemsWithDepth(data map[string]any, depth int) map[string]any {
	// Security control: prevent stack overflow from deep recursion
	if depth > 100 {
		panic("maximum recursion depth exceeded (100 levels) during list deduplication")
	}

	dataValue := reflect.ValueOf(data)
	iter := dataValue.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() == reflect.Interface {
			key = key.Elem()
		}
		value := iter.Value()
		if value.Kind() == reflect.Interface {
			value = value.Elem()
		}

		if value.Kind() == reflect.Map {
			DeduplicateListItemsWithDepth(value.Interface().(map[string]any), depth+1)
		} else if value.Kind() == reflect.Slice {
			deduplicatedList := make([]any, 0, value.Len())
			for i := range value.Len() {
				MergeListItem(value.Index(i).Interface(), &deduplicatedList)
			}
			for _, item := range deduplicatedList {
				if reflect.ValueOf(item).Kind() == reflect.Map {
					DeduplicateListItemsWithDepth(item.(map[string]any), depth+1)
				}
			}
			dataValue.SetMapIndex(key, reflect.ValueOf(deduplicatedList))
		}
	}
	return dataValue.Interface().(map[string]any)
}
