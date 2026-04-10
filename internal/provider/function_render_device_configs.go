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
			"Handles template evaluation, deep merging with precedence cascade (global → group → device), " +
			"interface group merging, and CLI template collection. Supports nxos, iosxe, and iosxr architectures.\n\n" +
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

	// 1. YAML decode + merge (preserving !env tags as literal strings)
	merged := NewOrderedMap(0)
	for _, yamlStr := range yamlStrings {
		decoded, err := yamlDecode(yamlStr)
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error decoding YAML string: "+err.Error()))
			return
		}
		if decoded != nil {
			MergeMaps(decoded, merged, true)
		}
	}

	// 2. Merge HCL model var on top
	modelNative, err := convertDynamicToNative(modelDynamic)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting model: "+err.Error()))
		return
	}
	if modelNative != nil {
		MergeMaps(modelNative, merged, true)
	}

	// 3. Defaults merge: extract user defaults from model, merge with module defaults
	defaults := make(map[string]any)
	if defaultsYaml != "" {
		moduleDefaults, err := yamlDecode(defaultsYaml)
		if err != nil {
			resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error decoding defaults YAML: "+err.Error()))
			return
		}
		if moduleDefaults != nil {
			// Resolve !env tags in defaults (they may reference env vars)
			moduleDefaults, err = resolveYamlTags(moduleDefaults)
			if err != nil {
				resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error resolving defaults YAML tags: "+err.Error()))
				return
			}
		}
		// Extract "defaults" key from module defaults YAML
		var moduleDefaultsVal any = NewOrderedMap(0)
		if md, ok := moduleDefaults.(*OrderedMap); ok {
			if v, ok := md.Get("defaults"); ok {
				moduleDefaultsVal = v
			}
		}
		// Extract user defaults from merged model (may be *OrderedMap or map[string]any)
		var userDefaultsVal any
		if v, ok := merged.Get("defaults"); ok {
			userDefaultsVal = v
		}
		// Merge user defaults on top of module defaults (user wins)
		if userDefaultsVal != nil {
			MergeMaps(userDefaultsVal, moduleDefaultsVal, true)
		}
		// Convert to map[string]any
		if d, ok := orderedMapToPlainMap(moduleDefaultsVal).(map[string]any); ok {
			defaults = d
		}
	}
	// Remove "defaults" key from model
	merged.Delete("defaults")

	// 4. Convert merged model to map[string]any
	model, ok := orderedMapToPlainMap(merged).(map[string]any)
	if !ok {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Model must be a map/object"))
		return
	}

	// 5. Extract file_templates from Dynamic
	fileTemplatesNative, err := convertDynamicToNative(fileTemplatesDynamic)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error converting file_templates: "+err.Error()))
		return
	}
	fileTemplates := make(map[string]string)
	if ftMap, ok := fileTemplatesNative.(map[string]any); ok {
		for k, v := range ftMap {
			if s, ok := v.(string); ok {
				fileTemplates[k] = s
			}
		}
	}

	// 6. Discover architecture and build provider_devices
	arch, err := discoverArchitecture(model)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError("Error discovering architecture: "+err.Error()))
		return
	}
	defaultManaged := getBoolVal(getMapVal(getMapVal(defaults, arch), "devices"), "managed", true)
	providerDevices := buildProviderDevices(model, arch, defaultManaged)

	// 7. Run the render pipeline
	result, err := renderDeviceConfigs(model, fileTemplates, managedDevicesTF, managedGroupsTF, defaults)
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

	// 9. Produce resolved output (resolve !env tags, strip nulls)
	resolvedNative, err := resolveYamlTags(result)
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
	fileTemplates   map[string]string
	defaultOrder    int
	defaultManaged  bool
	defaultConfig   map[string]any // defaults[arch].devices.configuration
}

// renderDeviceConfigs is the core pipeline.
func renderDeviceConfigs(model map[string]any, fileTemplates map[string]string, managedDevices, managedGroups []string, defaults map[string]any) (map[string]any, error) {
	// 1. Discover architecture
	arch, err := discoverArchitecture(model)
	if err != nil {
		return nil, err
	}

	// 2. Extract context
	rctx, err := extractRenderContext(model, arch, fileTemplates, defaults)
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

func extractRenderContext(model map[string]any, arch string, fileTemplates map[string]string, defaults map[string]any) (*renderContext, error) {
	archConfig := getMapVal(model, arch)
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

	return &renderContext{
		arch:            arch,
		archConfig:      archConfig,
		global:          getMapVal(archConfig, "global"),
		devices:         getSliceVal(archConfig, "devices"),
		deviceGroups:    getSliceVal(archConfig, "device_groups"),
		interfaceGroups: getSliceVal(archConfig, "interface_groups"),
		templates:       templates,
		fileTemplates:   fileTemplates,
		defaultOrder:    getIntVal(getMapVal(defaultsArch, "templates"), "order", 0),
		defaultManaged:  getBoolVal(getMapVal(defaultsArch, "devices"), "managed", true),
		defaultConfig:   getMapVal(getMapVal(defaultsArch, "devices"), "configuration"),
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

	// 4a. Build device variables (shallow merge)
	deviceVars := buildDeviceVariables(rctx, device)
	ctyVars, err := nativeToCtyMap(deviceVars)
	if err != nil {
		return nil, fmt.Errorf("converting variables: %w", err)
	}

	// 4b. Process file templates
	globalFileTmpls, err := processFileTemplates(rctx, getStringSlice(rctx.global, "templates"), ctyVars)
	if err != nil {
		return nil, fmt.Errorf("global file templates: %w", err)
	}

	var groupFileTmpls []map[string]any
	var groupModelTmpls []map[string]any
	var groupConfigs []map[string]any
	for _, dgRaw := range rctx.deviceGroups {
		dg, ok := dgRaw.(map[string]any)
		if !ok || !deviceMatchesGroup(device, dg) {
			continue
		}
		// Group file templates with extra group variables
		groupVars := mergeShallow(deviceVars, getMapVal(dg, "variables"))
		groupCtyVars, err := nativeToCtyMap(groupVars)
		if err != nil {
			return nil, fmt.Errorf("group %q vars: %w", getStringVal(dg, "name", ""), err)
		}

		ft, err := processFileTemplates(rctx, getStringSlice(dg, "templates"), groupCtyVars)
		if err != nil {
			return nil, fmt.Errorf("group %q file templates: %w", getStringVal(dg, "name", ""), err)
		}
		groupFileTmpls = append(groupFileTmpls, ft...)

		// Group model templates
		mt, err := processModelTemplates(rctx, getStringSlice(dg, "templates"), groupCtyVars)
		if err != nil {
			return nil, fmt.Errorf("group %q model templates: %w", getStringVal(dg, "name", ""), err)
		}
		groupModelTmpls = append(groupModelTmpls, mt)

		// Group configuration
		if cfg := getMapVal(dg, "configuration"); len(cfg) > 0 {
			groupConfigs = append(groupConfigs, cfg)
		}
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

	// 4d. 9-level precedence cascade
	merged := make(map[string]any)
	// 1. Global file templates
	for _, ft := range globalFileTmpls {
		MergeMaps(ft, merged, true)
	}
	// 2. Global model templates
	MergeMaps(globalModelTmpl, merged, true)
	// 3. Global configuration
	MergeMaps(getMapVal(rctx.global, "configuration"), merged, true)
	// 4. Group file templates
	for _, ft := range groupFileTmpls {
		MergeMaps(ft, merged, true)
	}
	// 5. Group model templates
	for _, mt := range groupModelTmpls {
		MergeMaps(mt, merged, true)
	}
	// 6. Group configurations
	for _, gc := range groupConfigs {
		MergeMaps(gc, merged, true)
	}
	// 7. Device file templates
	for _, ft := range deviceFileTmpls {
		MergeMaps(ft, merged, true)
	}
	// 8. Device model templates
	MergeMaps(deviceModelTmpl, merged, true)
	// 9. Device configuration
	MergeMaps(getMapVal(device, "configuration"), merged, true)

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
	applyInterfaceGroups(merged, igConfigs)

	// 4g. CLI templates
	cliTemplates, err := collectCliTemplates(rctx, device, deviceVars, ctyVars)
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

func buildDeviceVariables(rctx *renderContext, device map[string]any) map[string]any {
	result := map[string]any{
		"GLOBAL": rctx.archConfig,
	}
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

func processFileTemplates(rctx *renderContext, templateNames []string, vars map[string]cty.Value) ([]map[string]any, error) {
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
		rendered, err := renderHCLTemplate(content, vars)
		if err != nil {
			return nil, fmt.Errorf("rendering file template %q: %w", name, err)
		}
		decoded, err := yamlDecode(rendered)
		if err != nil {
			return nil, fmt.Errorf("decoding file template %q: %w", name, err)
		}
		if m, ok := normalizeToMapStringAny(decoded); ok {
			results = append(results, m)
		}
	}
	return results, nil
}

func processModelTemplates(rctx *renderContext, templateNames []string, vars map[string]cty.Value) (map[string]any, error) {
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
			MergeMaps(m, merged, true)
		}
	}
	return merged, nil
}

// renderTemplateValues walks a native value tree and renders HCL template
// expressions in string values. Non-string values are returned unchanged.
func renderTemplateValues(v any, vars map[string]cty.Value) (any, error) {
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
		return renderHCLTemplateValue(val, vars)
	default:
		return v, nil
	}
}

func templatePassOnMap(m map[string]any, vars map[string]cty.Value) (map[string]any, error) {
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

func resolveInterfaceGroupConfigs(rctx *renderContext, vars map[string]cty.Value) (map[string]map[string]any, error) {
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

func applyInterfaceGroups(config map[string]any, igConfigs map[string]map[string]any) {
	interfaces := getMapVal(config, "interfaces")
	if len(interfaces) == 0 || len(igConfigs) == 0 {
		return
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
						subs[j] = applyInterfaceGroupToItem(subMap, igConfigs)
					}
					itemMap["subinterfaces"] = subs
				}
			}
			items[i] = applyInterfaceGroupToItem(itemMap, igConfigs)
		}
		interfaces[typeName] = items
	}
	config["interfaces"] = interfaces
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

func applyInterfaceGroupToItem(item map[string]any, igConfigs map[string]map[string]any) map[string]any {
	groups := getStringSlice(item, "interface_groups")
	if len(groups) == 0 {
		return item
	}
	merged := make(map[string]any)
	for _, g := range groups {
		if cfg, ok := igConfigs[g]; ok {
			MergeMaps(cfg, merged, true)
		}
	}
	MergeMaps(item, merged, true)
	return merged
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

func collectCliTemplates(rctx *renderContext, device map[string]any, deviceVars map[string]any, ctyVars map[string]cty.Value) ([]any, error) {
	var result []any

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
		rendered, err := renderHCLTemplate(content, ctyVars)
		if err != nil {
			return nil, fmt.Errorf("global cli template %q: %w", name, err)
		}
		result = append(result, map[string]any{
			"name":    name,
			"content": rendered,
			"order":   getIntVal(tmpl, "order", rctx.defaultOrder),
		})
	}

	// Group CLI templates
	for _, dgRaw := range rctx.deviceGroups {
		dg, ok := dgRaw.(map[string]any)
		if !ok || !deviceMatchesGroup(device, dg) {
			continue
		}
		dgName := getStringVal(dg, "name", "")
		groupVars := mergeShallow(deviceVars, getMapVal(dg, "variables"))
		groupCtyVars, err := nativeToCtyMap(groupVars)
		if err != nil {
			return nil, fmt.Errorf("group %q cli vars: %w", dgName, err)
		}
		for _, name := range getStringSlice(dg, "templates") {
			tmpl, ok := rctx.templates[name]
			if !ok || getStringVal(tmpl, "type", "") != "cli" {
				continue
			}
			content := getStringVal(tmpl, "content", "")
			if content == "" {
				continue
			}
			rendered, err := renderHCLTemplate(content, groupCtyVars)
			if err != nil {
				return nil, fmt.Errorf("group %q cli template %q: %w", dgName, name, err)
			}
			result = append(result, map[string]any{
				"name":    fmt.Sprintf("%s/%s", name, dgName),
				"content": rendered,
				"order":   getIntVal(tmpl, "order", rctx.defaultOrder),
			})
		}
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
		rendered, err := renderHCLTemplate(content, ctyVars)
		if err != nil {
			return nil, fmt.Errorf("device cli template %q: %w", name, err)
		}
		result = append(result, map[string]any{
			"name":    name,
			"content": rendered,
			"order":   getIntVal(tmpl, "order", rctx.defaultOrder),
		})
	}

	// Direct device cli_templates
	if directCli := getSliceVal(device, "cli_templates"); len(directCli) > 0 {
		for _, cli := range directCli {
			if m, ok := cli.(map[string]any); ok {
				if _, hasOrder := m["order"]; !hasOrder {
					m["order"] = rctx.defaultOrder
				}
			}
		}
		result = append(result, directCli...)
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

// buildProviderDevices extracts name/url/managed from all devices for provider configuration.
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
		if url, ok := dm["url"]; ok && url != nil {
			pd["url"] = url
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
