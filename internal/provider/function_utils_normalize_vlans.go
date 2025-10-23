package provider

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Format constants to replace magic strings
const (
	FormatString = "string"
	FormatList   = "list"
)

// ValidVlanFormats defines valid format values
var ValidVlanFormats = map[string]bool{
	FormatString: true,
	FormatList:   true,
}

var _ function.Function = NormalizeVlansFunction{}

// validateVlanID validates a single VLAN ID is within valid range (1-4094)
func validateVlanID(id int64) error {
	if id < 1 || id > 4094 {
		return fmt.Errorf("VLAN ID %d is out of valid range (1-4094)", id)
	}
	return nil
}

// validateVlanRange validates a VLAN range and ensures from <= to and both are valid
func validateVlanRange(from, to int64) error {
	if err := validateVlanID(from); err != nil {
		return fmt.Errorf("range 'from' field: %w", err)
	}
	if err := validateVlanID(to); err != nil {
		return fmt.Errorf("range 'to' field: %w", err)
	}
	if from > to {
		return fmt.Errorf("VLAN range 'from' value %d cannot be greater than 'to' value %d", from, to)
	}
	return nil
}

func NewNormalizeVlansFunction() function.Function {
	return &NormalizeVlansFunction{}
}

type NormalizeVlansFunction struct{}

func (r NormalizeVlansFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "normalize_vlans"
}

func (r NormalizeVlansFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Normalize VLAN IDs and ranges into a compact string format or list of integers",
		MarkdownDescription: "Takes an object with optional `ids` (list of integers) and `ranges` (list of objects with `from`/`to` fields) and a required `format` parameter. Returns a normalized representation as either a string or list of integers. In string format, 3 or more consecutive IDs are grouped into ranges (e.g., '10-20'), while individual or pairs of VLANs are listed separately (e.g., '1,2' not '1-2'). Both `ids` and `ranges` fields are optional and can be omitted from the input object.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:                "input",
				MarkdownDescription: "An object containing optional `ids` (list of VLAN IDs) and `ranges` (list of objects with `from` and `to` fields). Both fields are optional and can be omitted from the input object.",
			},
			function.StringParameter{
				Name:                "format",
				MarkdownDescription: "Required output format: 'string' for compact range notation (e.g., '1,2,5,10-30' where ranges are only used for 3+ consecutive VLANs) or 'list' for array of individual VLAN IDs (e.g., [1,2,5,10,11,12,...,30]).",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (r NormalizeVlansFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var input types.Dynamic
	var formatValue string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &input, &formatValue))
	if resp.Error != nil {
		return
	}

	// Validate format parameter using constants
	if !ValidVlanFormats[formatValue] {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Invalid format '%s'. Must be '%s' or '%s'", formatValue, FormatString, FormatList)))
		return
	}

	// Collect all VLAN IDs
	vlanSet := make(map[int]bool)

	// Handle empty input
	if input.IsNull() || input.IsUnknown() {
		if formatValue == FormatList {
			emptyList, diags := types.ListValue(types.NumberType, []attr.Value{})
			if diags.HasError() {
				resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Failed to create empty list"))
				return
			}
			dynamicValue := types.DynamicValue(emptyList)
			resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, dynamicValue))
		} else {
			resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, ""))
		}
		return
	}

	// Handle object input by checking the underlying value
	switch obj := input.UnderlyingValue().(type) {
	case types.Object:
		elements := obj.Attributes()

		// Handle IDs attribute if present
		if idsVal, exists := elements["ids"]; exists && !idsVal.IsNull() && !idsVal.IsUnknown() {
			// Handle Tuple values (HCL literal arrays like [1, 2, 5])
			if idsTuple, ok := idsVal.(types.Tuple); ok {
				var ids []int64
				// Convert tuple elements to int64 slice
				tupleElements := idsTuple.Elements()
				for _, elem := range tupleElements {
					if numVal, ok := elem.(types.Number); ok {
						val, _ := numVal.ValueBigFloat().Int64()
						ids = append(ids, val)
					} else {
						resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("All IDs must be numbers"))
						return
					}
				}

				// Add individual IDs
				for _, id := range ids {
					if err := validateVlanID(id); err != nil {
						resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("in 'ids' field: %v", err)))
						return
					}
					vlanSet[int(id)] = true
				}
			} else if idsList, ok := idsVal.(types.List); ok {
				// Handle List values
				var ids []int64
				diags := idsList.ElementsAs(ctx, &ids, false)
				if diags.HasError() {
					resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Failed to parse IDs list"))
					return
				}

				// Add individual IDs
				for _, id := range ids {
					if err := validateVlanID(id); err != nil {
						resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("in 'ids' field: %v", err)))
						return
					}
					vlanSet[int(id)] = true
				}
			} else {
				resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("IDs must be a list or tuple, got %T", idsVal)))
				return
			}
		}

		// Handle Ranges attribute if present
		if rangesVal, exists := elements["ranges"]; exists && !rangesVal.IsNull() && !rangesVal.IsUnknown() {
			// Handle Tuple values (HCL literal arrays)
			if rangesTuple, ok := rangesVal.(types.Tuple); ok {
				tupleElements := rangesTuple.Elements()
				for _, elem := range tupleElements {
					if obj, ok := elem.(types.Object); ok {
						attrs := obj.Attributes()
						var from, to int64

						// Extract 'from' field
						if fromVal, exists := attrs["from"]; exists {
							if fromNum, ok := fromVal.(types.Number); ok {
								from, _ = fromNum.ValueBigFloat().Int64()
							} else {
								resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Range 'from' must be a number"))
								return
							}
						} else {
							resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Range missing 'from' field"))
							return
						}

						// Extract 'to' field
						if toVal, exists := attrs["to"]; exists {
							if toNum, ok := toVal.(types.Number); ok {
								to, _ = toNum.ValueBigFloat().Int64()
							} else {
								resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Range 'to' must be a number"))
								return
							}
						} else {
							resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Range missing 'to' field"))
							return
						}

						// Validate range using helper function
						if err := validateVlanRange(from, to); err != nil {
							resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("in 'ranges' field: %v", err)))
							return
						}

						// Add range to set
						for i := from; i <= to; i++ {
							vlanSet[int(i)] = true
						}
					} else {
						resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Range elements must be objects"))
						return
					}
				}
			} else if rangesList, ok := rangesVal.(types.List); ok {
				// Handle List values (fallback)
				var ranges []struct {
					From int64 `tfsdk:"from"`
					To   int64 `tfsdk:"to"`
				}
				diags := rangesList.ElementsAs(ctx, &ranges, false)
				if diags.HasError() {
					resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Failed to parse ranges list"))
					return
				}

				// Add ranges using helper function
				for _, r := range ranges {
					if err := validateVlanRange(r.From, r.To); err != nil {
						resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("in 'ranges' field: %v", err)))
						return
					}
					for i := r.From; i <= r.To; i++ {
						vlanSet[int(i)] = true
					}
				}
			} else {
				resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Ranges must be a list or tuple, got %T", rangesVal)))
				return
			}
		}

	default:
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Input must be an object"))
		return
	}

	// Convert to sorted slice
	var vlans []int
	for vlan := range vlanSet {
		vlans = append(vlans, vlan)
	}
	sort.Ints(vlans)

	// Handle empty result
	if len(vlans) == 0 {
		if formatValue == FormatList {
			emptyList, diags := types.ListValue(types.NumberType, []attr.Value{})
			if diags.HasError() {
				resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Failed to create empty list"))
				return
			}
			dynamicValue := types.DynamicValue(emptyList)
			resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, dynamicValue))
		} else {
			resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, ""))
		}
		return
	}

	if formatValue == FormatList {
		// Return as list of individual VLAN IDs
		var vlanValues []attr.Value
		for _, vlan := range vlans {
			// Create big.Float from integer for NumberValue - more efficient than float64 conversion
			vlanFloat := new(big.Float).SetInt64(int64(vlan))
			vlanValues = append(vlanValues, types.NumberValue(vlanFloat))
		}

		vlanList, diags := types.ListValue(types.NumberType, vlanValues)
		if diags.HasError() {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Failed to create VLAN list"))
			return
		}

		// Convert to dynamic value for return
		dynamicValue := types.DynamicValue(vlanList)
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, dynamicValue))
	} else {
		// Return as string with range notation (only for 3+ consecutive VLANs)
		var result []string
		start := vlans[0]
		end := vlans[0]

		for i := 1; i < len(vlans); i++ {
			if vlans[i] == end+1 {
				// Consecutive VLAN, extend the range
				end = vlans[i]
			} else {
				// Non-consecutive VLAN, finalize the current range
				rangeSize := end - start + 1
				if rangeSize >= 3 {
					// Use range notation for 3 or more consecutive VLANs
					result = append(result, fmt.Sprintf("%d-%d", start, end))
				} else {
					// Output individual VLANs for 1 or 2 consecutive IDs
					for j := start; j <= end; j++ {
						result = append(result, strconv.Itoa(j))
					}
				}
				start = vlans[i]
				end = vlans[i]
			}
		}

		// Add the final range
		rangeSize := end - start + 1
		if rangeSize >= 3 {
			// Use range notation for 3 or more consecutive VLANs
			result = append(result, fmt.Sprintf("%d-%d", start, end))
		} else {
			// Output individual VLANs for 1 or 2 consecutive IDs
			for j := start; j <= end; j++ {
				result = append(result, strconv.Itoa(j))
			}
		}

		output := strings.Join(result, ",")
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, output))
	}
}