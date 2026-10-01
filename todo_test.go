package main

import (
	"path/filepath"
	"testing"

	"github.com/twistedogic/pinky/internal/todo"
)

// TestTodo_LoadOnAttach: after attach, m.todos is loaded from
// ~/.local/share/pinky/<base(cwd)>/todos.json. Regression for
// "first run / no file" should yield an empty list.
func TestTodo_LoadOnAttach(t *testing.T) {
	// Pre-write a todos.json at the path attach() should look at.
	home := t.TempDir()
	cwd := "/some/project"
	base := filepath.Base(cwd)
	path := filepath.Join(home, base, "todos.json")
	want := []todo.Item{
		{Text: "review the diff"},
		{Text: "check tests", Done: true},
	}
	if err := todo.Save(path, want); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	m := newModel()
	m.fileRoot = cwd
	m.todoPath = path
	m.refreshTodos()

	if len(m.todos) != len(want) {
		t.Fatalf("len(m.todos): got %d, want %d (m.todos=%+v)", len(m.todos), len(want), m.todos)
	}
	for i := range want {
		if m.todos[i] != want[i] {
			t.Errorf("item %d: got %+v, want %+v", i, m.todos[i], want[i])
		}
	}
}

// TestTodo_LoadOnAttach_MissingFile: a missing file yields an
// empty list (no error surfaced to the user).
func TestTodo_LoadOnAttach_MissingFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "emptyproject", "todos.json")

	m := newModel()
	m.fileRoot = "/some/emptyproject"
	m.todoPath = path
	m.refreshTodos()

	if len(m.todos) != 0 {
		t.Errorf("m.todos: got %+v, want empty", m.todos)
	}
}