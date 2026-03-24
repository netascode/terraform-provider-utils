package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = YamlEncodeFunction{}

func NewYamlEncodeFunction() function.Function {
	return &YamlEncodeFunction{}
}

type YamlEncodeFunction struct{}

func (r YamlEncodeFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "yaml_encode"
}

func (r YamlEncodeFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Encode a value as a YAML string",
		MarkdownDescription: "Encode a given value as a YAML string using the `goccy/go-yaml` library. Produces YAML 1.2 compliant block-style output with 2-space indentation. Map keys are sorted alphabetically, strings that could be misinterpreted are automatically quoted, and null values are rendered as `null`.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:                "input",
				AllowNullValue:      true,
				AllowUnknownValues:  true,
				MarkdownDescription: "The value to encode as YAML. Can be any Terraform value type including strings, numbers, booleans, lists, maps, objects, and null.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (r YamlEncodeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var inputDynamic types.Dynamic

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &inputDynamic))
	if resp.Error != nil {
		return
	}

	// Security control: Add timeout protection
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Security control: Validate input size to prevent memory exhaustion
	if err := validateInputSize(inputDynamic, 10*1024*1024); err != nil { // 10MB limit
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Input size validation failed: "+err.Error()))
		return
	}

	// Handle top-level null/unknown
	if inputDynamic.IsNull() || inputDynamic.IsUnknown() {
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, "null\n"))
		return
	}

	// Convert Terraform Dynamic value to native Go types
	native, err := convertDynamicToNative(inputDynamic)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting input: "+err.Error()))
		return
	}

	// Encode to YAML using goccy/go-yaml
	result, err := yamlEncode(native)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error encoding to YAML: "+err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
