package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// TestIdleKey_Q_Quits: pressing `q` in idle state quits the program.
func TestIdleKey_Q_Quits(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected tea.Quit when q is pressed in idle state")
	}
	if updated.(model).state != stateIdle {
		t.Errorf("state = %d want stateIdle", updated.(model).state)
	}
}

// TestIdleKey_C_EntersCompose: pressing `c` in idle state transitions
// to compose mode (vim-style alias for Ctrl+N).
func TestIdleKey_C_EntersCompose(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil {
		t.Errorf("did not expect a cmd on entering compose; got %v", cmd)
	}
	if updated.(model).state != stateCompose {
		t.Errorf("state = %d want stateCompose", updated.(model).state)
	}
}

// TestPickerKey_Q_Quits: pressing `q` in picker state quits.
func TestPickerKey_Q_Quits(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = statePicking

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected tea.Quit when q is pressed in picker state")
	}
	if updated.(model).state != statePicking {
		t.Errorf("state should remain statePicking; got %d", updated.(model).state)
	}
}

// TestComposeKey_C_TypesC: pressing `c` in compose state types the
// letter 'c' (don't intercept vim-style keymaps in input mode).
func TestComposeKey_C_TypesC(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateCompose

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		// textarea.Update returns a command; absence would mean
		// the rune was swallowed instead of typed.
		t.Error("expected a cmd from textarea.Update for the typed c")
	}
	if updated.(model).state != stateCompose {
		t.Errorf("state should remain stateCompose; got %d", updated.(model).state)
	}
	// The textarea should now contain 'c'.
	if got := updated.(model).textarea.Value(); got != "c" {
		t.Errorf("textarea.Value() = %q want %q", got, "c")
	}
}

// TestHelpKey_TogglesHelpOverlay verifies `?` toggles the help
// markdown in the viewport. The help markdown replaces the agent
// message in the viewport when toggled on; another keypress
// dismisses it and restores the message.
func TestHelpKey_TogglesHelpOverlay(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !updated.(model).help {
		t.Error("expected help=true after pressing ?")
	}
	updated2, _ := updated.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if updated2.(model).help {
		t.Error("expected help=false after pressing ? again")
	}
}

// newIdleModelForKeymap builds a minimal model ready for key tests.
// Initializes a textarea so the cursor/blink machinery is wired up.
func newIdleModelForKeymap(t *testing.T) model {
	t.Helper()
	ta := textarea.New()
	ta.SetHeight(composeHeight)
	ta.SetWidth(80)
	ta.Focus()
	return model{
		state:    stateIdle,
		viewport: viewport.New(80, 20),
		textarea: ta,
		width:    80,
		height:   24,
	}
}
