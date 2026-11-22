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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zclconf/go-cty/cty"
)

var _ function.Function = RenderDeviceConfigsFunction{}

func NewRenderDeviceConfigsFunction() function.Function {
	return &RenderDeviceConfigsFunction{}
}

type RenderDeviceConfigsFunction struct{}

func (r RenderDeviceConfigsFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "render_device_configs"
}

func (r RenderDeviceConfigsFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Render per-device configurations from a hierarchical Network-as-Code model",
		MarkdownDescription: "Processes a Network as Code model structure to produce fully rendered per-device configurations. " +
			"Handles template evaluation, deep merging with precedence cascade (global → group → template instance → device), " +
			"interface group merging, and CLI template collection. Supports nxos, iosxe, and iosxr architectures. " +
			"Returns an error if the same attribute is a different kind of value (map, list, or scalar) at different precedence levels; " +
			"scalar values and types may otherwise differ across levels, with the more specific level taking precedence.\n\n" +
			"The `raw` output keeps `!env` tags unresolved, also where variables with `!env` tags are used in templates, " +
			"while the `resolved` output resolves them, so that variables with `!env` tags can be used within strings (e.g. `Bearer ${token}`). " +
			"In the `raw` output, a value consisting only of an `!env` tag is kept as `!env VAR` and written as a YAML tag by `yaml_encode`, " +
			"so that YAML loaders with `!env` support (e.g. `nac-validate`, `nac-test`) resolve it. A tag used within a string is kept as text " +
			"(e.g. `Bearer !env TOKEN`), which these tools do not resolve. If such a value needs to be resolved there, " +
			"use a tag for the whole value instead (e.g. an environment variable containing `Bearer ...`).\n\n" +
			"The names in the `templates` lists of `global`, the device groups and the devices must be templates of type `file`, `model` or `cli`; " +
			"unknown templates and templates of other types are reported as errors, all at once.\n\n" +
			"CLI templates are named after their template: `<template>` for templates of `global` and the device, `<template>/<group>` for device group templates " +
			"and `<template>/instance/<instance>` for template instances. An entry replaces an earlier entry with the same name, " +
			"in increasing order of precedence (`global`, device groups, template instances, device templates, then the device's `cli_templates`), " +
			"e.g. a template assigned both to `global` and to the device is only rendered once, and a device's `cli_templates` entry replaces a template of the same name.\n\n" +
			"~> This function is intended for use within the [Network as Code](https://netascode.cisco.com/) Terraform modules and is not intended for standalone use.\n\n" +
			"## Template Variables\n\n" +
			"The following variables are available inside `${}` template expressions in model templates, " +
			"file templates, CLI templates, configuration values and interface group configurations:\n\n" +
			"- The `variables` of `global`, the device groups the device belongs to and the device, " +
			"in increasing order of precedence. Templates of a device group additionally see that group's own `variables` on top of the device variables.\n" +
			"- `GLOBAL`: the whole architecture section of the merged model (e.g. `GLOBAL.devices`).\n" +
			"- `DEVICES`: all devices of the model in model order, including devices not managed in this run, each with its `name`, " +
			"the names of the `device_groups` it belongs to (devices listing the group and devices listed by the group, in device group order, followed by groups the device lists that are not defined), " +
			"the names of the `template_instances` targeting it (by `devices` or `device_groups`, in instance order, also if all items of an instance are narrowed to other devices) " +
			"and its `variables` (the variables of `global`, its device groups and the device, merged as for the device's own templates). " +
			"This allows templates to refer to other devices, e.g. " +
			"`[for d in DEVICES : d.variables.mgmt_ip if contains(d.device_groups, \"VPC\") && d.name != DEVICE.name]`.\n" +
			"- `DEVICE`: the device being rendered, i.e. its element of `DEVICES` " +
			"(e.g. `DEVICE.name`, `contains(DEVICE.device_groups, \"LEAFS\")`, `contains(DEVICE.template_instances, \"SERVICE_PROD_WEB\")`, `DEVICE.variables.leaf_id`). This allows a single template attached to a device group or `global` " +
			"to select the entries of a shared list that apply to the device, e.g. " +
			"`%{ for e in endpoints ~}%{ if contains(e.switches, DEVICE.name) ~}...%{ endif ~}%{ endfor ~}` in a file template.\n\n" +
			"A user variable named `GLOBAL`, `DEVICES` or `DEVICE` takes precedence over the built-in variable of the same name.\n\n" +
			"## Template Instances\n\n" +
			"A template instance (`<arch>.template_instances`) applies a template to a set of devices with its own variables, " +
			"e.g. one instance per service, without a device group per service:\n\n" +
			"- `name` (required, unique, without `/`) identifies the instance in error messages and in the names of CLI templates (`<template>/instance/<instance>`, " +
			"which differ from the names of device group CLI templates, `<template>/<group>`, also if a device group has the same name).\n" +
			"- `template` (required) is the name of a `file`, `model` or `cli` template in `<arch>.templates`.\n" +
			"- `devices` and `device_groups` select the devices the instance applies to (at least one is required). " +
			"Device groups include devices listing the group and devices listed by the group.\n" +
			"- `variables` are only visible to this instance. Its template sees the variables of `global`, the device groups and the device " +
			"(as other templates do), then the instance `variables`, in increasing order of precedence, plus `GLOBAL`, `DEVICES` and `DEVICE`.\n" +
			"- `items` renders the template once per item, each with its own `variables` on top of the instance `variables`. " +
			"An instance without `items` (or with `items` set to null) renders the template once, `items: []` not at all. " +
			"An item's `devices` and `device_groups` narrow the devices of the instance: devices must be devices of the instance, " +
			"device groups select the devices of the instance that are members. " +
			"An item's optional `name` (unique within the instance, without `/`) names its CLI templates `<template>/instance/<instance>/<name>`, " +
			"which do not change when items are added or reordered; CLI templates of items without a name are named `<template>/instance/<instance>/items[<index>]`.\n\n" +
			"Unknown attributes, templates, devices and device groups, values of the wrong type, duplicate names " +
			"and devices of an item outside its instance are reported as errors, all at once. " +
			"The configurations of template instances are merged after the device group configurations and before the device templates, " +
			"in instance and item order. Template instances (or items of one instance) setting a value in a map to different values " +
			"on a device are reported as errors; equal values (e.g. a feature enabled by several instances) are allowed. " +
			"List items are merged by their values, so instances adding the same list item (e.g. a VLAN ID) with different values " +
			"result in two list items, which fails when the configuration is applied (e.g. as a duplicate key in the Terraform module).\n\n" +
			"## Template Functions\n\n" +
			"The following functions are available inside `${}` template expressions in model templates, " +
			"file templates, and CLI templates:\n\n" +
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
			"**Error Handling:** `try`, `can`, `fail`\n\n" +
			"`fail(message)` stops rendering with the message as error, e.g. `${contains(vlans, vlan) ? vlan : fail(\"VLAN ${vlan} is not defined\")}`. " +
			"Only the selected branch of a conditional (or `%{ if }` directive) is evaluated, and `try` and `can` catch the error.\n\n" +
			"**Network:** `parse_vlans`, `expand_interfaces`\n\n" +
			"`parse_vlans(vlans)` converts VLANs in CLI notation (a number, a string such as `\"10,20-29\"`, or a list of them) " +
			"to the `ids` and `ranges` object the device data models use for VLAN lists, e.g. `parse_vlans([10, \"20-29\"])` returns " +
			"`{ids = [10], ranges = [{from = 20, to = 29}]}`, " +
			"for use as `trunk_allowed_vlans: ${jsonencode(parse_vlans(vlans))}` (nxos) or `trunk_allowed_vlans: {vlans: ${jsonencode(parse_vlans(vlans))}}` (iosxe). " +
			"An empty list returns an empty object and null returns null. " +
			"Invalid VLANs (other than IDs from 1 to 4094 and ranges of them, e.g. `all`) are reported as errors.\n\n" +
			"`expand_interfaces(interfaces)` expands interface ranges in CLI notation (a string such as `\"1/10,1/20-23\"`, or a list of them) " +
			"to a list of interface IDs, e.g. `[\"1/10\", \"1/20\", \"1/21\", \"1/22\", \"1/23\"]`, in the given order. A range applies to the last number of an ID " +
			"(e.g. `1/1/1-4` for breakout ports, `10-12` for port-channels) and covers at most 1024 interfaces.",
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
			function.ListParameter{
				Name:                "managed_devices",
				ElementType:         types.StringType,
				MarkdownDescription: "List of device names to manage. Empty list means all devices.",
			},
			function.ListParameter{
				Name:                "managed_device_groups",
				ElementType:         types.StringType,
				MarkdownDescription: "List of device group names to manage. Empty list means all device groups.",
			},
		},
		Return: function.ObjectReturn{
			AttributeTypes: map[string]attr.Type{
				"raw":              types.DynamicType,
				"resolved":         types.DynamicType,
				"provider_devices": types.DynamicType,
			},
		},
	}
}

func (r RenderDeviceConfigsFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var yamlStrings []string
	var modelDynamic types.Dynamic
	var defaultsYaml string
	var fileTemplatesDynamic types.Dynamic
	var managedDevicesTF []string
	var managedGroupsTF []string

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &yamlStrings, &modelDynamic, &defaultsYaml, &fileTemplatesDynamic, &managedDevicesTF, &managedGroupsTF))
	if resp.Error != nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 1-4. Merge YAML strings, HCL model and defaults
	model, defaults, err := mergeRenderInputs(yamlStrings, modelDynamic, defaultsYaml)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error "+err.Error()))
		return
	}

	// 5. Extract file_templates from Dynamic
	fileTemplates, err := extractFileTemplates(fileTemplatesDynamic)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error "+err.Error()))
		return
	}

	// 6. Discover architecture and build provider_devices
	arch, err := discoverArchitecture(model)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error discovering architecture: "+err.Error()))
		return
	}
	defaultManaged := getBoolVal(getMapVal(getMapVal(defaults, arch), "devices"), "managed", true)
	providerDevices := buildProviderDevices(model, arch, defaultManaged)

	// 7. Run the render pipeline with !env tags preserved in variables, so that
	// no secrets end up in the raw output
	result, err := renderDeviceConfigs(model, fileTemplates, managedDevicesTF, managedGroupsTF, defaults, false)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error rendering device configs: "+err.Error()))
		return
	}

	// 8. Produce raw output (strip nulls, preserve !env tags)
	rawResult := stripNulls(result)
	rawDynamic, err := convertNativeToDynamic(ctx, rawResult)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting raw result: "+err.Error()))
		return
	}

	// 9. Produce resolved output: run the render pipeline again with !env tags
	// resolved in variables, so that they can be used within strings
	// (e.g. "Bearer ${token}"), then resolve remaining !env tags, strip nulls
	// (skipped if the model contains no tags, as the result would be identical)
	resolvedRender := result
	if containsYamlTags(model) {
		resolvedRender, err = renderDeviceConfigs(model, fileTemplates, managedDevicesTF, managedGroupsTF, defaults, true)
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error rendering device configs: "+resolvedRenderError(err, model).Error()))
			return
		}
	}
	resolvedNative, err := resolveYamlTags(resolvedRender)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error resolving YAML tags: "+err.Error()))
		return
	}
	resolvedResult := stripNulls(resolvedNative)
	resolvedDynamic, err := convertNativeToDynamic(ctx, resolvedResult)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting resolved result: "+err.Error()))
		return
	}

	// 10. Produce provider_devices output
	pdResult := stripNulls(providerDevices)
	pdDynamic, err := convertNativeToDynamic(ctx, pdResult)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting provider_devices: "+err.Error()))
		return
	}

	// 11. Construct ObjectValue
	objValue, diags := types.ObjectValue(
		map[string]attr.Type{
			"raw":              types.DynamicType,
			"resolved":         types.DynamicType,
			"provider_devices": types.DynamicType,
		},
		map[string]attr.Value{
			"raw":              rawDynamic,
			"resolved":         resolvedDynamic,
			"provider_devices": pdDynamic,
		},
	)
	if diags.HasError() {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("failed to construct result object: %s", diags)))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, objValue))
}

// renderContext holds parsed state for the rendering pipeline.
type renderContext struct {
	arch            string
	archConfig      map[string]any
	global          map[string]any
	devices         []any
	deviceGroups    []any
	interfaceGroups []any
	templates       map[string]map[string]any
	instances       []templateInstance // parsed <arch>.template_instances
	fileTemplates   map[string]string
	defaultOrder    int
	defaultStage    string
	defaultManaged  bool
	defaultConfig   map[string]any // defaults[arch].devices.configuration
	// resolveVarTags resolves !env tags in template variables (including GLOBAL)
	// before rendering, so that they can be used within strings (e.g. "Bearer ${token}").
	resolveVarTags bool
	globalVar      any // value of the GLOBAL variable (archConfig, with tags resolved if resolveVarTags)
	// GLOBAL as a cty value, converted at most once per render on first use (see globalCtyValue)
	globalCty     cty.Value
	globalCtyErr  error
	globalCtyDone bool
	// globalCtyConversions counts conversions of GLOBAL (for tests)
	globalCtyConversions int
	// DEVICES as a cty value, converted at most once per render on first use (see devicesCtyValue)
	devicesCty     cty.Value
	devicesCtyErr  error
	devicesCtyDone bool
	// devicesCtyConversions counts conversions of DEVICES (for tests)
	devicesCtyConversions int
	// deviceIndex maps device names to their index in DEVICES (set by devicesCtyValue)
	deviceIndex map[string]int
}

// globalCtyValue returns the GLOBAL variable (the whole architecture config) as
// a cty value. It is converted once per render, on first use, and shared by all
// devices and device groups: cty values are immutable, and converting the whole
// model per device made rendering time grow with devices × model size.
func (rctx *renderContext) globalCtyValue() (cty.Value, error) {
	if !rctx.globalCtyDone {
		rctx.globalCty, rctx.globalCtyErr = nativeToCty(rctx.globalVar)
		rctx.globalCtyDone = true
		rctx.globalCtyConversions++
	}
	return rctx.globalCty, rctx.globalCtyErr
}

// devicesNative returns the DEVICES variable: all devices of the model (also
// devices not managed in this run), in model order, each with its name, the
// names of the device groups it belongs to and its variables (global, device
// group and device variables, as seen by its device templates). The device
// groups are the defined device groups it belongs to (in device group order,
// membership in either direction), followed by the device groups it lists that
// are not defined (in the order listed), as in GLOBAL. It also sets deviceIndex.
func (rctx *renderContext) devicesNative() []any {
	result := make([]any, 0, len(rctx.devices))
	rctx.deviceIndex = make(map[string]int, len(rctx.devices))
	for _, deviceRaw := range rctx.devices {
		device, ok := deviceRaw.(map[string]any)
		if !ok {
			continue
		}
		name := getStringVal(device, "name", "")
		groups := []any{}
		seen := map[string]bool{}
		for _, dgRaw := range rctx.deviceGroups {
			if dg, ok := dgRaw.(map[string]any); ok && deviceMatchesGroup(device, dg) {
				dgName := getStringVal(dg, "name", "")
				if !seen[dgName] {
					seen[dgName] = true
					groups = append(groups, dgName)
				}
			}
		}
		for _, dgName := range getStringSlice(device, "device_groups") {
			if !seen[dgName] {
				seen[dgName] = true
				groups = append(groups, dgName)
			}
		}
		var variables any = buildDeviceVariables(rctx, device)
		if rctx.resolveVarTags {
			variables = resolveYamlTagsLenient(variables)
		}
		// Template instances targeting the device (also if all of their items
		// are narrowed to other devices), in instance order
		instances := []any{}
		for _, inst := range rctx.instances {
			if inst.targets[name] {
				instances = append(instances, inst.name)
			}
		}
		if _, dup := rctx.deviceIndex[name]; !dup {
			rctx.deviceIndex[name] = len(result)
		}
		result = append(result, map[string]any{
			"name":               name,
			"device_groups":      groups,
			"template_instances": instances,
			"variables":          variables,
		})
	}
	return result
}

// devicesCtyValue returns the DEVICES variable as a cty value. Like GLOBAL, it
// is converted once per render, on first use, and shared by all devices; the
// DEVICE variable of each device is its element of DEVICES. A conversion error
// names the device whose entry could not be converted.
func (rctx *renderContext) devicesCtyValue() (cty.Value, error) {
	if !rctx.devicesCtyDone {
		rctx.devicesCty, rctx.devicesCtyErr = devicesToCty(rctx.devicesNative())
		rctx.devicesCtyDone = true
		rctx.devicesCtyConversions++
	}
	return rctx.devicesCty, rctx.devicesCtyErr
}

// devicesToCty converts the native DEVICES variable to a cty tuple, naming the
// device of an element that cannot be converted.
func devicesToCty(devices []any) (cty.Value, error) {
	if len(devices) == 0 {
		return cty.EmptyTupleVal, nil
	}
	elems := make([]cty.Value, len(devices))
	for i, d := range devices {
		v, err := nativeToCty(d)
		if err != nil {
			return cty.NilVal, fmt.Errorf("device %q: %w", getStringVal(toMapStringAny(d), "name", ""), err)
		}
		elems[i] = v
	}
	return cty.TupleVal(elems), nil
}

// varsToCty converts template variables to cty values and adds GLOBAL, DEVICES
// and DEVICE (the element of DEVICES of the named device, if device is set),
// unless a user variable of the same name takes precedence. If resolveVarTags
// is set, !env tags in all variables except GLOBAL (which is resolved once in
// extractRenderContext) are resolved, leaving tags of unset environment
// variables unchanged.
func (rctx *renderContext) varsToCty(vars map[string]any, device string) (map[string]cty.Value, error) {
	result, err := rctx.userVarsToCty(vars)
	if err != nil {
		return nil, err
	}
	if _, ok := result["GLOBAL"]; !ok {
		global, err := rctx.globalCtyValue()
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", "GLOBAL", err)
		}
		result["GLOBAL"] = global
	}
	_, userDevices := result["DEVICES"]
	_, userDevice := result["DEVICE"]
	if !userDevices || (!userDevice && device != "") {
		devices, err := rctx.devicesCtyValue()
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", "DEVICES", err)
		}
		if !userDevices {
			result["DEVICES"] = devices
		}
		if i, ok := rctx.deviceIndex[device]; ok && !userDevice && device != "" {
			result["DEVICE"] = devices.Index(cty.NumberIntVal(int64(i)))
		}
	}
	return result, nil
}

// userVarsToCty converts user variables to cty values, resolving !env tags in
// all variables except GLOBAL if resolveVarTags is set.
func (rctx *renderContext) userVarsToCty(vars map[string]any) (map[string]cty.Value, error) {
	if rctx.resolveVarTags {
		resolved := make(map[string]any, len(vars))
		for k, v := range vars {
			if k == "GLOBAL" {
				resolved[k] = v
				continue
			}
			resolved[k] = resolveYamlTagsLenient(v)
		}
		vars = resolved
	}
	return nativeToCtyMap(vars)
}

// overlayVarsToCty returns the converted variables of base with the overlay
// variables on top, converting only the overlay variables. User variables of
// the overlay take precedence over the built-in variables of base.
func (rctx *renderContext) overlayVarsToCty(base *templateVars, overlay map[string]any) (map[string]cty.Value, error) {
	baseValues, err := base.get()
	if err != nil {
		return nil, err
	}
	overlayValues, err := rctx.userVarsToCty(overlay)
	if err != nil {
		return nil, err
	}
	result := make(map[string]cty.Value, len(baseValues)+len(overlayValues))
	for k, v := range baseValues {
		result[k] = v
	}
	for k, v := range overlayValues {
		result[k] = v
	}
	return result, nil
}

// templateVars holds the template variables of a device or device group and
// converts them to cty on first use, so that devices without templates or
// ${...} expressions skip the conversion entirely, and each device group's
// variables are converted once per device.
type templateVars struct {
	rctx   *renderContext
	label  string // error context, e.g. `group "leafs" vars`
	native map[string]any
	device string        // name of the device rendered, for the DEVICE variable (not added if empty)
	base   *templateVars // if set, native are overlay variables on top of the variables of base
	values map[string]cty.Value
	err    error
	done   bool
}

func newTemplateVars(rctx *renderContext, label string, native map[string]any, device string) *templateVars {
	return &templateVars{rctx: rctx, label: label, native: native, device: device}
}

func (v *templateVars) get() (map[string]cty.Value, error) {
	if !v.done {
		if v.base != nil {
			v.values, v.err = v.rctx.overlayVarsToCty(v.base, v.native)
		} else {
			v.values, v.err = v.rctx.varsToCty(v.native, v.device)
		}
		if v.err != nil {
			v.err = fmt.Errorf("%s: %w", v.label, v.err)
		}
		v.done = true
	}
	return v.values, v.err
}

// matchedGroup is a device group the device belongs to, with its template variables.
type matchedGroup struct {
	group map[string]any
	name  string
	vars  *templateVars
}

// renderDeviceConfigs is the core pipeline.
// If resolveVarTags is set, !env tags in template variables are resolved before rendering.
func renderDeviceConfigs(model map[string]any, fileTemplates map[string]string, managedDevices, managedGroups []string, defaults map[string]any, resolveVarTags bool) (map[string]any, error) {
	// 1. Discover architecture
	arch, err := discoverArchitecture(model)
	if err != nil {
		return nil, err
	}

	// 2. Extract context
	rctx, err := extractRenderContext(model, arch, fileTemplates, defaults, resolveVarTags)
	if err != nil {
		return nil, err
	}

	// 3. Filter managed devices
	managed := filterManagedDevices(rctx, managedDevices, managedGroups)

	// 4. Process each device
	renderedDevices := make([]any, 0, len(managed))
	for _, deviceRaw := range managed {
		device, ok := deviceRaw.(map[string]any)
		if !ok {
			continue
		}

		rendered, err := renderSingleDevice(rctx, device)
		if err != nil {
			return nil, fmt.Errorf("device %v: %w", getStringVal(device, "name", ""), err)
		}
		renderedDevices = append(renderedDevices, rendered)
	}

	// 5. Return result
	return map[string]any{
		arch: map[string]any{
			"devices": renderedDevices,
		},
	}, nil
}

func discoverArchitecture(model map[string]any) (string, error) {
	var arch string
	for k := range model {
		if k == "defaults" {
			continue
		}
		if arch != "" {
			return "", fmt.Errorf("multiple architecture keys found: %q and %q", arch, k)
		}
		arch = k
	}
	if arch == "" {
		return "", fmt.Errorf("no architecture key found in model (only 'defaults' present)")
	}
	return arch, nil
}

// validateTemplateReferences checks the templates lists of global, the device
// groups and the devices: each name must be a template of type file, model or
// cli, as a misspelled name would otherwise not apply the template without
// notice. All problems are returned at once, each with the path of the list.
func validateTemplateReferences(archConfig map[string]any, templates map[string]map[string]any) error {
	var errs []error
	check := func(path string, owner map[string]any) {
		for _, name := range getStringSlice(owner, "templates") {
			tmpl, ok := templates[name]
			if !ok {
				errs = append(errs, fmt.Errorf("%s: unknown template %q", path, name))
				continue
			}
			switch t := getStringVal(tmpl, "type", ""); t {
			case "file", "model", "cli":
			default:
				errs = append(errs, fmt.Errorf("%s: template %q has unsupported type %q", path, name, t))
			}
		}
	}
	check("global.templates", getMapVal(archConfig, "global"))
	for i, dgRaw := range getSliceVal(archConfig, "device_groups") {
		if dg := toMapStringAny(dgRaw); dg != nil {
			check(fmt.Sprintf("device_groups[%s].templates", listEntryName(dg, i)), dg)
		}
	}
	for i, dRaw := range getSliceVal(archConfig, "devices") {
		if d := toMapStringAny(dRaw); d != nil {
			check(fmt.Sprintf("devices[%s].templates", listEntryName(d, i)), d)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid template references:\n%w", errors.Join(errs...))
	}
	return nil
}

// listEntryName returns the name of a list entry, or its index if it has none.
func listEntryName(m map[string]any, index int) string {
	if name := getStringVal(m, "name", ""); name != "" {
		return name
	}
	return fmt.Sprint(index)
}

func extractRenderContext(model map[string]any, arch string, fileTemplates map[string]string, defaults map[string]any, resolveVarTags bool) (*renderContext, error) {
	archConfig := getMapVal(model, arch)
	var globalVar any = archConfig
	if resolveVarTags {
		globalVar = resolveYamlTagsLenient(archConfig)
	}
	defaultsArch := getMapVal(defaults, arch)

	// Parse templates into lookup map
	templatesList := getSliceVal(archConfig, "templates")
	templates := make(map[string]map[string]any, len(templatesList))
	for _, t := range templatesList {
		if tm, ok := t.(map[string]any); ok {
			if name, ok := tm["name"].(string); ok {
				templates[name] = tm
			}
		}
	}

	devices := getSliceVal(archConfig, "devices")
	deviceGroups := getSliceVal(archConfig, "device_groups")
	refErr := validateTemplateReferences(archConfig, templates)
	instances, err := parseTemplateInstances(archConfig, templates, devices, deviceGroups)
	if err := errors.Join(refErr, err); err != nil {
		return nil, err
	}

	return &renderContext{
		arch:            arch,
		archConfig:      archConfig,
		global:          getMapVal(archConfig, "global"),
		devices:         devices,
		deviceGroups:    deviceGroups,
		instances:       instances,
		interfaceGroups: getSliceVal(archConfig, "interface_groups"),
		templates:       templates,
		fileTemplates:   fileTemplates,
		defaultOrder:    getIntVal(getMapVal(defaultsArch, "templates"), "order", 0),
		defaultStage:    getStringVal(getMapVal(defaultsArch, "templates"), "stage", "post"),
		defaultManaged:  getBoolVal(getMapVal(defaultsArch, "devices"), "managed", true),
		defaultConfig:   getMapVal(getMapVal(defaultsArch, "devices"), "configuration"),
		resolveVarTags:  resolveVarTags,
		globalVar:       globalVar,
	}, nil
}

func filterManagedDevices(rctx *renderContext, managedDevices, managedGroups []string) []any {
	if len(managedDevices) == 0 && len(managedGroups) == 0 {
		return rctx.devices
	}

	managedDeviceSet := toStringSet(managedDevices)
	managedGroupSet := toStringSet(managedGroups)

	var result []any
	for _, deviceRaw := range rctx.devices {
		device, ok := deviceRaw.(map[string]any)
		if !ok {
			continue
		}
		name := getStringVal(device, "name", "")

		// Check managed_devices filter
		if len(managedDeviceSet) > 0 && !managedDeviceSet[name] {
			continue
		}

		// Check managed_device_groups filter
		if len(managedGroupSet) > 0 {
			if !deviceInAnyManagedGroup(device, rctx.deviceGroups, managedGroupSet) {
				continue
			}
		}

		result = append(result, deviceRaw)
	}
	return result
}

func deviceInAnyManagedGroup(device map[string]any, deviceGroups []any, managedGroupSet map[string]bool) bool {
	deviceName := getStringVal(device, "name", "")
	deviceGroupNames := getStringSlice(device, "device_groups")

	for _, dgRaw := range deviceGroups {
		dg, ok := dgRaw.(map[string]any)
		if !ok {
			continue
		}
		dgName := getStringVal(dg, "name", "")
		if !managedGroupSet[dgName] {
			continue
		}
		// Bidirectional: device lists the group OR group lists the device
		if containsString(deviceGroupNames, dgName) {
			return true
		}
		if containsString(getStringSlice(dg, "devices"), deviceName) {
			return true
		}
	}
	return false
}

func deviceMatchesGroup(device map[string]any, group map[string]any) bool {
	deviceName := getStringVal(device, "name", "")
	dgName := getStringVal(group, "name", "")
	deviceGroupNames := getStringSlice(device, "device_groups")
	groupDeviceNames := getStringSlice(group, "devices")
	return containsString(deviceGroupNames, dgName) || containsString(groupDeviceNames, deviceName)
}

// renderSingleDevice processes one device through the full pipeline.
func renderSingleDevice(rctx *renderContext, device map[string]any) (map[string]any, error) {
	deviceName := getStringVal(device, "name", "")

	// 4a. Build device variables (shallow merge), converted to cty on first use
	deviceVars := buildDeviceVariables(rctx, device)

	ctyVars := newTemplateVars(rctx, "converting variables", deviceVars, deviceName)

	// Device groups of this device, each with its variables on top of the device variables
	var groups []matchedGroup
	for _, dgRaw := range rctx.deviceGroups {
		dg, ok := dgRaw.(map[string]any)
		if !ok || !deviceMatchesGroup(device, dg) {
			continue
		}
		name := getStringVal(dg, "name", "")
		groupVars := mergeShallow(deviceVars, getMapVal(dg, "variables"))
		groups = append(groups, matchedGroup{group: dg, name: name, vars: newTemplateVars(rctx, fmt.Sprintf("group %q vars", name), groupVars, deviceName)})
	}

	// 4b. Process file templates
	globalFileTmpls, err := processFileTemplates(rctx, getStringSlice(rctx.global, "templates"), ctyVars)
	if err != nil {
		return nil, fmt.Errorf("global file templates: %w", err)
	}

	var groupFileTmpls []map[string]any
	var groupModelTmpls []map[string]any
	var groupConfigs []map[string]any
	for _, g := range groups {
		// Group file templates with extra group variables
		ft, err := processFileTemplates(rctx, getStringSlice(g.group, "templates"), g.vars)
		if err != nil {
			return nil, fmt.Errorf("group %q file templates: %w", g.name, err)
		}
		groupFileTmpls = append(groupFileTmpls, ft...)

		// Group model templates
		mt, err := processModelTemplates(rctx, getStringSlice(g.group, "templates"), g.vars)
		if err != nil {
			return nil, fmt.Errorf("group %q model templates: %w", g.name, err)
		}
		groupModelTmpls = append(groupModelTmpls, mt)

		// Group configuration
		if cfg := getMapVal(g.group, "configuration"); len(cfg) > 0 {
			groupConfigs = append(groupConfigs, cfg)
		}
	}

	// Template instances that apply to the device
	instanceConfigs, instanceCli, err := renderTemplateInstances(rctx, deviceName, ctyVars)
	if err != nil {
		return nil, err
	}

	deviceFileTmpls, err := processFileTemplates(rctx, getStringSlice(device, "templates"), ctyVars)
	if err != nil {
		return nil, fmt.Errorf("device file templates: %w", err)
	}

	// 4c. Process model templates
	globalModelTmpl, err := processModelTemplates(rctx, getStringSlice(rctx.global, "templates"), ctyVars)
	if err != nil {
		return nil, fmt.Errorf("global model templates: %w", err)
	}

	deviceModelTmpl, err := processModelTemplates(rctx, getStringSlice(device, "templates"), ctyVars)
	if err != nil {
		return nil, fmt.Errorf("device model templates: %w", err)
	}

	// 4d. 10-level precedence cascade
	merged := make(map[string]any)
	// 1. Global file templates
	for _, ft := range globalFileTmpls {
		if _, err := MergeMaps(ft, merged, true); err != nil {
			return nil, fmt.Errorf("global file templates: %w", err)
		}
	}
	// 2. Global model templates
	if _, err := MergeMaps(globalModelTmpl, merged, true); err != nil {
		return nil, fmt.Errorf("global model templates: %w", err)
	}
	// 3. Global configuration
	if _, err := MergeMaps(deepCopy(getMapVal(rctx.global, "configuration")), merged, true); err != nil {
		return nil, fmt.Errorf("global configuration: %w", err)
	}
	// 4. Group file templates
	for _, ft := range groupFileTmpls {
		if _, err := MergeMaps(ft, merged, true); err != nil {
			return nil, fmt.Errorf("group file templates: %w", err)
		}
	}
	// 5. Group model templates
	for _, mt := range groupModelTmpls {
		if _, err := MergeMaps(mt, merged, true); err != nil {
			return nil, fmt.Errorf("group model templates: %w", err)
		}
	}
	// 6. Group configurations
	for _, gc := range groupConfigs {
		if _, err := MergeMaps(deepCopy(gc), merged, true); err != nil {
			return nil, fmt.Errorf("group configuration: %w", err)
		}
	}
	// 7. Template instances
	if err := checkInstanceConflicts(instanceConfigs); err != nil {
		return nil, err
	}
	for _, ic := range instanceConfigs {
		if _, err := MergeMaps(ic.config, merged, true); err != nil {
			return nil, fmt.Errorf("template instances: %w", err)
		}
	}
	// 8. Device file templates
	for _, ft := range deviceFileTmpls {
		if _, err := MergeMaps(ft, merged, true); err != nil {
			return nil, fmt.Errorf("device file templates: %w", err)
		}
	}
	// 9. Device model templates
	if _, err := MergeMaps(deviceModelTmpl, merged, true); err != nil {
		return nil, fmt.Errorf("device model templates: %w", err)
	}
	// 10. Device configuration
	if _, err := MergeMaps(deepCopy(getMapVal(device, "configuration")), merged, true); err != nil {
		return nil, fmt.Errorf("device configuration: %w", err)
	}

	// 4e. Final template pass
	merged, err = templatePassOnMap(merged, ctyVars)
	if err != nil {
		return nil, fmt.Errorf("final template pass: %w", err)
	}

	// 4e2. Apply defaults as fallbacks
	if len(rctx.defaultConfig) > 0 {
		applyDefaults(merged, rctx.defaultConfig)
	}

	// 4f. Interface groups
	igConfigs, err := resolveInterfaceGroupConfigs(rctx, ctyVars)
	if err != nil {
		return nil, fmt.Errorf("interface groups: %w", err)
	}
	if err := applyInterfaceGroups(merged, igConfigs); err != nil {
		return nil, fmt.Errorf("interface groups: %w", err)
	}

	// 4g. CLI templates
	cliTemplates, err := collectCliTemplates(rctx, device, ctyVars, groups, instanceCli)
	if err != nil {
		return nil, fmt.Errorf("cli templates: %w", err)
	}

	// 4h. Assemble output
	output := map[string]any{
		"name":          deviceName,
		"managed":       getBoolVal(device, "managed", rctx.defaultManaged),
		"configuration": merged,
		"cli_templates": cliTemplates,
	}
	// Copy optional metadata fields (omit if not present)
	for _, field := range []string{"url", "host", "protocol"} {
		if v, ok := device[field]; ok && v != nil {
			output[field] = v
		}
	}

	return output, nil
}

// buildDeviceVariables merges global, device group and device variables. GLOBAL
// is not part of the result: varsToCty adds it as a cty value converted once per
// render (a user variable named GLOBAL takes precedence).
func buildDeviceVariables(rctx *renderContext, device map[string]any) map[string]any {
	result := map[string]any{}
	// Global variables
	mergeShallowInto(result, getMapVal(rctx.global, "variables"))
	// Device group variables (in order)
	for _, dgRaw := range rctx.deviceGroups {
		dg, ok := dgRaw.(map[string]any)
		if !ok || !deviceMatchesGroup(device, dg) {
			continue
		}
		mergeShallowInto(result, getMapVal(dg, "variables"))
	}
	// Device variables
	mergeShallowInto(result, getMapVal(device, "variables"))
	return result
}

func processFileTemplates(rctx *renderContext, templateNames []string, vars *templateVars) ([]map[string]any, error) {
	var results []map[string]any
	for _, name := range templateNames {
		tmpl, ok := rctx.templates[name]
		if !ok || getStringVal(tmpl, "type", "") != "file" {
			continue
		}
		filePath := getStringVal(tmpl, "file", "")
		content, ok := rctx.fileTemplates[filePath]
		if !ok {
			return nil, fmt.Errorf("file template %q (path %q) not found in file_templates parameter", name, filePath)
		}
		values, err := vars.get()
		if err != nil {
			return nil, err
		}
		rendered, err := renderHCLTemplateWithFunctions(content, values, deviceTemplateFunctions)
		if err != nil {
			return nil, fmt.Errorf("rendering file template %q: %w", name, err)
		}
		decoded, err := yamlDecode(rendered)
		if err != nil {
			return nil, fmt.Errorf("decoding file template %q: %w", name, err)
		}
		// Convert nested maps too, like the model, as the final template pass
		// and the removal of null values only walk map[string]any
		if m, ok := orderedMapToPlainMap(decoded).(map[string]any); ok {
			results = append(results, m)
		}
	}
	return results, nil
}

func processModelTemplates(rctx *renderContext, templateNames []string, vars *templateVars) (map[string]any, error) {
	merged := make(map[string]any)
	for _, name := range templateNames {
		tmpl, ok := rctx.templates[name]
		if !ok || getStringVal(tmpl, "type", "") != "model" {
			continue
		}
		config := getMapVal(tmpl, "configuration")
		if len(config) == 0 {
			continue
		}
		rendered, err := renderTemplateValues(config, vars)
		if err != nil {
			return nil, fmt.Errorf("rendering model template %q: %w", name, err)
		}
		if m, ok := rendered.(map[string]any); ok {
			if _, err := MergeMaps(m, merged, true); err != nil {
				return nil, fmt.Errorf("model template %q: %w", name, err)
			}
		}
	}
	return merged, nil
}

// renderTemplateValues walks a native value tree and renders HCL template
// expressions in string values. Non-string values are returned unchanged.
// Variables are only converted if a string contains a template expression.
func renderTemplateValues(v any, vars *templateVars) (any, error) {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			rendered, err := renderTemplateValues(item, vars)
			if err != nil {
				return nil, err
			}
			result[k] = rendered
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			rendered, err := renderTemplateValues(item, vars)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	case string:
		if !strings.Contains(val, "${") {
			return val, nil
		}
		values, err := vars.get()
		if err != nil {
			return nil, err
		}
		return renderHCLTemplateValueWithFunctions(val, values, deviceTemplateFunctions)
	default:
		return v, nil
	}
}

func templatePassOnMap(m map[string]any, vars *templateVars) (map[string]any, error) {
	if len(m) == 0 {
		return m, nil
	}
	rendered, err := renderTemplateValues(m, vars)
	if err != nil {
		return nil, err
	}
	if result, ok := rendered.(map[string]any); ok {
		return result, nil
	}
	return m, nil
}

func resolveInterfaceGroupConfigs(rctx *renderContext, vars *templateVars) (map[string]map[string]any, error) {
	igConfigs := make(map[string]map[string]any)
	for _, igRaw := range rctx.interfaceGroups {
		ig, ok := igRaw.(map[string]any)
		if !ok {
			continue
		}
		name := getStringVal(ig, "name", "")
		if name == "" {
			continue
		}
		config := getMapVal(ig, "configuration")
		if len(config) == 0 {
			igConfigs[name] = map[string]any{}
			continue
		}
		rendered, err := renderTemplateValues(config, vars)
		if err != nil {
			return nil, fmt.Errorf("interface group %q: %w", name, err)
		}
		if m, ok := rendered.(map[string]any); ok {
			igConfigs[name] = m
		}
	}
	return igConfigs, nil
}

func applyInterfaceGroups(config map[string]any, igConfigs map[string]map[string]any) error {
	interfaces := getMapVal(config, "interfaces")
	if len(interfaces) == 0 || len(igConfigs) == 0 {
		return nil
	}
	for typeName, typeVal := range interfaces {
		items, ok := typeVal.([]any)
		if !ok {
			continue
		}
		for i, itemRaw := range items {
			itemMap := toMapStringAny(itemRaw)
			if itemMap == nil {
				continue
			}
			// Recurse into subinterfaces first
			if subsRaw, ok := itemMap["subinterfaces"]; ok {
				if subs, ok := subsRaw.([]any); ok {
					for j, subRaw := range subs {
						subMap := toMapStringAny(subRaw)
						if subMap == nil {
							continue
						}
						merged, err := applyInterfaceGroupToItem(subMap, igConfigs)
						if err != nil {
							return fmt.Errorf("interface %q subinterface: %w", typeName, err)
						}
						subs[j] = merged
					}
					itemMap["subinterfaces"] = subs
				}
			}
			merged, err := applyInterfaceGroupToItem(itemMap, igConfigs)
			if err != nil {
				return fmt.Errorf("interface %q: %w", typeName, err)
			}
			items[i] = merged
		}
		interfaces[typeName] = items
	}
	config["interfaces"] = interfaces
	return nil
}

// toMapStringAny converts *OrderedMap or map[string]any to map[string]any.
func toMapStringAny(v any) map[string]any {
	switch val := v.(type) {
	case map[string]any:
		return val
	case *OrderedMap:
		result := make(map[string]any, val.Len())
		for _, e := range val.Entries() {
			result[e.Key] = e.Value
		}
		return result
	}
	return nil
}

func applyInterfaceGroupToItem(item map[string]any, igConfigs map[string]map[string]any) (map[string]any, error) {
	groups := getStringSlice(item, "interface_groups")
	if len(groups) == 0 {
		return item, nil
	}
	merged := make(map[string]any)
	for _, g := range groups {
		if cfg, ok := igConfigs[g]; ok {
			if _, err := MergeMaps(deepCopy(cfg), merged, true); err != nil {
				return nil, fmt.Errorf("interface group %q: %w", g, err)
			}
		}
	}
	if _, err := MergeMaps(item, merged, true); err != nil {
		return nil, err
	}
	return merged, nil
}

// applyDefaults recursively applies default values as fallbacks into a config map.
// For each key in defaults:
//   - Missing from config → set from defaults (scalars only when inListItem)
//   - Both are maps → recurse
//   - Config has array, defaults has map → apply defaults to each array item
//   - Config has scalar → skip (config value takes precedence)
//
// When applying defaults to list items, new map sub-trees are not added —
// only scalar values and recursion into already-existing maps.
func applyDefaults(config map[string]any, defaults map[string]any) {
	for key, defVal := range defaults {
		configVal, exists := config[key]
		if !exists {
			config[key] = defVal
			continue
		}

		defMap := toMapStringAny(defVal)
		if defMap == nil {
			// Default is a scalar, config already has this key — skip
			continue
		}

		configMap := toMapStringAny(configVal)
		if configMap != nil {
			// Both are maps — recurse
			applyDefaults(configMap, defMap)
			config[key] = configMap
			continue
		}

		if configSlice, ok := configVal.([]any); ok {
			// Config has array, defaults has map — apply to each item
			for i, item := range configSlice {
				itemMap := toMapStringAny(item)
				if itemMap != nil {
					applyDefaults(itemMap, defMap)
					configSlice[i] = itemMap
				}
			}
			continue
		}
	}
}

// collectCliTemplates collects the CLI templates of a device: global, device
// group, template instance, device templates and direct cli_templates entries,
// in increasing order of precedence. The Terraform modules use the names as
// resource keys, so an entry replaces an earlier entry with the same name in
// place (e.g. a template assigned globally and to the device, or a direct entry
// overriding a template).
func collectCliTemplates(rctx *renderContext, device map[string]any, vars *templateVars, groups []matchedGroup, instanceCli []any) ([]any, error) {
	var result []any
	index := make(map[string]int)
	add := func(entry any) {
		name := getStringVal(toMapStringAny(entry), "name", "")
		if i, ok := index[name]; ok && name != "" {
			result[i] = entry
			return
		}
		index[name] = len(result)
		result = append(result, entry)
	}

	render := func(content string, vars *templateVars) (string, error) {
		values, err := vars.get()
		if err != nil {
			return "", err
		}
		return renderHCLTemplateWithFunctions(content, values, deviceTemplateFunctions)
	}

	// Global CLI templates
	for _, name := range getStringSlice(rctx.global, "templates") {
		tmpl, ok := rctx.templates[name]
		if !ok || getStringVal(tmpl, "type", "") != "cli" {
			continue
		}
		content := getStringVal(tmpl, "content", "")
		if content == "" {
			continue
		}
		rendered, err := render(content, vars)
		if err != nil {
			return nil, fmt.Errorf("global cli template %q: %w", name, err)
		}
		add(map[string]any{
			"name":    name,
			"content": rendered,
			"order":   getIntVal(tmpl, "order", rctx.defaultOrder),
			"stage":   getStringVal(tmpl, "stage", rctx.defaultStage),
		})
	}

	// Group CLI templates
	for _, g := range groups {
		dg, dgName := g.group, g.name
		for _, name := range getStringSlice(dg, "templates") {
			tmpl, ok := rctx.templates[name]
			if !ok || getStringVal(tmpl, "type", "") != "cli" {
				continue
			}
			content := getStringVal(tmpl, "content", "")
			if content == "" {
				continue
			}
			rendered, err := render(content, g.vars)
			if err != nil {
				return nil, fmt.Errorf("group %q cli template %q: %w", dgName, name, err)
			}
			add(map[string]any{
				"name":    fmt.Sprintf("%s/%s", name, dgName),
				"content": rendered,
				"order":   getIntVal(tmpl, "order", rctx.defaultOrder),
				"stage":   getStringVal(tmpl, "stage", rctx.defaultStage),
			})
		}
	}

	// Template instance CLI templates (rendered by renderTemplateInstances)
	for _, entry := range instanceCli {
		add(entry)
	}

	// Device CLI templates
	for _, name := range getStringSlice(device, "templates") {
		tmpl, ok := rctx.templates[name]
		if !ok || getStringVal(tmpl, "type", "") != "cli" {
			continue
		}
		content := getStringVal(tmpl, "content", "")
		if content == "" {
			continue
		}
		rendered, err := render(content, vars)
		if err != nil {
			return nil, fmt.Errorf("device cli template %q: %w", name, err)
		}
		add(map[string]any{
			"name":    name,
			"content": rendered,
			"order":   getIntVal(tmpl, "order", rctx.defaultOrder),
			"stage":   getStringVal(tmpl, "stage", rctx.defaultStage),
		})
	}

	// Direct device cli_templates
	if directCli := getSliceVal(device, "cli_templates"); len(directCli) > 0 {
		for _, cli := range directCli {
			if m, ok := cli.(map[string]any); ok {
				if _, hasOrder := m["order"]; !hasOrder {
					m["order"] = rctx.defaultOrder
				}
				if _, hasStage := m["stage"]; !hasStage {
					m["stage"] = rctx.defaultStage
				}
			}
		}
		for _, entry := range directCli {
			add(entry)
		}
	}

	if result == nil {
		result = []any{}
	}
	return result, nil
}

// orderedMapToPlainMap recursively converts *OrderedMap trees to map[string]any.
func orderedMapToPlainMap(v any) any {
	switch val := v.(type) {
	case *OrderedMap:
		result := make(map[string]any, val.Len())
		for _, e := range val.Entries() {
			result[e.Key] = orderedMapToPlainMap(e.Value)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = orderedMapToPlainMap(item)
		}
		return result
	default:
		return v
	}
}

// stripNulls recursively removes keys with nil values from maps.
func stripNulls(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			if v == nil {
				continue
			}
			result[k] = stripNulls(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = stripNulls(item)
		}
		return result
	default:
		return v
	}
}

// buildProviderDevices extracts name/managed and connection fields (url/host/protocol)
// from all devices for provider configuration.
func buildProviderDevices(model map[string]any, arch string, defaultManaged bool) []any {
	archConfig := getMapVal(model, arch)
	devices := getSliceVal(archConfig, "devices")
	result := make([]any, 0, len(devices))
	for _, d := range devices {
		dm := toMapStringAny(d)
		if dm == nil {
			continue
		}
		pd := map[string]any{
			"name":    getStringVal(dm, "name", ""),
			"managed": getBoolVal(dm, "managed", defaultManaged),
		}
		// Copy optional connection fields (omit if not present)
		for _, field := range []string{"url", "host", "protocol"} {
			if v, ok := dm[field]; ok && v != nil {
				pd[field] = v
			}
		}
		result = append(result, pd)
	}
	return result
}

// --- Helper functions ---

func nativeToCtyMap(m map[string]any) (map[string]cty.Value, error) {
	result := make(map[string]cty.Value, len(m))
	for k, v := range m {
		ctyVal, err := nativeToCty(v)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		result[k] = ctyVal
	}
	return result, nil
}

func getMapVal(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	if v, ok := m[key]; ok {
		if vm, ok := v.(map[string]any); ok {
			return vm
		}
		if om, ok := v.(*OrderedMap); ok {
			result := make(map[string]any, om.Len())
			for _, entry := range om.Entries() {
				result[entry.Key] = entry.Value
			}
			return result
		}
	}
	return map[string]any{}
}

func getSliceVal(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		if vs, ok := v.([]any); ok {
			return vs
		}
	}
	return nil
}

func getStringVal(m map[string]any, key string, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func getBoolVal(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func getIntVal(m map[string]any, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return def
}

func getStringSlice(m map[string]any, key string) []string {
	raw := getSliceVal(m, key)
	if raw == nil {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func toStringSet(s []string) map[string]bool {
	if len(s) == 0 {
		return nil
	}
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}

func containsString(s []string, val string) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}

func mergeShallow(base, overlay map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}

func mergeShallowInto(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func normalizeToMapStringAny(val any) (map[string]any, bool) {
	switch v := val.(type) {
	case map[string]any:
		return v, true
	case *OrderedMap:
		result := make(map[string]any, v.Len())
		for _, entry := range v.Entries() {
			result[entry.Key] = entry.Value
		}
		return result, true
	}
	return nil, false
}
