package provider

import (
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

func TestOrderedMap_SetAndGet(t *testing.T) {
	m := NewOrderedMap(4)
	m.Set("b", 2)
	m.Set("a", 1)
	m.Set("c", 3)

	v, ok := m.Get("a")
	if !ok || v != 1 {
		t.Errorf("Get(a) = %v, %v; want 1, true", v, ok)
	}
	v, ok = m.Get("b")
	if !ok || v != 2 {
		t.Errorf("Get(b) = %v, %v; want 2, true", v, ok)
	}
	_, ok = m.Get("missing")
	if ok {
		t.Errorf("Get(missing) should return false")
	}
}

func TestOrderedMap_SetPreservesOrder(t *testing.T) {
	m := NewOrderedMap(4)
	m.Set("z", 1)
	m.Set("a", 2)
	m.Set("m", 3)

	entries := m.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Key != "z" || entries[1].Key != "a" || entries[2].Key != "m" {
		t.Errorf("order not preserved: got %v", entries)
	}
}

func TestOrderedMap_SetUpdateInPlace(t *testing.T) {
	m := NewOrderedMap(4)
	m.Set("b", 1)
	m.Set("a", 2)
	m.Set("b", 99) // update existing — should keep position 0

	entries := m.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "b" || entries[0].Value != 99 {
		t.Errorf("expected b=99 at position 0, got %s=%v", entries[0].Key, entries[0].Value)
	}
	if entries[1].Key != "a" || entries[1].Value != 2 {
		t.Errorf("expected a=2 at position 1, got %s=%v", entries[1].Key, entries[1].Value)
	}
}

func TestOrderedMap_Delete(t *testing.T) {
	m := NewOrderedMap(4)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	m.Delete("b")

	if m.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", m.Len())
	}
	if m.Has("b") {
		t.Errorf("b should be deleted")
	}
	entries := m.Entries()
	if entries[0].Key != "a" || entries[1].Key != "c" {
		t.Errorf("order not preserved after delete: got %v", entries)
	}
	// Verify index is correct after delete
	v, ok := m.Get("c")
	if !ok || v != 3 {
		t.Errorf("Get(c) after delete = %v, %v; want 3, true", v, ok)
	}
}

func TestOrderedMap_DeleteNonExistent(t *testing.T) {
	m := NewOrderedMap(2)
	m.Set("a", 1)
	m.Delete("missing") // should be a no-op
	if m.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", m.Len())
	}
}

func TestOrderedMap_Len(t *testing.T) {
	m := NewOrderedMap(0)
	if m.Len() != 0 {
		t.Errorf("expected 0, got %d", m.Len())
	}
	m.Set("a", 1)
	if m.Len() != 1 {
		t.Errorf("expected 1, got %d", m.Len())
	}
	m.Set("a", 2) // update, not insert
	if m.Len() != 1 {
		t.Errorf("expected 1 after update, got %d", m.Len())
	}
}

func TestOrderedMap_Has(t *testing.T) {
	m := NewOrderedMap(2)
	m.Set("a", 1)
	if !m.Has("a") {
		t.Errorf("Has(a) should be true")
	}
	if m.Has("b") {
		t.Errorf("Has(b) should be false")
	}
}

func TestOrderedMap_ToMap(t *testing.T) {
	m := NewOrderedMap(2)
	m.Set("a", 1)
	m.Set("b", "two")

	plain := m.ToMap()
	if len(plain) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(plain))
	}
	if plain["a"] != 1 || plain["b"] != "two" {
		t.Errorf("ToMap returned unexpected values: %v", plain)
	}
}

func TestToMapSlice_Simple(t *testing.T) {
	m := NewOrderedMap(3)
	m.Set("z", "first")
	m.Set("a", "second")
	m.Set("m", "third")

	result := toMapSlice(m)
	ms, ok := result.(goyaml.MapSlice)
	if !ok {
		t.Fatalf("expected MapSlice, got %T", result)
	}
	if len(ms) != 3 {
		t.Fatalf("expected 3 items, got %d", len(ms))
	}
	if ms[0].Key != "z" || ms[1].Key != "a" || ms[2].Key != "m" {
		t.Errorf("order not preserved in MapSlice: %v", ms)
	}
}

func TestToMapSlice_Nested(t *testing.T) {
	inner := NewOrderedMap(2)
	inner.Set("y", 1)
	inner.Set("x", 2)

	outer := NewOrderedMap(2)
	outer.Set("nested", inner)
	outer.Set("list", []any{"a", inner})

	result := toMapSlice(outer)
	ms := result.(goyaml.MapSlice)

	// Check nested OrderedMap was converted
	nestedMS, ok := ms[0].Value.(goyaml.MapSlice)
	if !ok {
		t.Fatalf("expected nested MapSlice, got %T", ms[0].Value)
	}
	if nestedMS[0].Key != "y" || nestedMS[1].Key != "x" {
		t.Errorf("nested order wrong: %v", nestedMS)
	}

	// Check list contains converted OrderedMap
	list, ok := ms[1].Value.([]any)
	if !ok {
		t.Fatalf("expected []any for list, got %T", ms[1].Value)
	}
	if list[0] != "a" {
		t.Errorf("list[0] = %v, want 'a'", list[0])
	}
	if _, ok := list[1].(goyaml.MapSlice); !ok {
		t.Fatalf("list[1] should be MapSlice, got %T", list[1])
	}
}
