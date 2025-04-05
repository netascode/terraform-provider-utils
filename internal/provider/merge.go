package provider

import (
	"reflect"
)

func MergeMaps(src, dst map[string]any) map[string]any {
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
				dstValue.SetMapIndex(sKey, reflect.ValueOf(MergeMaps(sValue.Interface().(map[string]any), dValue.Interface().(map[string]any))))
			}
		} else if sValue.Kind() == reflect.Slice {
			dValue = reflect.ValueOf(dValue.Interface())
			if dValue.Kind() == reflect.Slice {
				dstValue.SetMapIndex(sKey, reflect.AppendSlice(dValue, sValue))
			}
		} else if sValue.Kind() != reflect.Invalid && !sValue.IsZero() {
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
			match := true
			comparison := false
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

				x := reflect.ValueOf(item).MapIndex(sKey)
				if x.Kind() == reflect.Interface {
					x = x.Elem()
				}
				if sValue.Kind() == reflect.Map || sValue.Kind() == reflect.Slice || !x.IsValid() || !sValue.IsValid() {
					continue
				}
				comparison = true
				if sValue.Interface() != x.Interface() {
					match = false
				}
			}
			// iterate over all dst map keys and values
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
				if dValue.Kind() == reflect.Map || dValue.Kind() == reflect.Slice || !x.IsValid() || !dValue.IsValid() {
					continue
				}
				comparison = true
				if dValue.Interface() != x.Interface() {
					match = false
				}
			}
			// Check if all primitive values have matched AND at least one comparison was done
			if match && comparison {
				MergeMaps(srcValue.Interface().(map[string]any), (*dst)[i].(map[string]any))
				return
			}
		}

	}
	t := append(*dst, src)
	*dst = t
}

func DeduplicateListItems(data map[string]any) map[string]any {
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
			DeduplicateListItems(value.Interface().(map[string]any))
		} else if value.Kind() == reflect.Slice {
			deduplicatedList := make([]any, 0, value.Len())
			for i := range value.Len() {
				MergeListItem(value.Index(i).Interface(), &deduplicatedList)
			}
			for _, item := range deduplicatedList {
				if reflect.ValueOf(item).Kind() == reflect.Map {
					DeduplicateListItems(item.(map[string]any))
				}
			}
			dataValue.SetMapIndex(key, reflect.ValueOf(deduplicatedList))
		}
	}
	return dataValue.Interface().(map[string]any)
}
