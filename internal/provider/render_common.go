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
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// secretMask replaces resolved !env values in error messages.
const secretMask = "***"

// collectYamlTagValues returns the resolved values of all resolvable YAML tags
// (e.g. "!env VAR") in the value tree. Tags that cannot be resolved are skipped.
func collectYamlTagValues(v any) []string {
	seen := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch val := v.(type) {
		case string:
			if !strings.HasPrefix(val, "!env ") {
				return
			}
			if resolved, err := resolveTagString(val); err == nil {
				if s, ok := resolved.(string); ok && s != "" {
					seen[s] = true
				}
			}
		case *OrderedMap:
			for _, e := range val.Entries() {
				walk(e.Value)
			}
		case map[string]any:
			for _, item := range val {
				walk(item)
			}
		case []any:
			for _, item := range val {
				walk(item)
			}
		}
	}
	walk(v)
	values := make([]string, 0, len(seen))
	for s := range seen {
		values = append(values, s)
	}
	return values
}

// maskSecrets replaces all occurrences of the given secret values in msg, as
// well as their upper and lower case variants, with secretMask. Longer values
// are replaced first, so that a value containing another one is fully masked.
func maskSecrets(msg string, secrets []string) string {
	variants := map[string]bool{}
	for _, s := range secrets {
		if s == "" {
			continue
		}
		variants[s] = true
		variants[strings.ToUpper(s)] = true
		variants[strings.ToLower(s)] = true
	}
	sorted := make([]string, 0, len(variants))
	for s := range variants {
		sorted = append(sorted, s)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if len(sorted[i]) != len(sorted[j]) {
			return len(sorted[i]) > len(sorted[j])
		}
		return sorted[i] < sorted[j]
	})
	for _, s := range sorted {
		msg = strings.ReplaceAll(msg, s, secretMask)
	}
	return msg
}

// resolvedRenderError wraps an error of the render pass with resolved !env
// variables, masking all resolved !env values of the model in its message, so
// that secrets do not end up in Terraform output or logs.
func resolvedRenderError(err error, model map[string]any) error {
	return fmt.Errorf("rendering with resolved !env variables (values of !env tags are masked): %s",
		maskSecrets(err.Error(), collectYamlTagValues(model)))
}

// mergeRenderInputs decodes and merges the YAML strings, merges the HCL model on
// top, and merges the user defaults (the model's "defaults" key) on top of the
// module defaults YAML. It returns the merged model without the "defaults" key
// and the merged defaults (the content of the "defaults" key), both as plain maps.
// `!env` tags are preserved as literal strings in the model and resolved in the
// module defaults.
func mergeRenderInputs(yamlStrings []string, modelDynamic types.Dynamic, defaultsYaml string) (map[string]any, map[string]any, error) {
	// 1. YAML decode + merge (preserving !env tags as literal strings)
	merged := NewOrderedMap(0)
	for _, yamlStr := range yamlStrings {
		decoded, err := yamlDecode(yamlStr)
		if err != nil {
			return nil, nil, fmt.Errorf("decoding YAML string: %w", err)
		}
		if decoded != nil {
			if _, err := MergeMaps(decoded, merged, true); err != nil {
				return nil, nil, fmt.Errorf("merging YAML string: %w", err)
			}
		}
	}

	// 2. Merge HCL model var on top
	modelNative, err := convertDynamicToNative(modelDynamic)
	if err != nil {
		return nil, nil, fmt.Errorf("converting model: %w", err)
	}
	if modelNative != nil {
		if _, err := MergeMaps(modelNative, merged, true); err != nil {
			return nil, nil, fmt.Errorf("merging model: %w", err)
		}
	}

	// 3. Defaults merge: extract user defaults from model, merge with module defaults
	defaults := make(map[string]any)
	if defaultsYaml != "" {
		moduleDefaults, err := yamlDecode(defaultsYaml)
		if err != nil {
			return nil, nil, fmt.Errorf("decoding defaults YAML: %w", err)
		}
		if moduleDefaults != nil {
			// Resolve !env tags in defaults (they may reference env vars)
			moduleDefaults, err = resolveYamlTags(moduleDefaults)
			if err != nil {
				return nil, nil, fmt.Errorf("resolving defaults YAML tags: %w", err)
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
			if _, err := MergeMaps(userDefaultsVal, moduleDefaultsVal, true); err != nil {
				return nil, nil, fmt.Errorf("merging defaults: %w", err)
			}
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
		return nil, nil, fmt.Errorf("model must be a map/object")
	}

	return model, defaults, nil
}

// extractFileTemplates converts the file_templates function argument (a map of
// file path to file content) to a map of strings. Non-string values are ignored.
func extractFileTemplates(fileTemplatesDynamic types.Dynamic) (map[string]string, error) {
	fileTemplatesNative, err := convertDynamicToNative(fileTemplatesDynamic)
	if err != nil {
		return nil, fmt.Errorf("converting file_templates: %w", err)
	}
	fileTemplates := make(map[string]string)
	if ftMap, ok := fileTemplatesNative.(map[string]any); ok {
		for k, v := range ftMap {
			if s, ok := v.(string); ok {
				fileTemplates[k] = s
			}
		}
	}
	return fileTemplates, nil
}
