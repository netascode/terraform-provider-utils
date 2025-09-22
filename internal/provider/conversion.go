package provider

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// convertDynamicToNative converts a types.Dynamic to a native Go type
func convertDynamicToNative(d types.Dynamic) (any, error) {
	val := d.UnderlyingValue()

	switch v := val.(type) {
	case types.String:
		if v.IsNull() {
			return nil, nil
		}
		return v.ValueString(), nil
	case types.Number:
		if v.IsNull() {
			return nil, nil
		}
		// Try to get as int first, fallback to float64
		if intVal, exact := v.ValueBigFloat().Int64(); exact == big.Exact {
			return int(intVal), nil
		}
		floatVal, _ := v.ValueBigFloat().Float64()
		return floatVal, nil
	case types.Bool:
		if v.IsNull() {
			return nil, nil
		}
		return v.ValueBool(), nil
	case types.List:
		if v.IsNull() {
			return nil, nil
		}
		var result []any
		for _, elem := range v.Elements() {
			if dynElem, ok := elem.(types.Dynamic); ok {
				nativeElem, err := convertDynamicToNative(dynElem)
				if err != nil {
					return nil, err
				}
				result = append(result, nativeElem)
			} else {
				// Handle non-dynamic elements by wrapping them
				nativeElem, err := convertDynamicToNative(types.DynamicValue(elem))
				if err != nil {
					return nil, err
				}
				result = append(result, nativeElem)
			}
		}
		return result, nil
	case types.Map:
		if v.IsNull() {
			return nil, nil
		}
		result := make(map[string]any)
		for key, elem := range v.Elements() {
			if dynElem, ok := elem.(types.Dynamic); ok {
				nativeElem, err := convertDynamicToNative(dynElem)
				if err != nil {
					return nil, err
				}
				result[key] = nativeElem
			} else {
				// Handle non-dynamic elements by wrapping them
				nativeElem, err := convertDynamicToNative(types.DynamicValue(elem))
				if err != nil {
					return nil, err
				}
				result[key] = nativeElem
			}
		}
		return result, nil
	case types.Object:
		if v.IsNull() {
			return nil, nil
		}
		result := make(map[string]any)
		for key, elem := range v.Attributes() {
			if dynElem, ok := elem.(types.Dynamic); ok {
				nativeElem, err := convertDynamicToNative(dynElem)
				if err != nil {
					return nil, err
				}
				result[key] = nativeElem
			} else {
				// Handle non-dynamic elements by wrapping them
				nativeElem, err := convertDynamicToNative(types.DynamicValue(elem))
				if err != nil {
					return nil, err
				}
				result[key] = nativeElem
			}
		}
		return result, nil
	case types.Tuple:
		if v.IsNull() {
			return nil, nil
		}
		var result []any
		for _, elem := range v.Elements() {
			if dynElem, ok := elem.(types.Dynamic); ok {
				nativeElem, err := convertDynamicToNative(dynElem)
				if err != nil {
					return nil, err
				}
				result = append(result, nativeElem)
			} else {
				// Handle non-dynamic elements by wrapping them
				nativeElem, err := convertDynamicToNative(types.DynamicValue(elem))
				if err != nil {
					return nil, err
				}
				result = append(result, nativeElem)
			}
		}
		return result, nil
	default:
		// For any other type, try to extract the underlying value directly
		return val, nil
	}
}

// convertNativeToDynamic converts a native Go type to types.Dynamic with proper type handling
func convertNativeToDynamic(ctx context.Context, val any) (types.Dynamic, error) {
	return convertNativeToDynamicWithDepth(ctx, val, 0)
}

// convertNativeToDynamicWithDepth converts a native Go type to types.Dynamic with recursion depth tracking
func convertNativeToDynamicWithDepth(ctx context.Context, val any, depth int) (types.Dynamic, error) {
	// Security control: prevent stack overflow from deep recursion
	if depth > 100 {
		return types.DynamicNull(), fmt.Errorf("maximum recursion depth exceeded (100 levels)")
	}

	if val == nil {
		return types.DynamicNull(), nil
	}

	switch v := val.(type) {
	case string:
		return types.DynamicValue(types.StringValue(v)), nil
	case int:
		return types.DynamicValue(types.NumberValue(big.NewFloat(float64(v)))), nil
	case int32:
		return types.DynamicValue(types.NumberValue(big.NewFloat(float64(v)))), nil
	case int64:
		return types.DynamicValue(types.NumberValue(big.NewFloat(float64(v)))), nil
	case float32:
		return types.DynamicValue(types.NumberValue(big.NewFloat(float64(v)))), nil
	case float64:
		return types.DynamicValue(types.NumberValue(big.NewFloat(v))), nil
	case bool:
		return types.DynamicValue(types.BoolValue(v)), nil
	case []any:
		elements := make([]attr.Value, len(v))
		elementTypes := make([]attr.Type, len(v))
		for i, elem := range v {
			dynElem, err := convertNativeToDynamicWithDepth(ctx, elem, depth+1)
			if err != nil {
				return types.DynamicNull(), fmt.Errorf("error converting list element %d: %w", i, err)
			}
			elements[i] = dynElem
			elementTypes[i] = types.DynamicType
		}
		// Use TupleValue for arrays with potentially mixed-type elements
		tupleVal, diag := types.TupleValue(elementTypes, elements)
		if diag.HasError() {
			return types.DynamicNull(), fmt.Errorf("error creating tuple value: %s", diag.Errors()[0].Summary())
		}
		return types.DynamicValue(tupleVal), nil
	case map[string]any:
		attrs := make(map[string]attr.Value)
		attrTypes := make(map[string]attr.Type)
		for key, elem := range v {
			dynElem, err := convertNativeToDynamicWithDepth(ctx, elem, depth+1)
			if err != nil {
				return types.DynamicNull(), fmt.Errorf("error converting map element '%s': %w", key, err)
			}
			attrs[key] = dynElem
			attrTypes[key] = types.DynamicType
		}
		// Use ObjectValue for mixed-type maps
		objVal, diag := types.ObjectValue(attrTypes, attrs)
		if diag.HasError() {
			return types.DynamicNull(), fmt.Errorf("error creating object value: %s", diag.Errors()[0].Summary())
		}
		return types.DynamicValue(objVal), nil
	default:
		return types.DynamicNull(), fmt.Errorf("unsupported type: %T", val)
	}
}

// validateInputSize validates that the input doesn't exceed the maximum allowed size
func validateInputSize(input types.Dynamic, maxSizeBytes int64) error {
	size, err := calculateDynamicSize(input, 0)
	if err != nil {
		return fmt.Errorf("error calculating input size: %w", err)
	}

	if size > maxSizeBytes {
		return fmt.Errorf("input size (%d bytes) exceeds maximum allowed size (%d bytes)", size, maxSizeBytes)
	}

	return nil
}

// calculateDynamicSize estimates the memory usage of a Dynamic value
func calculateDynamicSize(d types.Dynamic, depth int) (int64, error) {
	// Prevent infinite recursion in size calculation
	if depth > 100 {
		return 0, fmt.Errorf("maximum recursion depth exceeded during size calculation")
	}

	val := d.UnderlyingValue()
	var size int64 = 8 // Base size for the Dynamic wrapper

	switch v := val.(type) {
	case types.String:
		if !v.IsNull() {
			size += int64(len(v.ValueString()))
		}
	case types.Number:
		size += 24 // Approximate size for big.Float
	case types.Bool:
		size += 1
	case types.List:
		if !v.IsNull() {
			for _, elem := range v.Elements() {
				if dynElem, ok := elem.(types.Dynamic); ok {
					elemSize, err := calculateDynamicSize(dynElem, depth+1)
					if err != nil {
						return 0, err
					}
					size += elemSize
				} else {
					size += 8 // Estimate for non-dynamic elements
				}
			}
		}
	case types.Map:
		if !v.IsNull() {
			for key, elem := range v.Elements() {
				size += int64(len(key)) // Key size
				if dynElem, ok := elem.(types.Dynamic); ok {
					elemSize, err := calculateDynamicSize(dynElem, depth+1)
					if err != nil {
						return 0, err
					}
					size += elemSize
				} else {
					size += 8 // Estimate for non-dynamic elements
				}
			}
		}
	case types.Object:
		if !v.IsNull() {
			for key, elem := range v.Attributes() {
				size += int64(len(key)) // Key size
				if dynElem, ok := elem.(types.Dynamic); ok {
					elemSize, err := calculateDynamicSize(dynElem, depth+1)
					if err != nil {
						return 0, err
					}
					size += elemSize
				} else {
					size += 8 // Estimate for non-dynamic elements
				}
			}
		}
	default:
		size += 8 // Default estimate for unknown types
	}

	return size, nil
}