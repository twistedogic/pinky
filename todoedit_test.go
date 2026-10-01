package main

import (
	"path/filepath"
	"testing"

	"github.com/twistedogic/pinky/internal/todo"

	tea "charm.land/bubbletea/v2"
)

// TestTodoEdit_EnterSavesAndEscDiscards: Enter commits the edit
// buffer into the focused item's text and persists to disk; Esc
// discards the buffer without touching the item.
func TestTodoEdit_EnterSavesAndEscDiscards(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.tab = tabTodos
	m.state = stateTodoEdit
	m.todos = []todo.Item{{Text: "old"}, {Text: ""}}
	m.todoCursor = 1
	m.todoEdit = "fresh text"
	m.todoPath = filepath.Join(t.TempDir(), "todos.json")

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, _ := m.Update(enter)
	um := updated.(model)

	if um.state != stateTodoList {
		t.Errorf("after Enter: state = %v, want stateTodoList", um.state)
	}
	if um.todos[1].Text != "fresh text" {
		t.Errorf("after Enter: todos[1].Text = %q, want \"fresh text\"", um.todos[1].Text)
	}
	if um.todoEdit != "" {
		t.Errorf("after Enter: todoEdit = %q, want empty", um.todoEdit)
	}
	// Persisted to disk.
	loaded, err := todo.Load(um.todoPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 || loaded[1].Text != "fresh text" {
		t.Errorf("disk = %+v, want [old, fresh text]", loaded)
	}

	// Esc discards a fresh draft.
	um.state = stateTodoEdit
	um.todos[1].Text = "untouched"
	um.todoEdit = "garbage draft"
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	updated, _ = um.Update(esc)
	um = updated.(model)

	if um.state != stateTodoList {
		t.Errorf("after Esc: state = %v, want stateTodoList", um.state)
	}
	if um.todos[1].Text != "untouched" {
		t.Errorf("after Esc: todos[1].Text = %q, want \"untouched\" (untouched)", um.todos[1].Text)
	}
	if um.todoEdit != "" {
		t.Errorf("after Esc: todoEdit = %q, want empty", um.todoEdit)
	}
}

// TestTodoEdit_AppendsAndBackspaces: text accumulates via Text
// runes; backspace removes the last rune.
func TestTodoEdit_AppendsAndBackspaces(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateTodoEdit
	m.todos = []todo.Item{{Text: ""}}
	m.todoCursor = 0
	m.todoEdit = ""

	for _, r := range "hello" {
		k := tea.KeyPressMsg{Code: r, Text: string(r)}
		var tmp tea.Model
		tmp, _ = m.Update(k)
		m = tmp.(model)
	}
	if m.todoEdit != "hello" {
		t.Errorf("after typing: todoEdit = %q, want \"hello\"", m.todoEdit)
	}

	bs := tea.KeyPressMsg{Code: tea.KeyBackspace}
	var tmp tea.Model
	tmp, _ = m.Update(bs)
	m = tmp.(model)
	if m.todoEdit != "hell" {
		t.Errorf("after backspace: todoEdit = %q, want \"hell\"", m.todoEdit)
	}
}