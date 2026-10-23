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
	"math/rand"
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

// mergeYAMLDocs merges YAML documents in order, either with a listMerger (cached)
// or with a separate MergeMaps call per document, and returns the encoded result.
func mergeYAMLDocs(t *testing.T, docs []string, cached bool) string {
	t.Helper()
	merged := NewOrderedMap(0)
	merger := newListMerger(merged, true)
	for i, doc := range docs {
		decoded, err := yamlDecode(doc)
		if err != nil {
			t.Fatalf("doc %d: unexpected decode error: %s", i, err)
		}
		if cached {
			err = merger.Merge(decoded)
		} else {
			_, err = MergeMaps(decoded, merged, true)
		}
		if err != nil {
			t.Fatalf("doc %d: unexpected merge error: %s", i, err)
		}
	}
	out, err := yamlEncode(merged)
	if err != nil {
		t.Fatalf("unexpected encode error: %s", err)
	}
	return out
}

func TestMergeMaps_AppendItemWithoutPrimitives(t *testing.T) {
	cases := []struct {
		name     string
		appended any
	}{
		{name: "primitive item", appended: "abc"},
		{name: "map item without primitives", appended: map[string]any{"map": map[string]any{"elem1": "value1"}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dst := map[string]any{"list": []any{map[string]any{"name": "b"}}}
			src := map[string]any{"list": []any{
				c.appended,
				map[string]any{"name": "a", "x": 1},
				map[string]any{"name": "a", "x": 2},
			}}
			result := map[string]any{"list": []any{
				map[string]any{"name": "b"},
				c.appended,
				map[string]any{"name": "a", "x": 1},
				map[string]any{"name": "a", "x": 2},
			}}
			if _, err := MergeMaps(src, dst, true); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !reflect.DeepEqual(dst, result) {
				t.Fatalf("Error matching dst and result: %#v vs %#v", dst, result)
			}
		})
	}

	// Source items after the appended item must not panic with index out of range
	got := mergeYAMLDocs(t, []string{
		"l:\n- name: a\n",
		"l:\n- x\n- name: b\n  v: 1\n- name: c\n  v: 1\n",
	}, false)
	want := "l:\n  - name: a\n  - x\n  - name: b\n    v: 1\n  - name: c\n    v: 1\n"
	if got != want {
		t.Fatalf("unexpected result:\n%s\nwant:\n%s", got, want)
	}
}

func TestListMerger_MergeIntroducesDuplicate(t *testing.T) {
	cases := []struct {
		name string
		docs []string
		want string
	}{
		{
			name: "flat",
			docs: []string{
				"list:\n  - name: x\n  - id: 1\n",
				// Merges into the first item, which then matches the second item
				"list:\n  - name: x\n    id: 1\n",
				"list:\n  - id: 1\n    c: 3\n",
			},
			want: "list:\n  - name: x\n    id: 1\n  - id: 1\n  - id: 1\n    c: 3\n",
		},
		{
			name: "nested",
			docs: []string{
				"tenants:\n  - name: t\n    list:\n      - name: x\n      - id: 1\n",
				"tenants:\n  - name: t\n    list:\n      - name: x\n        id: 1\n",
				"tenants:\n  - name: t\n    list:\n      - id: 1\n        c: 3\n",
			},
			want: "tenants:\n  - name: t\n    list:\n      - name: x\n        id: 1\n      - id: 1\n      - id: 1\n        c: 3\n",
		},
		{
			// Aliases are deep-copied by yamlDecode, so merging into one does not change the other
			name: "aliases",
			docs: []string{
				"a:\n  - &item\n    name: n\nb:\n  - *item\n  - id: 1\n",
				"b:\n  - name: z\na:\n  - name: n\n    id: 1\n",
				"b:\n  - id: 1\n    c: 3\n",
			},
			want: "a:\n  - name: \"n\"\n    id: 1\nb:\n  - name: \"n\"\n  - id: 1\n    c: 3\n  - name: z\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, cached := range []bool{false, true} {
				if got := mergeYAMLDocs(t, c.docs, cached); got != c.want {
					t.Fatalf("cached=%v: unexpected result:\n%s\nwant:\n%s", cached, got, c.want)
				}
			}
		})
	}
}

func TestListMerger_ContinueAfterError(t *testing.T) {
	docs := []string{
		"list:\n  - name: a\n    c: x\n  - id: 1\n",
		// Sets id in the first item, which then matches the second item, before
		// failing with a shape conflict for c
		"list:\n  - name: a\n    id: 1\n    c: [y]\n",
		// Must be appended, as the list now contains duplicates
		"list:\n  - id: 1\n    d: 2\n",
	}
	want := "list:\n  - name: a\n    c: x\n    id: 1\n  - id: 1\n  - id: 1\n    d: 2\n"

	for _, cached := range []bool{false, true} {
		merged := NewOrderedMap(0)
		merger := newListMerger(merged, true)
		for i, doc := range docs {
			decoded, err := yamlDecode(doc)
			if err != nil {
				t.Fatalf("doc %d: unexpected decode error: %s", i, err)
			}
			if cached {
				err = merger.Merge(decoded)
			} else {
				_, err = MergeMaps(decoded, merged, true)
			}
			if (err != nil) != (i == 1) {
				t.Fatalf("cached=%v: doc %d: unexpected error result: %v", cached, i, err)
			}
		}
		if got, _ := yamlEncode(merged); got != want {
			t.Fatalf("cached=%v: unexpected result:\n%s\nwant:\n%s", cached, got, want)
		}
	}
}

// randomScalar returns a random scalar or null value.
func randomScalar(rng *rand.Rand) any {
	return []any{"a", "b", 1, 2, nil}[rng.Intn(5)]
}

// randomMergeMap generates a random map. The shape of each value depends on its
// key (x: scalar, y: map, z: list), so that documents never conflict in shape.
func randomMergeMap(rng *rand.Rand, depth int) *OrderedMap {
	m := NewOrderedMap(2)
	if depth > 3 {
		return m
	}
	for i := 0; i < 2; i++ {
		switch key := string("xyz"[rng.Intn(3)]); key {
		case "x":
			m.Set(key, randomScalar(rng))
		case "y":
			m.Set(key, randomMergeMap(rng, depth+1))
		case "z":
			m.Set(key, randomMergeList(rng, depth+1))
		}
	}
	return m
}

// randomMergeList generates a random list whose items often share primitive
// values, so that they merge or become duplicates.
func randomMergeList(rng *rand.Rand, depth int) []any {
	items := []any{}
	if depth > 3 {
		return items
	}
	for n := rng.Intn(4); n > 0; n-- {
		switch r := rng.Float64(); {
		case r < 0.1:
			items = append(items, randomScalar(rng))
			continue
		case r < 0.15:
			// Map item without primitive values
			item := NewOrderedMap(1)
			item.Set("l", randomMergeList(rng, depth+1))
			items = append(items, item)
			continue
		}
		item := NewOrderedMap(3)
		item.Set([]string{"name", "id"}[rng.Intn(2)], string("ab"[rng.Intn(2)]))
		if rng.Float64() < 0.5 {
			item.Set("v", 1+rng.Intn(2))
		}
		if rng.Float64() < 0.5 {
			item.Set("l", randomMergeList(rng, depth+1))
		}
		items = append(items, item)
	}
	return items
}

// toPlainMaps converts all *OrderedMap values to map[string]any, as produced by
// convertDynamicToNative for the merge function.
func toPlainMaps(v any) any {
	switch val := v.(type) {
	case *OrderedMap:
		m := make(map[string]any, val.Len())
		for _, e := range val.Entries() {
			m[e.Key] = toPlainMaps(e.Value)
		}
		return m
	case []any:
		items := make([]any, len(val))
		for i, item := range val {
			items[i] = toPlainMaps(item)
		}
		return items
	default:
		return v
	}
}

// randomMergeDocs generates random documents for comparing cached and uncached
// merging. With conflicts, the key "c" has a random shape, so that some documents
// fail to merge.
func randomMergeDocs(seed int64, conflicts bool) []any {
	rng := rand.New(rand.NewSource(seed))
	var docs []any
	for i := 0; i < 6; i++ {
		root := NewOrderedMap(3)
		root.Set("list", randomMergeList(rng, 0))
		root.Set("map", randomMergeMap(rng, 0))
		if conflicts && rng.Float64() < 0.3 {
			root.Set("c", []any{randomScalar(rng), randomMergeMap(rng, 3), randomMergeList(rng, 3)}[rng.Intn(3)])
		}
		root.Set("list2", randomMergeList(rng, 0))
		doc := NewOrderedMap(1)
		doc.Set("root", root)
		docs = append(docs, doc)
	}
	return docs
}

func TestListMerger_MatchesSequentialMergeMaps(t *testing.T) {
	cases := []struct {
		name      string
		plain     bool
		conflicts bool
	}{
		{name: "ordered maps"},
		// Plain maps are iterated in random order, so documents must not fail
		// halfway, as the partially merged result would differ between runs
		{name: "plain maps", plain: true},
		// Merging continues after documents failing with a top-level conflict
		// (see TestListMerger_ContinueAfterError for failures inside list items)
		{name: "ordered maps with conflicts", conflicts: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			failed := 0
			for seed := int64(0); seed < 200; seed++ {
				docs := randomMergeDocs(seed, c.conflicts)
				var expected, actual any = NewOrderedMap(0), NewOrderedMap(0)
				if c.plain {
					for i := range docs {
						docs[i] = toPlainMaps(docs[i])
					}
					expected, actual = map[string]any{}, map[string]any{}
				}

				var expectedErrs, actualErrs []int
				for i, doc := range docs {
					if _, err := MergeMaps(deepCopy(doc), expected, true); err != nil {
						expectedErrs = append(expectedErrs, i)
					}
				}
				merger := newListMerger(actual, true)
				for i, doc := range docs {
					if err := merger.Merge(deepCopy(doc)); err != nil {
						actualErrs = append(actualErrs, i)
					}
				}

				if !reflect.DeepEqual(expectedErrs, actualErrs) {
					t.Fatalf("seed %d: failed documents %v without cache, %v with cache", seed, expectedErrs, actualErrs)
				}
				if len(expectedErrs) > 0 {
					if !c.conflicts {
						t.Fatalf("seed %d: unexpected merge errors in documents %v", seed, expectedErrs)
					}
					failed++
				}
				if !reflect.DeepEqual(expected, actual) {
					want, _ := yamlEncode(expected)
					got, _ := yamlEncode(actual)
					t.Fatalf("seed %d: cached result differs:\n%s\nwant:\n%s", seed, got, want)
				}
			}
			if c.conflicts && failed < 20 {
				t.Fatalf("only %d seeds with failed documents, expected at least 20", failed)
			}
		})
	}
}

func BenchmarkYamlMerge_ManyInputs(b *testing.B) {
	// Many documents adding items to the same large list, all sharing a common
	// primitive value, so that a full duplicate scan is expensive
	const inputs, itemsPerInput = 100, 10
	var docs []any
	for i := 0; i < inputs; i++ {
		items := make([]any, 0, itemsPerInput)
		for j := 0; j < itemsPerInput; j++ {
			item := NewOrderedMap(2)
			item.Set("name", fmt.Sprintf("item-%d-%d", i, j))
			item.Set("vrf", "default")
			items = append(items, item)
		}
		doc := NewOrderedMap(1)
		doc.Set("list", items)
		docs = append(docs, doc)
	}

	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprintf("cached=%v", cached), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				copies := make([]any, len(docs))
				for i, doc := range docs {
					copies[i] = deepCopy(doc)
				}
				merged := NewOrderedMap(0)
				merger := newListMerger(merged, true)
				b.StartTimer()
				for _, doc := range copies {
					var err error
					if cached {
						err = merger.Merge(doc)
					} else {
						_, err = MergeMaps(doc, merged, true)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
