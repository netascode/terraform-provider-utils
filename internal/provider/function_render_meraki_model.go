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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zclconf/go-cty/cty"
)

var _ function.Function = RenderMerakiModelFunction{}

func NewRenderMerakiModelFunction() function.Function {
	return &RenderMerakiModelFunction{}
}

type RenderMerakiModelFunction struct{}

func (r RenderMerakiModelFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "render_meraki_model"
}

func (r RenderMerakiModelFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Render a Meraki Network-as-Code model including network and device templates",
		MarkdownDescription: "Processes a Meraki Network as Code model and applies network and device templates. " +
			"YAML strings and the HCL model are merged first, then for every network " +
			"(`meraki.domains[].organizations[].networks[]`) the templates listed in `templates` are looked up in " +
			"`meraki.templates.networks`, rendered and merged in list order, with the network's own configuration " +
			"taking precedence. Afterwards the same is done for every device of every network " +
			"(`networks[].devices[]`, including devices added by network templates) using `meraki.templates.devices`.\n\n" +
			"Templates are either of type `model` (inline `configuration`, where every string value is rendered as a template) " +
			"or of type `file` (a template file referenced by `file`, whose content must be provided via `file_templates`, " +
			"rendered and decoded as YAML).\n\n" +
			"The following variables are available in templates:\n\n" +
			"- Network templates: `GLOBAL` (the `meraki` section of the merged model) and the network's `variables`.\n" +
			"- Device templates: `GLOBAL`, the parent network's `variables` (after network templates have been applied) " +
			"and the device's `variables`, where device variables take precedence over network variables.\n\n" +
			"Only template content is rendered, `${}` expressions in the regular configuration are left untouched. " +
			"Returns an error if the same attribute is a different kind of value (map, list, or scalar) in a template and in the configuration.\n\n" +
			"The result contains the rendered model with `!env` tags preserved (`raw`), the rendered model with `!env` tags resolved (`resolved`) " +
			"and the merged default values (`defaults`). Variables with `!env` tags can be used within strings (e.g. `Bearer ${token}`): " +
			"they are resolved in `resolved` and kept unresolved in `raw`. " +
			"In `raw`, a value consisting only of an `!env` tag is kept as `!env VAR` and written as a YAML tag by `yaml_encode`, " +
			"so that YAML loaders with `!env` support (e.g. `nac-validate`, `nac-test`) resolve it. A tag used within a string is kept as text " +
			"(e.g. `Bearer !env TOKEN`), which these tools do not resolve. If such a value needs to be resolved there, " +
			"use a tag for the whole value instead (e.g. an environment variable containing `Bearer ...`). " +
			"The `meraki.templates` section is not included in the rendered model.\n\n" +
			"~> This function is intended for use within the [Network as Code](https://netascode.cisco.com/) Terraform modules and is not intended for standalone use.\n\n" +
			"## Template Functions\n\n" +
			"The following functions are available inside `${}` template expressions in model and file templates:\n\n" +
			"**Numeric:** `abs`, `ceil`, `floor`, `log`, `max`, `min`, `parseint`, `pow`, `signum`, `sum`\n\n" +
			"**String:** `chomp`, `endswith`, `format`, `formatlist`, `indent`, `join`, `lower`, `regex`, " +
			"`regexall`, `regexreplace`, `replace`, `sort`, `split`, `startswith`, `strcontains`, `strlen`, " +
			"`substr`, `title`, `trim`, `trimprefix`, `trimsuffix`, `trimspace`, `upper`\n\n" +
			"**Collection:** `chunklist`, `coalesce`, `coalescelist`, `compact`, `concat`, `contains`, " +
			"`distinct`, `element`, `flatten`, `index`, `keys`, `length`, `lookup`, `merge`, `one`, " +
			"`range`, `reverse`, `setintersection`, `setproduct`, `setsubtract`, `setunion`, `slice`, " +
			"`values`, `zipmap`\n\n" +
			"**Type Conversion:** `tobool`, `tonumber`, `tostring`\n\n" +
			"**Encoding:** `base64decode`, `base64encode`, `csvdecode`, `jsondecode`, `jsonencode`\n\n" +
			"**Date/Time:** `formatdate`, `timeadd`\n\n" +
			"**Boolean:** `alltrue`, `anytrue`\n\n" +
			"**Error Handling:** `try`, `can`",
		Parameters: []function.Parameter{
			function.ListParameter{
				Name:                "yaml_strings",
				ElementType:         types.StringType,
				MarkdownDescription: "List of YAML strings to decode and merge into the base model.",
			},
			function.DynamicParameter{
				Name:                "model",
				MarkdownDescription: "HCL model variable to merge on top of the YAML strings.",
			},
			function.StringParameter{
				Name:                "defaults_yaml",
				MarkdownDescription: "Module defaults YAML string. User defaults from model override these.",
			},
			function.DynamicParameter{
				Name:                "file_templates",
				MarkdownDescription: "Map of file path to pre-read file content for file-type templates.",
			},
		},
		Return: function.ObjectReturn{
			AttributeTypes: map[string]attr.Type{
				"raw":      types.DynamicType,
				"resolved": types.DynamicType,
				"defaults": types.DynamicType,
			},
		},
	}
}

func (r RenderMerakiModelFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var yamlStrings []string
	var modelDynamic types.Dynamic
	var defaultsYaml string
	var fileTemplatesDynamic types.Dynamic

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &yamlStrings, &modelDynamic, &defaultsYaml, &fileTemplatesDynamic))
	if resp.Error != nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 1. Merge YAML strings, HCL model and defaults
	model, defaults, err := mergeRenderInputs(yamlStrings, modelDynamic, defaultsYaml)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error "+err.Error()))
		return
	}

	// 2. Extract file_templates from Dynamic
	fileTemplates, err := extractFileTemplates(fileTemplatesDynamic)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error "+err.Error()))
		return
	}

	// 3. Produce raw output: apply templates to the model with !env tags preserved
	// (so no secrets end up in the raw output), strip nulls
	rawResult, err := renderMerakiModel(model, fileTemplates, false)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error rendering Meraki model: "+err.Error()))
		return
	}
	rawDynamic, err := convertNativeToDynamic(ctx, stripNulls(rawResult))
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting raw result: "+err.Error()))
		return
	}

	// 4. Produce resolved output: apply templates again with !env tags resolved in
	// variables, so that they can be used within strings (e.g. "Bearer ${token}"),
	// then resolve remaining !env tags, strip nulls
	// (skipped if the model contains no tags, as the result would be identical)
	resolvedResult := rawResult
	if containsYamlTags(model) {
		resolvedResult, err = renderMerakiModel(model, fileTemplates, true)
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error rendering Meraki model: "+resolvedRenderError(err, model).Error()))
			return
		}
	}
	resolvedNative, err := resolveYamlTags(resolvedResult)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error resolving YAML tags: "+err.Error()))
		return
	}
	resolvedDynamic, err := convertNativeToDynamic(ctx, stripNulls(resolvedNative))
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting resolved result: "+err.Error()))
		return
	}

	// 6. Produce defaults output (resolve !env tags of user defaults, strip nulls)
	resolvedDefaults, err := resolveYamlTags(defaults)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error resolving defaults YAML tags: "+err.Error()))
		return
	}
	defaultsDynamic, err := convertNativeToDynamic(ctx, stripNulls(resolvedDefaults))
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting defaults: "+err.Error()))
		return
	}

	// 7. Construct ObjectValue
	objValue, diags := types.ObjectValue(
		map[string]attr.Type{
			"raw":      types.DynamicType,
			"resolved": types.DynamicType,
			"defaults": types.DynamicType,
		},
		map[string]attr.Value{
			"raw":      rawDynamic,
			"resolved": resolvedDynamic,
			"defaults": defaultsDynamic,
		},
	)
	if diags.HasError() {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("failed to construct result object: %s", diags)))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, objValue))
}

// merakiRenderContext holds parsed state for the Meraki rendering pipeline.
type merakiRenderContext struct {
	networkTemplates map[string]map[string]any
	deviceTemplates  map[string]map[string]any
	fileTemplates    map[string]string
	global           cty.Value
	resolveVarTags   bool
}

// renderMerakiModel applies network and device templates to the merged model and
// returns {"meraki": {...}} without the "templates" section. If resolveVarTags is set,
// !env tags in template variables (including GLOBAL) are resolved before rendering,
// leaving tags of unset environment variables unchanged.
func renderMerakiModel(model map[string]any, fileTemplates map[string]string, resolveVarTags bool) (map[string]any, error) {
	meraki := getMapVal(model, "meraki")

	templates := getMapVal(meraki, "templates")
	networkTemplates, err := indexMerakiTemplates(getSliceVal(templates, "networks"))
	if err != nil {
		return nil, fmt.Errorf("network templates: %w", err)
	}
	deviceTemplates, err := indexMerakiTemplates(getSliceVal(templates, "devices"))
	if err != nil {
		return nil, fmt.Errorf("device templates: %w", err)
	}

	var globalVar any = meraki
	if resolveVarTags {
		globalVar = resolveYamlTagsLenient(meraki)
	}
	global, err := nativeToCty(globalVar)
	if err != nil {
		return nil, fmt.Errorf("converting GLOBAL variable: %w", err)
	}

	rctx := &merakiRenderContext{
		networkTemplates: networkTemplates,
		deviceTemplates:  deviceTemplates,
		fileTemplates:    fileTemplates,
		global:           global,
		resolveVarTags:   resolveVarTags,
	}

	result := make(map[string]any, len(meraki))
	for k, v := range meraki {
		if k == "templates" || k == "domains" {
			continue
		}
		result[k] = deepCopy(v)
	}
	if domains, ok := meraki["domains"].([]any); ok {
		renderedDomains := make([]any, len(domains))
		for i, d := range domains {
			domain := toMapStringAny(d)
			if domain == nil {
				renderedDomains[i] = deepCopy(d)
				continue
			}
			rendered, err := renderMerakiDomain(rctx, domain)
			if err != nil {
				return nil, fmt.Errorf("domain %q: %w", getStringVal(domain, "name", ""), err)
			}
			renderedDomains[i] = rendered
		}
		result["domains"] = renderedDomains
	}

	return map[string]any{"meraki": result}, nil
}

// indexMerakiTemplates builds a name -> template map and validates the templates.
func indexMerakiTemplates(list []any) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any, len(list))
	for _, t := range list {
		tmpl := toMapStringAny(t)
		if tmpl == nil {
			continue
		}
		name := getStringVal(tmpl, "name", "")
		if name == "" {
			return nil, fmt.Errorf("template without name")
		}
		if _, exists := result[name]; exists {
			// First definition wins
			continue
		}
		switch t := getStringVal(tmpl, "type", ""); t {
		case "model", "file":
		default:
			return nil, fmt.Errorf("template %q has unsupported type %q, expected 'model' or 'file'", name, t)
		}
		result[name] = tmpl
	}
	return result, nil
}

func renderMerakiDomain(rctx *merakiRenderContext, domain map[string]any) (map[string]any, error) {
	result := copyWithout(domain, "organizations")
	organizations, ok := domain["organizations"].([]any)
	if !ok {
		if v, exists := domain["organizations"]; exists {
			result["organizations"] = deepCopy(v)
		}
		return result, nil
	}
	renderedOrgs := make([]any, len(organizations))
	for i, o := range organizations {
		org := toMapStringAny(o)
		if org == nil {
			renderedOrgs[i] = deepCopy(o)
			continue
		}
		rendered, err := renderMerakiOrganization(rctx, org)
		if err != nil {
			return nil, fmt.Errorf("organization %q: %w", getStringVal(org, "name", ""), err)
		}
		renderedOrgs[i] = rendered
	}
	result["organizations"] = renderedOrgs
	return result, nil
}

func renderMerakiOrganization(rctx *merakiRenderContext, org map[string]any) (map[string]any, error) {
	result := copyWithout(org, "networks")
	networks, ok := org["networks"].([]any)
	if !ok {
		if v, exists := org["networks"]; exists {
			result["networks"] = deepCopy(v)
		}
		return result, nil
	}
	renderedNetworks := make([]any, len(networks))
	for i, n := range networks {
		network := toMapStringAny(n)
		if network == nil {
			renderedNetworks[i] = deepCopy(n)
			continue
		}
		rendered, err := renderMerakiNetwork(rctx, network)
		if err != nil {
			return nil, fmt.Errorf("network %q: %w", getStringVal(network, "name", ""), err)
		}
		renderedNetworks[i] = rendered
	}
	result["networks"] = renderedNetworks
	return result, nil
}

func renderMerakiNetwork(rctx *merakiRenderContext, network map[string]any) (map[string]any, error) {
	// 1. Apply network templates with GLOBAL + network variables
	networkVars := getMapVal(network, "variables")
	ctyVars, err := merakiTemplateVars(rctx, networkVars)
	if err != nil {
		return nil, err
	}
	rendered, err := applyMerakiTemplates(rctx, network, rctx.networkTemplates, ctyVars)
	if err != nil {
		return nil, err
	}

	// 2. Apply device templates with GLOBAL + (rendered) network variables + device variables
	devices, ok := rendered["devices"].([]any)
	if !ok {
		return rendered, nil
	}
	renderedNetworkVars := getMapVal(rendered, "variables")
	for i, d := range devices {
		device := toMapStringAny(d)
		if device == nil {
			continue
		}
		deviceVars := mergeShallow(renderedNetworkVars, getMapVal(device, "variables"))
		ctyVars, err := merakiTemplateVars(rctx, deviceVars)
		if err != nil {
			return nil, fmt.Errorf("device %q: %w", merakiDeviceName(device), err)
		}
		renderedDevice, err := applyMerakiTemplates(rctx, device, rctx.deviceTemplates, ctyVars)
		if err != nil {
			return nil, fmt.Errorf("device %q: %w", merakiDeviceName(device), err)
		}
		devices[i] = renderedDevice
	}
	rendered["devices"] = devices
	return rendered, nil
}

// merakiDeviceName returns the device name or serial for error messages.
func merakiDeviceName(device map[string]any) string {
	if name := getStringVal(device, "name", ""); name != "" {
		return name
	}
	return getStringVal(device, "serial", "")
}

// merakiTemplateVars builds the template variables: GLOBAL plus the given variables,
// where the given variables take precedence.
func merakiTemplateVars(rctx *merakiRenderContext, vars map[string]any) (map[string]cty.Value, error) {
	plain := orderedMapToPlainMap(vars).(map[string]any)
	if rctx.resolveVarTags {
		plain = resolveYamlTagsLenient(plain).(map[string]any)
	}
	ctyVars, err := nativeToCtyMap(plain)
	if err != nil {
		return nil, fmt.Errorf("converting variables: %w", err)
	}
	if _, ok := ctyVars["GLOBAL"]; !ok {
		ctyVars["GLOBAL"] = rctx.global
	}
	return ctyVars, nil
}

// applyMerakiTemplates renders the templates referenced by obj["templates"], merges
// them in list order and merges obj on top. The result is a new plain map, obj is
// not modified.
func applyMerakiTemplates(rctx *merakiRenderContext, obj map[string]any, templates map[string]map[string]any, vars map[string]cty.Value) (map[string]any, error) {
	templateNames := getStringSlice(obj, "templates")
	if len(templateNames) == 0 {
		return deepCopy(obj).(map[string]any), nil
	}

	merged := make(map[string]any)
	for _, name := range templateNames {
		tmpl, ok := templates[name]
		if !ok {
			return nil, fmt.Errorf("template %q not found", name)
		}
		var rendered map[string]any
		switch getStringVal(tmpl, "type", "") {
		case "model":
			config := getMapVal(tmpl, "configuration")
			v, err := renderMerakiTemplateValues(config, vars)
			if err != nil {
				return nil, fmt.Errorf("rendering model template %q: %w", name, err)
			}
			rendered = toMapStringAny(v)
		case "file":
			filePath := getStringVal(tmpl, "file", "")
			content, ok := lookupFileTemplate(rctx.fileTemplates, filePath)
			if !ok {
				return nil, fmt.Errorf("file template %q (path %q) not found in file_templates parameter", name, filePath)
			}
			s, err := renderHCLTemplate(content, vars)
			if err != nil {
				return nil, fmt.Errorf("rendering file template %q: %w", name, err)
			}
			decoded, err := yamlDecode(s)
			if err != nil {
				return nil, fmt.Errorf("decoding file template %q: %w", name, err)
			}
			if decoded == nil {
				continue
			}
			rendered = toMapStringAny(orderedMapToPlainMap(decoded))
			if rendered == nil {
				return nil, fmt.Errorf("file template %q must render to a YAML map", name)
			}
		}
		if rendered == nil {
			continue
		}
		if _, err := MergeMaps(rendered, merged, true); err != nil {
			return nil, fmt.Errorf("template %q: %w", name, err)
		}
	}

	if _, err := MergeMaps(deepCopy(obj), merged, true); err != nil {
		return nil, fmt.Errorf("merging templates: %w", err)
	}
	return orderedMapToPlainMap(merged).(map[string]any), nil
}

// renderMerakiTemplateValues walks a native value tree and renders every string
// value containing a template interpolation (${...}) or directive (%{...}),
// returning a new value tree. Unlike renderTemplateValues, directives such as
// "%{ if x }a%{ else }b%{ endif }" are rendered as well, matching the behaviour
// of rendering the whole template with Terraform's templatestring().
func renderMerakiTemplateValues(v any, vars map[string]cty.Value) (any, error) {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			rendered, err := renderMerakiTemplateValues(item, vars)
			if err != nil {
				return nil, err
			}
			result[k] = rendered
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			rendered, err := renderMerakiTemplateValues(item, vars)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	case string:
		if !strings.Contains(val, "${") && !strings.Contains(val, "%{") {
			return val, nil
		}
		return renderHCLTemplateValue(val, vars)
	default:
		return deepCopy(v), nil
	}
}

// lookupFileTemplate looks up a file template by path, falling back to a
// comparison of cleaned paths (e.g. "./data/t.yaml" matches "data/t.yaml").
func lookupFileTemplate(fileTemplates map[string]string, path string) (string, bool) {
	if content, ok := fileTemplates[path]; ok {
		return content, true
	}
	cleaned := filepath.Clean(path)
	for k, content := range fileTemplates {
		if filepath.Clean(k) == cleaned {
			return content, true
		}
	}
	return "", false
}

// copyWithout returns a deep copy of m without the given key.
func copyWithout(m map[string]any, except string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if k == except {
			continue
		}
		result[k] = deepCopy(v)
	}
	return result
}
