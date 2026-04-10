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
	"sort"

	goyaml "github.com/goccy/go-yaml"
)

// Entry is a key-value pair in an OrderedMap.
type Entry struct {
	Key   string
	Value any
}

// OrderedMap preserves insertion order of keys while providing O(1) lookups.
// Used as the internal representation for YAML mappings to maintain source key order
// through decode → merge → encode roundtrips.
type OrderedMap struct {
	entries []Entry
	index   map[string]int // key → position in entries
}

// NewOrderedMap creates a new OrderedMap with the given initial capacity.
func NewOrderedMap(cap int) *OrderedMap {
	return &OrderedMap{
		entries: make([]Entry, 0, cap),
		index:   make(map[string]int, cap),
	}
}

// Get returns the value for a key and whether it exists. O(1).
func (m *OrderedMap) Get(key string) (any, bool) {
	idx, ok := m.index[key]
	if !ok {
		return nil, false
	}
	return m.entries[idx].Value, true
}

// Set inserts or updates a key-value pair.
// If the key already exists, the value is updated in-place (preserving position).
// If the key is new, it is appended at the end.
func (m *OrderedMap) Set(key string, value any) {
	if idx, ok := m.index[key]; ok {
		m.entries[idx].Value = value
		return
	}
	m.index[key] = len(m.entries)
	m.entries = append(m.entries, Entry{Key: key, Value: value})
}

// Delete removes a key from the map, preserving the order of remaining entries.
func (m *OrderedMap) Delete(key string) {
	idx, ok := m.index[key]
	if !ok {
		return
	}
	// Shift entries after the deleted one
	copy(m.entries[idx:], m.entries[idx+1:])
	m.entries = m.entries[:len(m.entries)-1]
	delete(m.index, key)
	// Rebuild indices for shifted entries
	for i := idx; i < len(m.entries); i++ {
		m.index[m.entries[i].Key] = i
	}
}

// Len returns the number of entries.
func (m *OrderedMap) Len() int {
	return len(m.entries)
}

// Entries returns the ordered slice of entries.
// The caller must not modify the returned slice.
func (m *OrderedMap) Entries() []Entry {
	return m.entries
}

// Has returns whether the key exists. O(1).
func (m *OrderedMap) Has(key string) bool {
	_, ok := m.index[key]
	return ok
}

// ToMap converts to an unordered map[string]any.
// Used at type boundaries where ordering is not needed (e.g., Terraform conversion).
func (m *OrderedMap) ToMap() map[string]any {
	result := make(map[string]any, len(m.entries))
	for _, e := range m.entries {
		result[e.Key] = e.Value
	}
	return result
}

// toMapSlice recursively converts a value to goccy/go-yaml MapSlice representation.
// *OrderedMap → MapSlice, map[string]any → MapSlice, []any elements are recursively converted, other types pass through.
func toMapSlice(v any) any {
	switch val := v.(type) {
	case *OrderedMap:
		ms := make(goyaml.MapSlice, 0, val.Len())
		for _, e := range val.entries {
			ms = append(ms, goyaml.MapItem{
				Key:   e.Key,
				Value: toMapSlice(e.Value),
			})
		}
		return ms
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ms := make(goyaml.MapSlice, 0, len(val))
		for _, k := range keys {
			ms = append(ms, goyaml.MapItem{
				Key:   k,
				Value: toMapSlice(val[k]),
			})
		}
		return ms
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = toMapSlice(item)
		}
		return result
	default:
		return v
	}
}
