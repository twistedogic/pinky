package todo

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")

	items := []Item{
		{Text: "first", Done: false},
		{Text: "second", Done: true},
		{Text: "third", Done: false},
	}

	if err := Save(path, items); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(got) != len(items) {
		t.Fatalf("len: got %d, want %d", len(got), len(items))
	}
	for i := range items {
		if got[i] != items[i] {
			t.Errorf("item %d: got %+v, want %+v", i, got[i], items[i])
		}
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	_, err := Load(path)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err: got %v, want ErrNotFound", err)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "todos.json")

	if err := Save(path, []Item{{Text: "x"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Text != "x" {
		t.Errorf("got %+v, want one item with text=x", got)
	}
}

func TestLoadEmptyList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")

	if err := Save(path, []Item{}); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d items, want 0", len(got))
	}
}