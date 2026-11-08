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
		MarkdownDescription: "Encode a given value as a YAML string using the `goccy/go-yaml` library. Produces YAML 1.2 compliant block-style output with 2-space indentation. Map keys are sorted alphabetically, strings that could be misinterpreted are automatically quoted, and null values are rendered as `null`. String values of the form `!env VAR` (e.g. preserved by `render_device_configs` or `render_meraki_model` in their `raw` output) are emitted as YAML `!env` tags (`!env VAR`) instead of quoted strings, so that they are resolved again when the YAML is loaded by a YAML loader with `!env` support (e.g. `nac-yaml`). YAML loaders without `!env` support, such as Terraform's `yamldecode`, cannot read such output. Strings that only contain an `!env` tag as part of the value (e.g. `Bearer !env TOKEN`) are emitted as regular strings.",
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

	// Encode to YAML using goccy/go-yaml, emitting "!env VAR" strings as YAML tags
	result, err := yamlEncode(tagYamlStrings(native))
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error encoding to YAML: "+err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}
