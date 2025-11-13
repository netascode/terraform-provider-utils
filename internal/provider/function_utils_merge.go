package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = MergeFunction{}

func NewMergeFunction() function.Function {
	return &MergeFunction{}
}

type MergeFunction struct{}

func (r MergeFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "merge"
}

func (r MergeFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Merge a list of data structures",
		MarkdownDescription: "Merge a list of data structures into a single data structure, where maps are deep merged and list entries are compared against existing list entries and if all primitive values match, the entries are deep merged.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:                "input",
				MarkdownDescription: "A list of data structures to be merged.",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (r MergeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var inputDynamic types.Dynamic

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &inputDynamic))

	if resp.Error != nil {
		return
	}

	// Security control: Add timeout protection for merge operations
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Security control: Validate input size to prevent memory exhaustion
	if err := validateInputSize(inputDynamic, 10*1024*1024); err != nil { // 10MB limit
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Input size validation failed: "+err.Error()))
		return
	}

	// Extract the list from the dynamic input
	var input []types.Dynamic
	inputValue := inputDynamic.UnderlyingValue()

	switch v := inputValue.(type) {
	case types.List:
		if v.IsNull() || v.IsUnknown() {
			input = []types.Dynamic{}
		} else {
			for _, elem := range v.Elements() {
				if dynElem, ok := elem.(types.Dynamic); ok {
					input = append(input, dynElem)
				} else {
					input = append(input, types.DynamicValue(elem))
				}
			}
		}
	case types.Tuple:
		if v.IsNull() || v.IsUnknown() {
			input = []types.Dynamic{}
		} else {
			for _, elem := range v.Elements() {
				if dynElem, ok := elem.(types.Dynamic); ok {
					input = append(input, dynElem)
				} else {
					input = append(input, types.DynamicValue(elem))
				}
			}
		}
	default:
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Input must be a list of data structures"))
		return
	}

	// Convert first input to base map, or initialize empty map
	var merged map[string]any
	if len(input) == 0 {
		merged = map[string]any{}
	} else {
		// Convert first dynamic value to map[string]any
		firstValue, err := convertDynamicToNative(input[0])
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting first input: "+err.Error()))
			return
		}

		if firstMap, ok := firstValue.(map[string]any); ok {
			// Create a copy to avoid modifying the original
			merged = make(map[string]any)
			for k, v := range firstMap {
				merged[k] = v
			}
		} else {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("First input must be a map/object"))
			return
		}
	}

	// Merge remaining inputs
	for i := 1; i < len(input); i++ {
		data, err := convertDynamicToNative(input[i])
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting input: "+err.Error()))
			return
		}

		if dataMap, ok := data.(map[string]any); ok {
			MergeMaps(dataMap, merged, true)
		} else {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("All inputs must be maps/objects"))
			return
		}
	}

	// Convert back to Dynamic
	result, err := convertNativeToDynamic(ctx, merged)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting result: "+err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}