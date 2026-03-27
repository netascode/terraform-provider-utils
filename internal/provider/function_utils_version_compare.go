package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-framework/function"
)

var _ function.Function = VersionCompareFunction{}

func NewVersionCompareFunction() function.Function {
	return &VersionCompareFunction{}
}

type VersionCompareFunction struct{}

func (r VersionCompareFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "version_compare"
}

func (r VersionCompareFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Compare two semantic versions",
		MarkdownDescription: "Compares two semantic version strings and returns an integer: positive if v1 > v2, negative if v1 < v2, or zero if v1 = v2. Supports standard semantic versioning (e.g., '1.2.3', '25.2.2') with optional 'v' prefix, prerelease versions, and metadata.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "v1",
				MarkdownDescription: "First version string to compare (e.g., '1.2.3', 'v25.2.2').",
			},
			function.StringParameter{
				Name:                "v2",
				MarkdownDescription: "Second version string to compare (e.g., '1.2.3', 'v25.2.2').",
			},
		},
		Return: function.Int64Return{},
	}
}

func (r VersionCompareFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var v1String, v2String string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &v1String, &v2String))
	if resp.Error != nil {
		return
	}

	// Strip leading 'v' prefix if present for compatibility
	v1String = strings.TrimPrefix(v1String, "v")
	v2String = strings.TrimPrefix(v2String, "v")

	// Parse first version
	v1, err := version.NewVersion(v1String)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Invalid version string v1: '%s' - %s", v1String, err.Error())))
		return
	}

	// Parse second version
	v2, err := version.NewVersion(v2String)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("Invalid version string v2: '%s' - %s", v2String, err.Error())))
		return
	}

	// Compare versions: returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
	result := int64(v1.Compare(v2))

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
