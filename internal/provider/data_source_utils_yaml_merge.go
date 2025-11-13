package provider

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

var _ datasource.DataSource = (*yamlMergeDataSource)(nil)

func NewYamlMergeDataSource() datasource.DataSource {
	return &yamlMergeDataSource{}
}

type yamlMergeDataSource struct{}

func (d *yamlMergeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_yaml_merge"
}

func (d *yamlMergeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Merge a list of YAML strings into a single YAML string, where maps are deep merged and list entries are compared against existing list entries and if all primitive values match, the entries are deep merged. YAML `!env` tags can be used to resolve values from environment variables.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Hexadecimal encoding of the checksum of the output.",
				Computed:    true,
			},
			"input": schema.ListAttribute{
				Description: "A list of YAML strings that is merged into the `output` attribute.",
				ElementType: types.StringType,
				Required:    true,
			},
			"output": schema.StringAttribute{
				Description: "The merged output.",
				Computed:    true,
			},
			"merge_list_items": schema.BoolAttribute{
				Description: "Merge list entries if all primitive values match. Default value is `true`.",
				Optional:    true,
			},
		},
	}
}

type YamlMerge struct {
	Id             types.String `tfsdk:"id"`
	Input          []string     `tfsdk:"input"`
	Output         types.String `tfsdk:"output"`
	MergeListItems types.Bool   `tfsdk:"merge_list_items"`
}

func (d *yamlMergeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config YamlMerge

	// Read config
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.MergeListItems.IsUnknown() || config.MergeListItems.IsNull() {
		config.MergeListItems = types.BoolValue(true)
	}

	merged := map[string]any{}
	for _, input := range config.Input {
		var data map[string]any
		b := []byte(input)

		err := YamlUnmarshal(b, &data)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading YAML string",
				fmt.Sprintf("Error reading YAML string: %s", err),
			)
			return
		}

		MergeMaps(data, merged, config.MergeListItems.ValueBool())
	}

	output, err := yaml.Marshal(merged)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error converting result to YAML",
			fmt.Sprintf("Error converting result to YAML: %s", err),
		)
		return
	}

	config.Output = types.StringValue(string(output))

	checksum := sha1.Sum(output)
	config.Id = types.StringValue(hex.EncodeToString(checksum[:]))

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
