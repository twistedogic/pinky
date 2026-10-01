package main

import (
	"path/filepath"
	"testing"

	"github.com/twistedogic/pinky/internal/todo"

	tea "charm.land/bubbletea/v2"
)

// TestTodoList_CursorMoves: j / k move the focused cursor,
// clamped at the list ends.
func TestTodoList_CursorMoves(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateTodoList
	m.todos = []todo.Item{
		{Text: "a"},
		{Text: "b"},
		{Text: "c"},
	}
	m.todoCursor = 0

	j := tea.KeyPressMsg{Code: rune('j'), Text: "j"}
	updated, _ := m.Update(j)
	um := updated.(model)
	if um.todoCursor != 1 {
		t.Errorf("after j: todoCursor = %d, want 1", um.todoCursor)
	}

	updated, _ = um.Update(j)
	um = updated.(model)
	if um.todoCursor != 2 {
		t.Errorf("after j again: todoCursor = %d, want 2", um.todoCursor)
	}

	// Clamped at end.
	updated, _ = um.Update(j)
	um = updated.(model)
	if um.todoCursor != 2 {
		t.Errorf("j at end: todoCursor = %d, want 2 (clamped)", um.todoCursor)
	}

	k := tea.KeyPressMsg{Code: rune('k'), Text: "k"}
	updated, _ = um.Update(k)
	um = updated.(model)
	if um.todoCursor != 1 {
		t.Errorf("after k: todoCursor = %d, want 1", um.todoCursor)
	}

	updated, _ = um.Update(k)
	um = updated.(model)
	updated, _ = um.Update(k)
	um = updated.(model)
	if um.todoCursor != 0 {
		t.Errorf("k clamped at 0: todoCursor = %d, want 0", um.todoCursor)
	}
}

// TestTodoList_SpaceTogglesDone: space flips done on the focused
// item and persists to disk.
func TestTodoList_SpaceTogglesDone(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateTodoList
	m.todos = []todo.Item{{Text: "x"}}
	m.todoCursor = 0
	m.todoPath = filepath.Join(t.TempDir(), "todos.json")

	space := tea.KeyPressMsg{Code: rune(' '), Text: string(' ')}
	updated, _ := m.Update(space)
	um := updated.(model)

	if !um.todos[0].Done {
		t.Errorf("after space: done = false, want true")
	}
	if um.todoPath == "" {
		t.Fatal("todoPath not set")
	}
	// Persisted to disk.
	loaded, err := todo.Load(um.todoPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 || !loaded[0].Done {
		t.Errorf("loaded = %+v, want one item with done=true", loaded)
	}
}

// TestTodoList_DeleteRemovesItem: d removes the focused item,
// cursor clamps to the new last (or 0 if empty), persists.
func TestTodoList_DeleteRemovesItem(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateTodoList
	m.todos = []todo.Item{
		{Text: "first"},
		{Text: "second"},
		{Text: "third"},
	}
	m.todoCursor = 1
	m.todoPath = filepath.Join(t.TempDir(), "todos.json")

	d := tea.KeyPressMsg{Code: rune('d'), Text: "d"}
	updated, _ := m.Update(d)
	um := updated.(model)

	if len(um.todos) != 2 {
		t.Errorf("len(todos) = %d, want 2", len(um.todos))
	}
	if um.todos[0].Text != "first" || um.todos[1].Text != "third" {
		t.Errorf("todos = %+v, want [first, third]", um.todos)
	}
}

// TestTodoList_AddOpensEdit: a appends a blank item and enters
// stateTodoEdit.
func TestTodoList_AddOpensEdit(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateTodoList
	m.todos = []todo.Item{{Text: "existing"}}
	m.todoCursor = 0

	a := tea.KeyPressMsg{Code: rune('a'), Text: "a"}
	updated, _ := m.Update(a)
	um := updated.(model)

	if um.state != stateTodoEdit {
		t.Errorf("after a: state = %v, want stateTodoEdit", um.state)
	}
	if len(um.todos) != 2 {
		t.Errorf("len(todos) = %d, want 2", len(um.todos))
	}
	if um.todos[1].Text != "" {
		t.Errorf("new item text = %q, want empty", um.todos[1].Text)
	}
	if um.todoCursor != 1 {
		t.Errorf("todoCursor = %d, want 1 (the new item)", um.todoCursor)
	}
}

// TestTodoList_EscReturnsToMessage: Esc from stateTodoList jumps
// to the message tab.
func TestTodoList_EscReturnsToMessage(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.tab = tabTodos
	m.state = stateTodoList

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	updated, _ := m.Update(esc)
	um := updated.(model)

	if um.tab != tabMessage {
		t.Errorf("after Esc: tab = %v, want tabMessage", um.tab)
	}
	if um.state != stateNav {
		t.Errorf("after Esc: state = %v, want stateNav", um.state)
	}
}