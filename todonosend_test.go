package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/todo"
)

// TestTodo_SIsNoOp: pressing `s` in stateTodoList and stateTodoEdit
// does not trigger an inject call and does not modify the todo
// list or any comments staged for sending.
func TestTodo_SIsNoOp(t *testing.T) {
	for _, st := range []state{stateTodoList, stateTodoEdit} {
		t.Run(stateName(st), func(t *testing.T) {
			m := newIdleModelForKeymap(t)
			m.tab = tabTodos
			m.state = st
			m.todos = []todo.Item{{Text: "alpha"}, {Text: "beta"}}
			m.todoCursor = 0
			// Stage a comment so any unintended send path would
			// flush it; we assert it stays put after the press.
			m.comments = []render.Comment{{
				Kind:    render.CommentMessage,
				ByteA:   0,
				ByteC:   5,
				Source:  "alpha",
				Text:    "should-not-send",
			}}

			beforeLen := len(m.comments)
			beforeTodos := len(m.todos)
			beforeState := m.state
			beforeTab := m.tab

			s := tea.KeyPressMsg{Code: rune('s'), Text: "s"}
			updated, cmd := m.Update(s)
			um := updated.(model)

			if cmd != nil {
				t.Errorf("cmd = %v, want nil (s should be a no-op)", cmd)
			}
			if um.state != beforeState {
				t.Errorf("state changed: got %v, want %v", um.state, beforeState)
			}
			if um.tab != beforeTab {
				t.Errorf("tab changed: got %v, want %v", um.tab, beforeTab)
			}
			if len(um.comments) != beforeLen {
				t.Errorf("comments flushed: got %d, want %d", len(um.comments), beforeLen)
			}
			if len(um.todos) != beforeTodos {
				t.Errorf("todos modified: got %d, want %d", len(um.todos), beforeTodos)
			}
		})
	}
}