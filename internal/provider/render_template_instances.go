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
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// templateInstance is a template applied to a set of devices with its own
// variables (<arch>.template_instances), rendered once per item.
type templateInstance struct {
	name      string
	template  string
	tmpl      map[string]any
	targets   map[string]bool // names of the devices the instance applies to
	variables map[string]any
	items     []templateInstanceItem
}

// templateInstanceItem is one rendering of a template instance. An instance
// without `items` has a single implicit item applying to all its targets.
type templateInstanceItem struct {
	label     string          // e.g. "items[2]" or "items[name=esx01]", empty for the implicit item
	cliName   string          // suffix of CLI template names, e.g. "items[2]" or "esx01", empty for the implicit item
	targets   map[string]bool // nil: all targets of the instance
	variables map[string]any
}

var (
	templateInstanceKeys     = map[string]bool{"name": true, "template": true, "devices": true, "device_groups": true, "variables": true, "items": true}
	templateInstanceItemKeys = map[string]bool{"name": true, "devices": true, "device_groups": true, "variables": true}
)

// instanceValidator collects the problems of template instances.
type instanceValidator struct {
	errs []error
}

func (v *instanceValidator) errorf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

// unknownKeys reports the attributes of m that are not in allowed.
func (v *instanceValidator) unknownKeys(m map[string]any, allowed map[string]bool, path string) {
	var unknown []string
	for k := range m {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		v.errorf("%s: unknown attribute %q", path, k)
	}
}

// stringList returns the list of strings m[key] (nil if absent or null).
func (v *instanceValidator) stringList(m map[string]any, key, path string) []string {
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		v.errorf("%s: %s must be a list of strings", path, key)
		return nil
	}
	result := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			v.errorf("%s: %s must be a list of strings", path, key)
			return nil
		}
		result = append(result, s)
	}
	return result
}

// variables returns the map m["variables"] (nil if absent or null).
func (v *instanceValidator) variables(m map[string]any, path string) map[string]any {
	raw, ok := m["variables"]
	if !ok || raw == nil {
		return nil
	}
	vars, ok := normalizeToMapStringAny(raw)
	if !ok {
		v.errorf("%s: variables must be a map", path)
	}
	return vars
}

// templateInstanceName returns m["name"] as a string (numbers are accepted), or "" if absent
// or not a scalar.
func templateInstanceName(m map[string]any) string {
	switch n := m["name"].(type) {
	case string:
		return n
	case int, int64, uint64, float64:
		return fmt.Sprint(n)
	}
	return ""
}

// deviceGroupMembers returns the device names per device group name, for the
// device groups defined in <arch>.device_groups and those only referenced by
// devices, with membership in either direction (as deviceMatchesGroup).
func deviceGroupMembers(devices, deviceGroups []any) map[string]map[string]bool {
	members := map[string]map[string]bool{}
	add := func(group, device string) {
		if members[group] == nil {
			members[group] = map[string]bool{}
		}
		if device != "" {
			members[group][device] = true
		}
	}
	for _, dgRaw := range deviceGroups {
		dg := toMapStringAny(dgRaw)
		name := getStringVal(dg, "name", "")
		if name == "" {
			continue
		}
		add(name, "")
		for _, d := range getStringSlice(dg, "devices") {
			add(name, d)
		}
	}
	for _, dRaw := range devices {
		d := toMapStringAny(dRaw)
		for _, g := range getStringSlice(d, "device_groups") {
			add(g, getStringVal(d, "name", ""))
		}
	}
	return members
}

// parseTemplateInstances parses and validates <arch>.template_instances. All
// problems are returned at once, each with the path of the instance or item.
func parseTemplateInstances(archConfig map[string]any, templates map[string]map[string]any, devices, deviceGroups []any) ([]templateInstance, error) {
	rawList, ok := archConfig["template_instances"]
	if !ok || rawList == nil {
		return nil, nil
	}
	v := &instanceValidator{}
	raw, ok := rawList.([]any)
	if !ok {
		v.errorf("template_instances: must be a list")
	}
	if len(raw) == 0 && len(v.errs) == 0 {
		return nil, nil
	}

	deviceNames := map[string]bool{}
	for _, dRaw := range devices {
		if name := getStringVal(toMapStringAny(dRaw), "name", ""); name != "" {
			deviceNames[name] = true
		}
	}
	groupMembers := deviceGroupMembers(devices, deviceGroups)

	// targetsOf returns the devices selected by the devices and device groups
	// of an instance or item, reporting unknown names.
	targetsOf := func(devs, groups []string, path string) map[string]bool {
		targets := map[string]bool{}
		for _, d := range devs {
			if !deviceNames[d] {
				v.errorf("%s: unknown device %q", path, d)
				continue
			}
			targets[d] = true
		}
		for _, g := range groups {
			members, ok := groupMembers[g]
			if !ok {
				v.errorf("%s: unknown device group %q", path, g)
				continue
			}
			for d := range members {
				targets[d] = true
			}
		}
		return targets
	}

	var instances []templateInstance
	seen := map[string]bool{}
	for i, instRaw := range raw {
		path := fmt.Sprintf("template_instances[%d]", i)
		m, ok := normalizeToMapStringAny(instRaw)
		if !ok {
			v.errorf("%s: must be a map", path)
			continue
		}
		name := templateInstanceName(m)
		if name == "" {
			v.errorf("%s: name is required", path)
			continue
		}
		path = fmt.Sprintf("template_instances[%s]", name)
		if seen[name] {
			v.errorf("%s: duplicate name", path)
			continue
		}
		seen[name] = true
		if strings.Contains(name, "/") {
			v.errorf("%s: name must not contain \"/\"", path)
		}
		v.unknownKeys(m, templateInstanceKeys, path)

		inst := templateInstance{name: name, variables: v.variables(m, path)}
		inst.template = getStringVal(m, "template", "")
		if inst.template == "" {
			v.errorf("%s: template is required", path)
		} else if tmpl, ok := templates[inst.template]; !ok {
			v.errorf("%s: unknown template %q", path, inst.template)
		} else {
			switch t := getStringVal(tmpl, "type", ""); t {
			case "file", "model", "cli":
				inst.tmpl = tmpl
			default:
				v.errorf("%s: template %q has unsupported type %q", path, inst.template, t)
			}
		}

		devs := v.stringList(m, "devices", path)
		groups := v.stringList(m, "device_groups", path)
		if len(devs) == 0 && len(groups) == 0 {
			v.errorf("%s: devices or device_groups is required", path)
		}
		inst.targets = targetsOf(devs, groups, path)

		// No items (or null): a single implicit item for all targets
		itemsRaw, hasItems := m["items"]
		if !hasItems || itemsRaw == nil {
			inst.items = []templateInstanceItem{{}}
			instances = append(instances, inst)
			continue
		}
		items, ok := itemsRaw.([]any)
		if !ok {
			v.errorf("%s: items must be a list", path)
			continue
		}
		cliNames := map[string]bool{}
		for j, itemRaw := range items {
			itemPath := fmt.Sprintf("%s.items[%d]", path, j)
			im, ok := normalizeToMapStringAny(itemRaw)
			if !ok {
				v.errorf("%s: must be a map", itemPath)
				continue
			}
			item := templateInstanceItem{
				label:   fmt.Sprintf("items[%d]", j),
				cliName: fmt.Sprintf("items[%d]", j),
			}
			if itemName := templateInstanceName(im); itemName != "" {
				item.label = fmt.Sprintf("items[name=%s]", itemName)
				item.cliName = itemName
				itemPath = path + "." + item.label
				if strings.Contains(itemName, "/") {
					v.errorf("%s: name must not contain \"/\"", itemPath)
				}
			}
			if cliNames[item.cliName] {
				v.errorf("%s: duplicate name", itemPath)
			}
			cliNames[item.cliName] = true
			v.unknownKeys(im, templateInstanceItemKeys, itemPath)
			item.variables = v.variables(im, itemPath)

			itemDevices := v.stringList(im, "devices", itemPath)
			itemGroups := v.stringList(im, "device_groups", itemPath)
			if len(itemDevices) > 0 || len(itemGroups) > 0 {
				// Devices named explicitly must be targets of the instance,
				// device groups select the instance targets that are members
				item.targets = map[string]bool{}
				for _, d := range itemDevices {
					switch {
					case !deviceNames[d]:
						v.errorf("%s: unknown device %q", itemPath, d)
					case !inst.targets[d]:
						v.errorf("%s: device %q is not a target of the template instance", itemPath, d)
					default:
						item.targets[d] = true
					}
				}
				for d := range targetsOf(nil, itemGroups, itemPath) {
					if inst.targets[d] {
						item.targets[d] = true
					}
				}
			}
			inst.items = append(inst.items, item)
		}
		instances = append(instances, inst)
	}
	if len(v.errs) > 0 {
		return nil, fmt.Errorf("invalid template instances:\n%w", errors.Join(v.errs...))
	}
	return instances, nil
}

// resolveInstanceExpressions renders the template expressions left in the
// rendered configuration of an instance item, e.g. from a variable referring to
// another variable (`desc: "VLAN_${vlan_id}"`), with the variables of the item.
// The final template pass of the device would otherwise render them with the
// device variables only, which lack the instance and item variables or have a
// different value of the same name. It renders the same values as the final
// template pass.
func resolveInstanceExpressions(cfg map[string]any, vars *templateVars) (map[string]any, error) {
	return templatePassOnMap(cfg, vars)
}

// instanceConfig is the rendered configuration of a template instance item.
type instanceConfig struct {
	label  string // e.g. template instance "EP" items[name=esx01]
	config map[string]any
}

// checkInstanceConflicts reports values that template instances (or items of
// one instance) set to different values on a device, as the later one would
// otherwise replace the earlier one without notice. Only values in maps are
// compared: list items are merged by their values, so conflicting list items
// are kept as separate items instead of replacing each other. Equal values
// (e.g. a feature enabled by several services) are allowed. All conflicts are
// returned at once.
func checkInstanceConflicts(configs []instanceConfig) error {
	if len(configs) < 2 {
		return nil
	}
	type setter struct {
		value any
		label string
	}
	seen := map[string]setter{}
	var errs []error
	var walk func(m map[string]any, path, label string)
	walk = func(m map[string]any, path, label string) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			p := conflictPath(path, k)
			if v == nil {
				continue
			}
			if child := toMapStringAny(v); child != nil {
				walk(child, p, label)
				continue
			}
			if !isPrimitive(v) {
				continue
			}
			prev, ok := seen[p]
			if !ok {
				seen[p] = setter{value: v, label: label}
				continue
			}
			if !scalarsEqual(prev.value, v) {
				errs = append(errs, fmt.Errorf("%s: %s sets %s, %s sets %s", p, prev.label, formatScalar(prev.value), label, formatScalar(v)))
			}
		}
	}
	for _, c := range configs {
		walk(c.config, "", c.label)
	}
	if len(errs) > 0 {
		return fmt.Errorf("conflicting template instances:\n%w", errors.Join(errs...))
	}
	return nil
}

// scalarsEqual compares two scalar values by their text, so that values the
// Terraform modules treat the same are equal independent of their Go type,
// e.g. 10 from a model template and "10" from a quoted file template value, or
// true and "true". Integers are compared exactly.
func scalarsEqual(a, b any) bool {
	return scalarText(a) == scalarText(b)
}

func scalarText(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case bool:
		return strconv.FormatBool(n)
	case int:
		return strconv.Itoa(n)
	case int32:
		return strconv.FormatInt(int64(n), 10)
	case int64:
		return strconv.FormatInt(n, 10)
	case uint64:
		return strconv.FormatUint(n, 10)
	case float32:
		return floatText(float64(n))
	case float64:
		return floatText(n)
	}
	return fmt.Sprint(v)
}

// floatText formats whole numbers like integers (1.0 → "1"), others in their
// shortest exact form.
func floatText(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1<<53 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func formatScalar(v any) string {
	if s, ok := v.(string); ok {
		return strconv.Quote(s)
	}
	return fmt.Sprint(v)
}

// renderTemplateInstances renders the template instances that apply to a
// device, in instance and item order. Each item sees the device variables
// (deviceVars, including GLOBAL, DEVICES and DEVICE), the instance variables
// and the item variables, in increasing order of precedence; only the instance
// and item variables are converted per item. It returns the configurations of
// file and model templates (with the label of their instance item) and the CLI
// templates.
func renderTemplateInstances(rctx *renderContext, deviceName string, deviceVars *templateVars) ([]instanceConfig, []any, error) {
	var configs []instanceConfig
	var cliTemplates []any
	for _, inst := range rctx.instances {
		if !inst.targets[deviceName] {
			continue
		}
		for _, item := range inst.items {
			if item.targets != nil && !item.targets[deviceName] {
				continue
			}
			label := fmt.Sprintf("template instance %q", inst.name)
			if item.label != "" {
				label += " " + item.label
			}
			vars := &templateVars{
				rctx:   rctx,
				label:  label + " vars",
				native: mergeShallow(inst.variables, item.variables),
				base:   deviceVars,
			}

			switch getStringVal(inst.tmpl, "type", "") {
			case "file":
				ft, err := processFileTemplates(rctx, []string{inst.template}, vars)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", label, err)
				}
				for _, cfg := range ft {
					cfg, err = resolveInstanceExpressions(cfg, vars)
					if err != nil {
						return nil, nil, fmt.Errorf("%s: %w", label, err)
					}
					configs = append(configs, instanceConfig{label: label, config: cfg})
				}
			case "model":
				mt, err := processModelTemplates(rctx, []string{inst.template}, vars)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", label, err)
				}
				if len(mt) > 0 {
					mt, err = resolveInstanceExpressions(mt, vars)
					if err != nil {
						return nil, nil, fmt.Errorf("%s: %w", label, err)
					}
					configs = append(configs, instanceConfig{label: label, config: mt})
				}
			case "cli":
				content := getStringVal(inst.tmpl, "content", "")
				if content == "" {
					continue
				}
				values, err := vars.get()
				if err != nil {
					return nil, nil, err
				}
				rendered, err := renderHCLTemplateWithFunctions(content, values, deviceTemplateFunctions)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: cli template %q: %w", label, inst.template, err)
				}
				// The instance segment keeps the name apart from the CLI
				// templates of device groups (<template>/<group>)
				name := fmt.Sprintf("%s/instance/%s", inst.template, inst.name)
				if item.cliName != "" {
					name += "/" + item.cliName
				}
				cliTemplates = append(cliTemplates, map[string]any{
					"name":    name,
					"content": rendered,
					"order":   getIntVal(inst.tmpl, "order", rctx.defaultOrder),
					"stage":   getStringVal(inst.tmpl, "stage", rctx.defaultStage),
				})
			}
		}
	}
	return configs, cliTemplates, nil
}
