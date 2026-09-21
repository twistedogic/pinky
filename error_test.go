package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/session"
)

func TestAttachFailure_ShowsErrorInTUI(t *testing.T) {
	// Simulate a picker whose attach will fail because we point it at
	// a non-existent pane id. The session.Open call will return an
	// error; pinky should display that error in the TUI rather than
	// quitting.
	m := newModel()
	m.agents = []session.AgentSession{{Session: "s", Window: "0", Pane: "0", PaneID: "%999", Agent: "pi"}}
	m.pickCursor = 0
	m.width = 80
	m.height = 24
	m.state = statePicking

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)

	if got.state != stateError {
		t.Errorf("after attach failure, state = %d want stateError (%d)", got.state, stateError)
	}
	if got.err == nil {
		t.Error("expected err to be set after attach failure")
	}
	// No quit command yet — we wait for the user to dismiss the error.
	if cmd != nil {
		t.Errorf("did not expect a cmd on enter into error state; got %v", cmd)
	}

	// View should render the error.
	view := got.View()
	if !strings.Contains(strings.ToLower(view), "error") {
		t.Errorf("expected error message in view; got: %q", view)
	}

	// Now any key quits.
	updated2, cmd2 := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd2 == nil {
		t.Error("expected tea.Quit when user dismisses the error")
	}
	if updated2.(model).state != stateError {
		t.Errorf("state should remain stateError until quit; got %d", updated2.(model).state)
	}
}

func TestErrorState_QuitsOnAnyKey(t *testing.T) {
	m := newModel()
	m.err = errors.New("something went wrong")
	m.state = stateError
	m.width = 80
	m.height = 24
	m.viewport = viewport.New(80, 20)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected tea.Quit on any key in error state")
	}
	if updated.(model).state != stateError {
		// State stays in error until quit; the cmd handles exit.
		t.Errorf("state should stay stateError; got %d", updated.(model).state)
	}
}

func TestErrorState_QuitsOnCtrlC(t *testing.T) {
	m := newModel()
	m.err = errors.New("boom")
	m.state = stateError

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit cmd on Ctrl+C in error state")
	}
	if updated.(model).state != stateError {
		t.Errorf("state = %d want stateError", updated.(model).state)
	}
}

// silence unused import
var _ tea.Model = model{}
