package main

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// TestNavKey_Q_Quits: pressing `q` in nav state quits the program.
func TestNavKey_Q_Quits(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Error("expected tea.Quit when q is pressed in nav state")
	}
	if updated.(model).state != stateNav {
		t.Errorf("state = %d want stateNav", updated.(model).state)
	}
}

// TestNavKey_N_EntersCompose: pressing `n` in nav state transitions
// to compose mode. (Old design used `c` here; per change, `c` is
// block-level comment composer.)
func TestNavKey_N_EntersCompose(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
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

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
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

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
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

// TestHelpKey_TogglesShowAll verifies `?` flips the help between the
// short (one-line) and full (multi-column) footer. The footer is
// always visible; the toggle just switches its verbosity.
func TestHelpKey_TogglesShowAll(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyPressMsg{Text: "?"})
	if !updated.(model).help.ShowAll {
		t.Error("expected help.ShowAll=true after pressing ?")
	}
	updated2, _ := updated.(model).Update(tea.KeyPressMsg{Text: "?"})
	if updated2.(model).help.ShowAll {
		t.Error("expected help.ShowAll=false after pressing ? again")
	}
}

// newIdleModelForKeymap builds a minimal model ready for key tests.
// Initializes a textarea so the cursor/blink machinery is wired up
// and the comment composer so tests that open it don't nil-deref.
func newIdleModelForKeymap(t *testing.T) model {
	t.Helper()
	ta := textarea.New()
	ta.SetHeight(composeHeight)
	ta.SetWidth(80)
	ta.Focus()
	cta := textarea.New()
	cta.SetHeight(composeHeight)
	cta.SetWidth(80)
	return model{
		state:     stateNav,
		viewport:  viewport.New(viewport.WithWidth(80), viewport.WithHeight(20)),
		textarea:  ta,
		commentTa: cta,
		width:     80,
		height:    24,
	}
}
