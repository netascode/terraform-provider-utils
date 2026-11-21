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
			"A template can inherit from other templates of the same kind (network or device) by listing them in its own `templates` " +
			"attribute, where later parents take precedence. The templates referenced by a network or device are expanded with the templates " +
			"they inherit from, and every template is applied once, in an order that keeps the order of the referenced templates and of the " +
			"parents of every template, with parents applied before the templates inheriting from them (C3 linearization). " +
			"If no such order exists, e.g. because a template is referenced after a template inheriting from it, or two templates list " +
			"the same parents in a different order, an error is returned. Inheritance cycles and references to unknown parent templates " +
			"are an error as well. Like other template errors, these errors only occur for templates that are used. " +
			"`templates` and `variables` within the `configuration` of a template (or the content of a file template) are ignored, " +
			"use the `templates` and `variables` attributes of the template instead.\n\n" +
			"A template can define default values for template variables in its own `variables` attribute. The default variables of all " +
			"applied templates (including inherited ones) are merged in application order, so that a template overrides the defaults " +
			"of its parents and of templates applied before it. Default variable values are not rendered as templates.\n\n" +
			"The following variables are available in templates, in increasing order of precedence:\n\n" +
			"- Network templates: `GLOBAL` (the `meraki` section of the merged model), the default variables of the applied network templates " +
			"and the network's `variables`.\n" +
			"- Device templates: `GLOBAL`, the default variables of the network's templates, the default variables of the applied device templates, " +
			"the network's `variables` and the device's `variables`.\n\n" +
			"The `variables` of a rendered network include the default variables of its templates. The `variables` of a rendered device " +
			"include the default variables of its templates that are not overridden by the network's `variables`, so that the variables " +
			"used for the device templates are always the rendered network `variables` merged with the rendered device `variables`.\n\n" +
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
	networkTemplates *merakiTemplateSet
	deviceTemplates  *merakiTemplateSet
	fileTemplates    map[string]string
	resolveVarTags   bool
	globalVar        any // value of the GLOBAL variable (the meraki section, with tags resolved if resolveVarTags)
	// GLOBAL as a cty value, converted at most once per render, when the first
	// network or device with templates is rendered (see globalCtyValue)
	globalCty     cty.Value
	globalCtyErr  error
	globalCtyDone bool
	// globalCtyConversions counts conversions of GLOBAL (for tests)
	globalCtyConversions int
}

// globalCtyValue returns the GLOBAL variable as a cty value, converting it on
// first use only, so that models without templates skip the conversion.
func (rctx *merakiRenderContext) globalCtyValue() (cty.Value, error) {
	if !rctx.globalCtyDone {
		rctx.globalCty, rctx.globalCtyErr = nativeToCty(rctx.globalVar)
		rctx.globalCtyConversions++
		rctx.globalCtyDone = true
	}
	return rctx.globalCty, rctx.globalCtyErr
}

// renderMerakiModel applies network and device templates to the merged model and
// returns {"meraki": {...}} without the "templates" section. If resolveVarTags is set,
// !env tags in template variables (including GLOBAL) are resolved before rendering,
// leaving tags of unset environment variables unchanged.
func renderMerakiModel(model map[string]any, fileTemplates map[string]string, resolveVarTags bool) (map[string]any, error) {
	result, _, err := renderMerakiModelContext(model, fileTemplates, resolveVarTags)
	return result, err
}

// renderMerakiModelContext is renderMerakiModel, also returning the render
// context for inspection in tests.
func renderMerakiModelContext(model map[string]any, fileTemplates map[string]string, resolveVarTags bool) (map[string]any, *merakiRenderContext, error) {
	meraki := getMapVal(model, "meraki")

	templates := getMapVal(meraki, "templates")
	networkTemplates, err := indexMerakiTemplates(getSliceVal(templates, "networks"))
	if err != nil {
		return nil, nil, fmt.Errorf("network templates: %w", err)
	}
	deviceTemplates, err := indexMerakiTemplates(getSliceVal(templates, "devices"))
	if err != nil {
		return nil, nil, fmt.Errorf("device templates: %w", err)
	}

	var globalVar any = meraki
	if resolveVarTags {
		globalVar = resolveYamlTagsLenient(meraki)
	}

	rctx := &merakiRenderContext{
		networkTemplates: networkTemplates,
		deviceTemplates:  deviceTemplates,
		fileTemplates:    fileTemplates,
		resolveVarTags:   resolveVarTags,
		globalVar:        globalVar,
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
				return nil, nil, fmt.Errorf("domain %q: %w", getStringVal(domain, "name", ""), err)
			}
			renderedDomains[i] = rendered
		}
		result["domains"] = renderedDomains
	}

	return map[string]any{"meraki": result}, rctx, nil
}

// merakiTemplateSet holds the network or device templates by name. Inheritance
// is resolved on first use and memoized, so that errors in templates that are
// not used (e.g. an unknown parent template) do not fail the rendering, like
// other template errors (e.g. an undefined variable).
type merakiTemplateSet struct {
	templates map[string]map[string]any
	// precedence maps a template name to its precedence list: the template
	// itself followed by the templates it inherits from, highest precedence first
	precedence map[string][]string
	// expansions memoizes expand by the referenced template names, as many
	// networks or devices typically reference the same templates
	expansions map[string]*merakiExpansion
}

// merakiExpansion holds the templates to apply for the template references of
// a network or device.
type merakiExpansion struct {
	// templates in application order (lowest precedence first)
	templates []string
	// defaultVars are the merged default variables of the templates; shared
	// between networks or devices, so it must not be modified
	defaultVars map[string]any
}

// indexMerakiTemplates builds a name -> template map. Inheritance and the
// template attributes (`templates`, `variables`) are validated when a template
// is used.
func indexMerakiTemplates(list []any) (*merakiTemplateSet, error) {
	templates := make(map[string]map[string]any, len(list))
	for _, t := range list {
		tmpl := toMapStringAny(t)
		if tmpl == nil {
			continue
		}
		name := getStringVal(tmpl, "name", "")
		if name == "" {
			return nil, fmt.Errorf("template without name")
		}
		if _, exists := templates[name]; exists {
			// First definition wins
			continue
		}
		switch t := getStringVal(tmpl, "type", ""); t {
		case "model", "file":
		default:
			return nil, fmt.Errorf("template %q has unsupported type %q, expected 'model' or 'file'", name, t)
		}
		if config, ok := tmpl["configuration"]; ok {
			if stripped, changed := withoutTemplateAttributes(toMapStringAny(config)); changed {
				// Shallow copy, so that the input model is not modified
				tmpl = mergeShallow(tmpl, map[string]any{"configuration": stripped})
			}
		}
		templates[name] = tmpl
	}
	return &merakiTemplateSet{
		templates:  templates,
		precedence: make(map[string][]string),
		expansions: make(map[string]*merakiExpansion),
	}, nil
}

// validateMerakiTemplate checks the inheritance and variable attributes of a template.
func validateMerakiTemplate(tmpl map[string]any) error {
	if v, ok := tmpl["templates"]; ok && v != nil {
		list, ok := v.([]any)
		if !ok {
			return fmt.Errorf("'templates' must be a list of template names")
		}
		for _, item := range list {
			if s, ok := item.(string); !ok || s == "" {
				return fmt.Errorf("'templates' must be a list of template names")
			}
		}
	}
	if v, ok := tmpl["variables"]; ok && v != nil && toMapStringAny(v) == nil {
		return fmt.Errorf("'variables' must be a map")
	}
	return nil
}

// merakiTemplateAttributes are the attributes of a template itself (inheritance
// and default variables). They are ignored within the configuration of a model
// template and the content of a file template, as the network or device schema
// included there allows them, but they would not be applied as expected.
var merakiTemplateAttributes = []string{"templates", "variables"}

// withoutTemplateAttributes returns config without merakiTemplateAttributes and
// whether any of them were removed. config is not modified; if nothing is
// removed, config itself is returned.
func withoutTemplateAttributes(config map[string]any) (map[string]any, bool) {
	changed := false
	for _, key := range merakiTemplateAttributes {
		if _, ok := config[key]; ok {
			changed = true
			break
		}
	}
	if !changed {
		return config, false
	}
	result := make(map[string]any, len(config))
	for k, v := range config {
		result[k] = v
	}
	for _, key := range merakiTemplateAttributes {
		delete(result, key)
	}
	return result, true
}

// expand returns the templates to apply for the given template references
// (later references taking precedence) and their merged default variables.
// Inherited templates are included and every template is applied once, in an
// order that preserves both the order of the references and the order of the
// parents of every template, with parents applied before the templates
// inheriting from them (C3 linearization). If no such order exists, e.g.
// because a template is referenced before one of its parents, an error is
// returned.
func (s *merakiTemplateSet) expand(names []string) (*merakiExpansion, error) {
	key := strings.Join(names, "\x00")
	if e, ok := s.expansions[key]; ok {
		return e, nil
	}
	precedence, err := s.linearize(names, nil, "")
	if err != nil {
		return nil, err
	}
	order := reversedStrings(precedence)
	e := &merakiExpansion{templates: order, defaultVars: s.defaultVariables(order)}
	s.expansions[key] = e
	return e, nil
}

// precedenceOf returns the precedence list of the named template (see
// merakiTemplateSet.precedence). stack holds the templates currently being
// resolved, to detect cycles.
func (s *merakiTemplateSet) precedenceOf(name string, stack []string) ([]string, error) {
	if p, ok := s.precedence[name]; ok {
		return p, nil
	}
	for i, n := range stack {
		if n == name {
			return nil, fmt.Errorf("template inheritance cycle: %s -> %s", strings.Join(stack[i:], " -> "), name)
		}
	}
	tmpl, ok := s.templates[name]
	if !ok {
		if len(stack) == 0 {
			return nil, fmt.Errorf("template %q not found", name)
		}
		return nil, fmt.Errorf("template %q: parent template %q not found", stack[len(stack)-1], name)
	}
	if err := validateMerakiTemplate(tmpl); err != nil {
		return nil, fmt.Errorf("template %q: %w", name, err)
	}
	stack = append(stack[:len(stack):len(stack)], name)
	parents, err := s.linearize(getStringSlice(tmpl, "templates"), stack, name)
	if err != nil {
		return nil, err
	}
	p := append([]string{name}, parents...)
	s.precedence[name] = p
	return p, nil
}

// linearize returns the C3 linearization of the given template references
// (later references taking precedence), highest precedence first. child is the
// template whose parents are linearized, for error messages ("" for the
// references of a network or device).
func (s *merakiTemplateSet) linearize(names []string, stack []string, child string) ([]string, error) {
	names = appendUniqueStrings(nil, make(map[string]bool, len(names)), names...)
	seqs := make([][]string, 0, len(names)+1)
	for i := len(names) - 1; i >= 0; i-- {
		p, err := s.precedenceOf(names[i], stack)
		if err != nil {
			return nil, err
		}
		seqs = append(seqs, p)
	}
	seqs = append(seqs, reversedStrings(names))
	merged, conflicting, ok := c3Merge(seqs)
	if !ok {
		msg := fmt.Sprintf("inconsistent order of templates %s: a template is listed after a template "+
			"inheriting from it, or templates inherit from the same templates in a different order", quoteJoin(conflicting))
		if child != "" {
			return nil, fmt.Errorf("template %q: %s", child, msg)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return merged, nil
}

// c3Merge merges precedence lists (highest precedence first) into one list that
// preserves the order of every input list, taking the first possible head of
// the lists in order (C3 linearization). If no such list exists, it returns
// false and the heads that could not be ordered.
func c3Merge(seqs [][]string) ([]string, []string, bool) {
	var result []string
	for {
		remaining := make([][]string, 0, len(seqs))
		for _, seq := range seqs {
			if len(seq) > 0 {
				remaining = append(remaining, seq)
			}
		}
		if len(remaining) == 0 {
			return result, nil, true
		}
		head, found := "", false
		for _, seq := range remaining {
			if !inAnyTail(seq[0], remaining) {
				head, found = seq[0], true
				break
			}
		}
		if !found {
			var heads []string
			seen := make(map[string]bool)
			for _, seq := range remaining {
				heads = appendUniqueStrings(heads, seen, seq[0])
			}
			return nil, heads, false
		}
		result = append(result, head)
		for i, seq := range remaining {
			if seq[0] == head {
				remaining[i] = seq[1:]
			}
		}
		seqs = remaining
	}
}

// inAnyTail reports whether item is contained in any list of seqs other than
// at its first position.
func inAnyTail(item string, seqs [][]string) bool {
	for _, seq := range seqs {
		for _, s := range seq[1:] {
			if s == item {
				return true
			}
		}
	}
	return false
}

// defaultVariables merges the default variables of the given templates in
// order, later templates taking precedence.
func (s *merakiTemplateSet) defaultVariables(names []string) map[string]any {
	var result map[string]any
	for _, name := range names {
		vars := toMapStringAny(s.templates[name]["variables"])
		if len(vars) == 0 {
			continue
		}
		if result == nil {
			result = make(map[string]any, len(vars))
		}
		mergeShallowInto(result, vars)
	}
	return result
}

// appendUniqueStrings appends the items not yet in seen to list and adds them to seen.
func appendUniqueStrings(list []string, seen map[string]bool, items ...string) []string {
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			list = append(list, item)
		}
	}
	return list
}

// reversedStrings returns a reversed copy of list.
func reversedStrings(list []string) []string {
	result := make([]string, len(list))
	for i, item := range list {
		result[len(list)-1-i] = item
	}
	return result
}

// quoteJoin returns the quoted items separated by commas.
func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
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
	// 1. Apply network templates with GLOBAL + template default variables + network variables
	rendered, networkDefaults, err := applyMerakiTemplates(rctx, network, rctx.networkTemplates, merakiParentVars{})
	if err != nil {
		return nil, err
	}

	// 2. Apply device templates with GLOBAL + network template default variables +
	// device template default variables + network variables + device variables
	devices, ok := rendered["devices"].([]any)
	if !ok {
		return rendered, nil
	}
	parent := merakiParentVars{defaults: networkDefaults, own: getMapVal(network, "variables")}
	for i, d := range devices {
		device := toMapStringAny(d)
		if device == nil {
			continue
		}
		renderedDevice, _, err := applyMerakiTemplates(rctx, device, rctx.deviceTemplates, parent)
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
		global, err := rctx.globalCtyValue()
		if err != nil {
			return nil, fmt.Errorf("converting GLOBAL variable: %w", err)
		}
		ctyVars["GLOBAL"] = global
	}
	return ctyVars, nil
}

// merakiParentVars are the variables of the network a device belongs to.
type merakiParentVars struct {
	// defaults are the default variables of the network's templates, with a
	// lower precedence than the default variables of the device's templates
	defaults map[string]any
	// own are the network's own variables, with a higher precedence than the
	// default variables of the device's templates
	own map[string]any
}

// applyMerakiTemplates renders the templates referenced by obj["templates"]
// (expanded into their inheritance chains), merges them in order and merges obj
// on top. Templates are rendered with the default variables of the parent's
// templates, the default variables of the applied templates, the parent's own
// variables and the variables of obj, in increasing order of precedence. The
// default variables of the applied templates that are not overridden by the
// parent's own variables are added to the variables of the result, so that the
// variables used for rendering are always the rendered variables of the parent
// (if any) merged with the rendered variables of obj. The result is a new plain
// map, obj is not modified. The default variables of the applied templates are
// returned as well; they must not be modified. The variables (incl. GLOBAL) are
// only converted to cty if obj references templates.
func applyMerakiTemplates(rctx *merakiRenderContext, obj map[string]any, set *merakiTemplateSet, parent merakiParentVars) (map[string]any, map[string]any, error) {
	templateNames := getStringSlice(obj, "templates")
	if len(templateNames) == 0 {
		return deepCopy(obj).(map[string]any), nil, nil
	}
	expansion, err := set.expand(templateNames)
	if err != nil {
		return nil, nil, err
	}
	expanded, defaultVars := expansion.templates, expansion.defaultVars
	nativeVars := mergeShallow(mergeShallow(mergeShallow(parent.defaults, defaultVars), parent.own), getMapVal(obj, "variables"))
	vars, err := merakiTemplateVars(rctx, nativeVars)
	if err != nil {
		return nil, nil, err
	}

	merged := make(map[string]any)
	for _, name := range expanded {
		tmpl := set.templates[name]
		var rendered map[string]any
		switch getStringVal(tmpl, "type", "") {
		case "model":
			config := getMapVal(tmpl, "configuration")
			v, err := renderMerakiTemplateValues(config, vars)
			if err != nil {
				return nil, nil, fmt.Errorf("rendering model template %q: %w", name, err)
			}
			rendered = toMapStringAny(v)
		case "file":
			filePath := getStringVal(tmpl, "file", "")
			content, ok := lookupFileTemplate(rctx.fileTemplates, filePath)
			if !ok {
				return nil, nil, fmt.Errorf("file template %q (path %q) not found in file_templates parameter", name, filePath)
			}
			s, err := renderHCLTemplate(content, vars)
			if err != nil {
				return nil, nil, fmt.Errorf("rendering file template %q: %w", name, err)
			}
			decoded, err := yamlDecode(s)
			if err != nil {
				return nil, nil, fmt.Errorf("decoding file template %q: %w", name, err)
			}
			if decoded == nil {
				continue
			}
			rendered = toMapStringAny(orderedMapToPlainMap(decoded))
			if rendered == nil {
				return nil, nil, fmt.Errorf("file template %q must render to a YAML map", name)
			}
			rendered, _ = withoutTemplateAttributes(rendered)
		}
		if rendered == nil {
			continue
		}
		if _, err := MergeMaps(rendered, merged, true); err != nil {
			return nil, nil, fmt.Errorf("template %q: %w", name, err)
		}
	}

	if _, err := MergeMaps(deepCopy(obj), merged, true); err != nil {
		return nil, nil, fmt.Errorf("merging templates: %w", err)
	}
	result := orderedMapToPlainMap(merged).(map[string]any)
	effectiveDefaults := make(map[string]any, len(defaultVars))
	for k, v := range defaultVars {
		if _, overridden := parent.own[k]; !overridden {
			effectiveDefaults[k] = v
		}
	}
	if len(effectiveDefaults) > 0 {
		result["variables"] = mergeShallow(orderedMapToPlainMap(deepCopy(effectiveDefaults)).(map[string]any), getMapVal(result, "variables"))
	}
	return result, defaultVars, nil
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
