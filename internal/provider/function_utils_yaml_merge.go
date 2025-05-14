package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

var _ function.Function = YamlMergeFunction{}

func NewYamlMergeFunction() function.Function {
	return &YamlMergeFunction{}
}

type YamlMergeFunction struct{}

func (r YamlMergeFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "yaml_merge"
}

func (r YamlMergeFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Merge a list of YAML strings",
		MarkdownDescription: "Merge a list of YAML strings into a single YAML string, where maps are deep merged and list entries are compared against existing list entries and if all primitive values match, the entries are deep merged. YAML `!env` tags can be used to resolve values from environment variables.",
		Parameters: []function.Parameter{
			function.ListParameter{
				Name:                "input",
				ElementType:         types.StringType,
				MarkdownDescription: "A list of YAML strings that is merged.",
			},
		},
		Return: function.StringReturn{},
	}
}

func (r YamlMergeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var input []string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &input))

	if resp.Error != nil {
		return
	}

	merged := map[string]any{}
	for _, input := range input {
		var data map[string]any
		b := []byte(input)

		err := YamlUnmarshal(b, &data)
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error reading YAML string: "+err.Error()))
			return
		}

		MergeMaps(data, merged)
	}

	DeduplicateListItems(merged)

	output, err := yaml.Marshal(merged)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting results to YAML: "+err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, string(output)))
}
