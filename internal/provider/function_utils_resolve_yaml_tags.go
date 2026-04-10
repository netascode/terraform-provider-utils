// Copyright © 2022 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Mozilla Public License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mozilla.org/MPL/2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = ResolveYamlTagsFunction{}

func NewResolveYamlTagsFunction() function.Function {
	return &ResolveYamlTagsFunction{}
}

type ResolveYamlTagsFunction struct{}

func (r ResolveYamlTagsFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "resolve_yaml_tags"
}

func (r ResolveYamlTagsFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Resolve YAML tags in a data structure",
		MarkdownDescription: "Recursively walk a data structure and resolve YAML tag strings. Currently supports the `!env VARNAME` tag, which is resolved to the value of the corresponding environment variable. This is intended to be used after `yaml_decode` which preserves unknown YAML tags as literal strings.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:                "input",
				AllowNullValue:      true,
				AllowUnknownValues:  true,
				MarkdownDescription: "The data structure to resolve YAML tags in. Can be any Terraform value type.",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (r ResolveYamlTagsFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var inputDynamic types.Dynamic

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &inputDynamic))
	if resp.Error != nil {
		return
	}

	// Security control: Add timeout protection
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Handle top-level null/unknown
	if inputDynamic.IsNull() || inputDynamic.IsUnknown() {
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, types.DynamicNull()))
		return
	}

	// Convert Terraform Dynamic value to native Go types
	native, err := convertDynamicToNative(inputDynamic)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting input: "+err.Error()))
		return
	}

	// Resolve YAML tags in the native value
	resolved, err := resolveYamlTags(native)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error resolving YAML tags: "+err.Error()))
		return
	}

	// Convert back to Terraform Dynamic type
	result, err := convertNativeToDynamic(ctx, resolved)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting result: "+err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
