package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

var _ function.Function = YamlDecodeFunction{}

func NewYamlDecodeFunction() function.Function {
	return &YamlDecodeFunction{}
}

type YamlDecodeFunction struct{}

func (r YamlDecodeFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "yaml_decode"
}

func (r YamlDecodeFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Decode a YAML string into a Terraform value",
		MarkdownDescription: "Decode a YAML-formatted string and return the resulting value. Uses the `goccy/go-yaml` library for YAML 1.2 compliant parsing. Unknown YAML tags on scalar values are preserved as literal strings (e.g., `!env ABC` becomes the string `\"!env ABC\"`). Standard YAML tags (`!!str`, `!!int`, `!!float`, `!!bool`, `!!null`, `!!map`, `!!seq`, `!!timestamp`, `!!binary`) are handled normally. Only a single YAML document is supported.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "input",
				MarkdownDescription: "A YAML-formatted string to decode. Must contain exactly one YAML document.",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (r YamlDecodeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var input string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &input))
	if resp.Error != nil {
		return
	}

	// Security control: Add timeout protection
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Security control: Validate input size to prevent memory exhaustion
	if int64(len(input)) > 10*1024*1024 { // 10MB limit
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Input size (%d bytes) exceeds maximum allowed size (10MB)", len(input))))
		return
	}

	// Decode YAML with tag preservation
	native, err := yamlDecode(input)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error decoding YAML: "+err.Error()))
		return
	}

	// Convert native Go value to Terraform Dynamic type
	result, err := convertNativeToDynamic(ctx, native)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting result: "+err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
