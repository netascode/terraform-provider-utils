package provider

import (
	"context"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"strings"
)

func TestConvertNativeToDynamic(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    any
		wantType string
		wantErr  bool
	}{
		{
			name:     "string conversion",
			input:    "test string",
			wantType: "types.String",
			wantErr:  false,
		},
		{
			name:     "int conversion",
			input:    42,
			wantType: "types.Number",
			wantErr:  false,
		},
		{
			name:     "float64 conversion",
			input:    3.14,
			wantType: "types.Number",
			wantErr:  false,
		},
		{
			name:     "bool conversion",
			input:    true,
			wantType: "types.Bool",
			wantErr:  false,
		},
		{
			name:     "nil conversion",
			input:    nil,
			wantType: "null",
			wantErr:  false,
		},
		{
			name:     "map conversion",
			input:    map[string]any{"key": "value", "number": 123},
			wantType: "types.Map",
			wantErr:  false,
		},
		{
			name:     "slice conversion",
			input:    []any{1, "two", true},
			wantType: "types.List",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertNativeToDynamic(ctx, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("convertNativeToDynamic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Skip further checks if we expected an error
			}

			// Verify the result is not null (unless input was nil)
			if tt.input == nil {
				if !result.IsNull() {
					t.Errorf("convertNativeToDynamic() with nil input should return null, got %v", result)
				}
				return
			}

			if result.IsNull() {
				t.Errorf("convertNativeToDynamic() returned null for non-nil input %v", tt.input)
			}

			// Additional type-specific checks
			underlyingValue := result.UnderlyingValue()
			switch tt.wantType {
			case "types.String":
				if _, ok := underlyingValue.(types.String); !ok {
					t.Errorf("convertNativeToDynamic() expected String type, got %T", underlyingValue)
				}
			case "types.Number":
				if _, ok := underlyingValue.(types.Number); !ok {
					t.Errorf("convertNativeToDynamic() expected Number type, got %T", underlyingValue)
				}
			case "types.Bool":
				if _, ok := underlyingValue.(types.Bool); !ok {
					t.Errorf("convertNativeToDynamic() expected Bool type, got %T", underlyingValue)
				}
			case "types.Map":
				// We now use Object for maps to support mixed types
				if _, ok := underlyingValue.(types.Object); !ok {
					t.Errorf("convertNativeToDynamic() expected Object type, got %T", underlyingValue)
				}
			case "types.List":
				// We now use Tuple for arrays to support mixed types
				if _, ok := underlyingValue.(types.Tuple); !ok {
					t.Errorf("convertNativeToDynamic() expected Tuple type, got %T", underlyingValue)
				}
			}
		})
	}
}

func TestConvertDynamicToNative(t *testing.T) {
	tests := []struct {
		name      string
		input     types.Dynamic
		wantType  string
		wantValue any
		wantErr   bool
	}{
		{
			name:      "string conversion",
			input:     types.DynamicValue(types.StringValue("test")),
			wantType:  "string",
			wantValue: "test",
			wantErr:   false,
		},
		{
			name:      "number conversion",
			input:     types.DynamicValue(types.NumberValue(big.NewFloat(42))),
			wantType:  "int",
			wantValue: 42,
			wantErr:   false,
		},
		{
			name:      "bool conversion",
			input:     types.DynamicValue(types.BoolValue(true)),
			wantType:  "bool",
			wantValue: true,
			wantErr:   false,
		},
		{
			name:     "null conversion",
			input:    types.DynamicNull(),
			wantType: "nil",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertDynamicToNative(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("convertDynamicToNative() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Skip further checks if we expected an error
			}

			// Type-specific checks
			switch tt.wantType {
			case "string":
				if str, ok := result.(string); !ok || str != tt.wantValue {
					t.Errorf("convertDynamicToNative() = %v (%T), want %v (%s)", result, result, tt.wantValue, tt.wantType)
				}
			case "int":
				if intVal, ok := result.(int); !ok || intVal != tt.wantValue {
					t.Errorf("convertDynamicToNative() = %v (%T), want %v (%s)", result, result, tt.wantValue, tt.wantType)
				}
			case "bool":
				if boolVal, ok := result.(bool); !ok || boolVal != tt.wantValue {
					t.Errorf("convertDynamicToNative() = %v (%T), want %v (%s)", result, result, tt.wantValue, tt.wantType)
				}
			case "nil":
				if result != nil {
					t.Errorf("convertDynamicToNative() = %v, want nil", result)
				}
			}
		})
	}
}

func TestConvertNativeToDynamicRecursionLimit(t *testing.T) {
	ctx := context.Background()

	// Create a deeply nested structure that exceeds the recursion limit
	var deepMap any = "bottom"
	for i := 0; i < 102; i++ { // Exceed the 100 level limit
		deepMap = map[string]any{
			"level": deepMap,
		}
	}

	_, err := convertNativeToDynamic(ctx, deepMap)
	if err == nil {
		t.Error("convertNativeToDynamic() should have failed with recursion depth error")
	}

	if err != nil {
		// Check that the error message contains the recursion depth error
		errMsg := err.Error()
		if !strings.Contains(errMsg, "maximum recursion depth exceeded (100 levels)") {
			t.Errorf("convertNativeToDynamic() should contain recursion depth error, got: %v", err)
		}
	}
}

func TestValidateInputSize(t *testing.T) {
	// Test small input that should pass
	smallInput := types.DynamicValue(types.StringValue("small"))
	err := validateInputSize(smallInput, 1000)
	if err != nil {
		t.Errorf("validateInputSize() should not fail for small input: %v", err)
	}

	// Test that would exceed size limit (this is approximated)
	largeString := make([]byte, 1000)
	for i := range largeString {
		largeString[i] = 'x'
	}
	largeInput := types.DynamicValue(types.StringValue(string(largeString)))
	err = validateInputSize(largeInput, 500) // Set limit smaller than string
	if err == nil {
		t.Error("validateInputSize() should fail for large input")
	}
}