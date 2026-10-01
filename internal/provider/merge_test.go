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
	"reflect"
	"strings"
	"testing"
)

func TestMergeMaps(t *testing.T) {
	cases := []struct {
		dst    map[string]any
		src    map[string]any
		result map[string]any
	}{
		// merge maps
		{
			dst: map[string]any{
				"e1": "abc",
			},
			src: map[string]any{
				"e2": "def",
			},
			result: map[string]any{
				"e1": "abc",
				"e2": "def",
			},
		},
		// merge empty destination map
		{
			dst: map[string]any{
				"e1": nil,
			},
			src: map[string]any{
				"e1": "abc",
			},
			result: map[string]any{
				"e1": "abc",
			},
		},
		// merge empty destination map nested
		{
			dst: map[string]any{
				"e1": nil,
			},
			src: map[string]any{
				"e1": map[string]any{
					"e2": "abc",
				},
			},
			result: map[string]any{
				"e1": map[string]any{
					"e2": "abc",
				},
			},
		},
		// merge empty source map
		{
			dst: map[string]any{
				"e1": "abc",
			},
			src: map[string]any{
				"e1": nil,
			},
			result: map[string]any{
				"e1": "abc",
			},
		},
		// merge empty source map nested
		{
			dst: map[string]any{
				"e1": map[string]any{
					"e2": "abc",
				},
			},
			src: map[string]any{
				"e1": nil,
			},
			result: map[string]any{
				"e1": map[string]any{
					"e2": "abc",
				},
			},
		},
		// merge nested maps
		{
			dst: map[string]any{
				"root": map[string]any{
					"child1": "abc",
				},
			},
			src: map[string]any{
				"root": map[string]any{
					"child2": "def",
				},
			},
			result: map[string]any{
				"root": map[string]any{
					"child1": "abc",
					"child2": "def",
				},
			},
		},
		// append when merging lists
		{
			dst: map[string]any{
				"list": []any{
					map[string]any{
						"child1": "abc",
					},
				},
			},
			src: map[string]any{
				"list": []any{
					map[string]any{
						"child2": "def",
					},
				},
			},
			result: map[string]any{
				"list": []any{
					map[string]any{
						"child1": "abc",
					},
					map[string]any{
						"child2": "def",
					},
				},
			},
		},
		// merge matching items across lists (no duplicates within each list)
		{
			dst: map[string]any{
				"list": []any{
					map[string]any{
						"child1": "abc",
					},
				},
			},
			src: map[string]any{
				"list": []any{
					map[string]any{
						"child1": "abc",
					},
				},
			},
			result: map[string]any{
				"list": []any{
					map[string]any{
						"child1": "abc",
					},
				},
			},
		},
		// src bool replaces dst primitive value
		{
			dst: map[string]any{
				"attr": false,
			},
			src: map[string]any{
				"attr": true,
			},
			result: map[string]any{
				"attr": true,
			},
		},
		{
			dst: map[string]any{
				"attr": true,
			},
			src: map[string]any{
				"attr": false,
			},
			result: map[string]any{
				"attr": false,
			},
		},
		// empty src string replaces dst string
		{
			dst: map[string]any{
				"attr": "abc",
			},
			src: map[string]any{
				"attr": "",
			},
			result: map[string]any{
				"attr": "",
			},
		},
		// src string replaces dst string
		{
			dst: map[string]any{
				"attr": "abc",
			},
			src: map[string]any{
				"attr": "def",
			},
			result: map[string]any{
				"attr": "def",
			},
		},
		// src number does replace dst number
		{
			dst: map[string]any{
				"attr": 5,
			},
			src: map[string]any{
				"attr": 0,
			},
			result: map[string]any{
				"attr": 0,
			},
		},
		// src number does replace dst string
		{
			dst: map[string]any{
				"attr": "abc",
			},
			src: map[string]any{
				"attr": 0,
			},
			result: map[string]any{
				"attr": 0,
			},
		},
		// src string does replace dst number
		{
			dst: map[string]any{
				"attr": 5,
			},
			src: map[string]any{
				"attr": "abc",
			},
			result: map[string]any{
				"attr": "abc",
			},
		},
		// empty src map does not replace dst map
		{
			dst: map[string]any{
				"attr": "abc",
			},
			src: map[string]any{},
			result: map[string]any{
				"attr": "abc",
			},
		},
		// src map gets merged with dst map
		{
			dst: map[string]any{},
			src: map[string]any{
				"attr": "abc",
			},
			result: map[string]any{
				"attr": "abc",
			},
		},
		// concatenate when source list has duplicates (preserve duplicates)
		{
			dst: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
						"x":    1,
					},
				},
			},
			src: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
					},
					map[string]any{
						"name": "a",
					},
				},
			},
			result: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
						"x":    1,
					},
					map[string]any{
						"name": "a",
					},
					map[string]any{
						"name": "a",
					},
				},
			},
		},
		// concatenate when destination list has duplicates (preserve duplicates)
		{
			dst: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
					},
					map[string]any{
						"name": "a",
					},
				},
			},
			src: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
						"x":    1,
					},
				},
			},
			result: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
					},
					map[string]any{
						"name": "a",
					},
					map[string]any{
						"name": "a",
						"x":    1,
					},
				},
			},
		},
		// merge when no duplicates present
		{
			dst: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
						"x":    1,
					},
				},
			},
			src: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
						"y":    2,
					},
				},
			},
			result: map[string]any{
				"list": []any{
					map[string]any{
						"name": "a",
						"x":    1,
						"y":    2,
					},
				},
			},
		},
	}

	for _, c := range cases {
		if _, err := MergeMaps(c.src, c.dst, true); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(c.dst, c.result) {
			t.Fatalf("Error matching dst and result: %#v vs %#v", c.dst, c.result)
		}
	}
}

func TestMergeMaps_ShapeConflict(t *testing.T) {
	cases := []struct {
		name        string
		dst         map[string]any
		src         map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "map vs list conflict",
			dst: map[string]any{
				"attr": map[string]any{"a": "b"},
			},
			src: map[string]any{
				"attr": []any{"a", "b"},
			},
			wantErr:     true,
			errContains: "attr",
		},
		{
			name: "map vs scalar conflict",
			dst: map[string]any{
				"attr": map[string]any{"a": "b"},
			},
			src: map[string]any{
				"attr": "scalar",
			},
			wantErr:     true,
			errContains: "attr",
		},
		{
			name: "list vs scalar conflict",
			dst: map[string]any{
				"attr": []any{"a"},
			},
			src: map[string]any{
				"attr": 5,
			},
			wantErr:     true,
			errContains: "attr",
		},
		{
			name: "nested map vs list conflict reports full path",
			dst: map[string]any{
				"root": map[string]any{
					"attr": map[string]any{"a": "b"},
				},
			},
			src: map[string]any{
				"root": map[string]any{
					"attr": []any{"a"},
				},
			},
			wantErr:     true,
			errContains: "root.attr",
		},
		{
			name: "conflict inside matched list item",
			dst: map[string]any{
				"list": []any{
					map[string]any{
						"name":  "a",
						"attrs": map[string]any{"x": "y"},
					},
				},
			},
			src: map[string]any{
				"list": []any{
					map[string]any{
						"name":  "a",
						"attrs": []any{"x"},
					},
				},
			},
			wantErr:     true,
			errContains: "attrs",
		},
		{
			name: "null src never conflicts",
			dst: map[string]any{
				"attr": map[string]any{"a": "b"},
			},
			src: map[string]any{
				"attr": nil,
			},
			wantErr: false,
		},
		{
			name: "null dst never conflicts",
			dst: map[string]any{
				"attr": nil,
			},
			src: map[string]any{
				"attr": []any{"a"},
			},
			wantErr: false,
		},
		{
			name: "scalar value and type differences are still unrestricted",
			dst: map[string]any{
				"attr": 5,
			},
			src: map[string]any{
				"attr": "abc",
			},
			wantErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := MergeMaps(c.src, c.dst, true)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if c.errContains != "" && !strings.Contains(err.Error(), c.errContains) {
					t.Fatalf("expected error to contain %q, got: %s", c.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestMergeListItem(t *testing.T) {
	cases := []struct {
		dst    []any
		src    any
		result []any
	}{
		// merge primitive list items
		{
			dst: []any{
				"abc",
				"def",
			},
			src: "ghi",
			result: []any{
				"abc",
				"def",
				"ghi",
			},
		},
		// do not merge matching primitive list items
		{
			dst: []any{
				"abc",
				"def",
			},
			src: "abc",
			result: []any{
				"abc",
				"def",
				"abc",
			},
		},
		// merge matching map list items
		{
			dst: []any{
				map[string]any{
					"name": "abc",
					"map": map[string]any{
						"elem1": "value1",
						"elem2": "value2",
					},
				},
			},
			src: map[string]any{
				"name": "abc",
				"map": map[string]any{
					"elem3": "value3",
				},
			},
			result: []any{
				map[string]any{
					"name": "abc",
					"map": map[string]any{
						"elem1": "value1",
						"elem2": "value2",
						"elem3": "value3",
					},
				},
			},
		},
		// merge matching map list items with extra src primitive attribute
		{
			dst: []any{
				map[string]any{
					"name": "abc",
					"map": map[string]any{
						"elem1": "value1",
						"elem2": "value2",
					},
				},
			},
			src: map[string]any{
				"name":  "abc",
				"name2": "def",
				"map": map[string]any{
					"elem3": "value3",
				},
			},
			result: []any{
				map[string]any{
					"name":  "abc",
					"name2": "def",
					"map": map[string]any{
						"elem1": "value1",
						"elem2": "value2",
						"elem3": "value3",
					},
				},
			},
		},
		// merge matching map list items with extra dst primitive attribute
		{
			dst: []any{
				map[string]any{
					"name":  "abc",
					"name2": "def",
					"map": map[string]any{
						"elem1": "value1",
						"elem2": "value2",
					},
				},
			},
			src: map[string]any{
				"name": "abc",
				"map": map[string]any{
					"elem3": "value3",
				},
			},
			result: []any{
				map[string]any{
					"name":  "abc",
					"name2": "def",
					"map": map[string]any{
						"elem1": "value1",
						"elem2": "value2",
						"elem3": "value3",
					},
				},
			},
		},
		// merge matching dict list items with extra dst and src primitive attribute
		{
			dst: []any{
				map[string]any{
					"name":  "abc",
					"name2": "def",
				},
			},
			src: map[string]any{
				"name":  "abc",
				"name3": "ghi",
			},
			result: []any{
				map[string]any{
					"name":  "abc",
					"name2": "def",
					"name3": "ghi",
				},
			},
		},
		// append map to list containing primitives (mixed types)
		{
			dst: []any{
				"abc",
				"def",
			},
			src: map[string]any{
				"name": "ghi",
			},
			result: []any{
				"abc",
				"def",
				map[string]any{
					"name": "ghi",
				},
			},
		},
	}

	for _, c := range cases {
		if err := MergeListItem(c.src, &c.dst, true); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(c.dst, c.result) {
			t.Fatalf("Error matching dst and result: %#v vs %#v", c.dst, c.result)
		}
	}
}
