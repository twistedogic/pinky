package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
)

// keyModelVal drives a single key press against a value-typed
// model and returns the updated value.
func keyModelVal(t *testing.T, m model, key string) model {
	t.Helper()
	r := rune(key[0])
	upd, _ := m.Update(tea.KeyPressMsg{Code: r, Text: key})
	return upd.(model)
}

// TestNavSearch_TypingAppendsRunes: printable runes (including
// j/k/h/l/c/s) go to the query while the prompt is open.
func TestNavSearch_TypingAppendsRunes(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	if !m.navSearch.active {
		t.Fatalf("expected navSearch.active=true after /")
	}
	for _, r := range "func" {
		m = keyModelVal(t, m, string(r))
	}
	if got := string(m.navSearch.query); got != "func" {
		t.Errorf("query = %q, want %q", got, "func")
	}
}

// TestNavSearch_BackspaceTrims: Backspace drops the last rune.
func TestNavSearch_BackspaceTrims(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "fun" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = upd.(model)
	if got := string(m.navSearch.query); got != "fu" {
		t.Errorf("query = %q, want %q", got, "fu")
	}
}

// TestNavSearch_EscClears: Esc closes the prompt and clears
// the query.
func TestNavSearch_EscClears(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "hi" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = upd.(model)
	if m.navSearch.active {
		t.Errorf("expected navSearch.active=false after Esc")
	}
	if len(m.navSearch.query) != 0 {
		t.Errorf("expected query cleared; got %q", string(m.navSearch.query))
	}
}

// TestFileSearch_TypingAppendsRunes: printable runes go to the
// file viewer search query while the prompt is open.
func TestFileSearch_TypingAppendsRunes(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	if !m.fileSearchState.active {
		t.Fatalf("expected fileSearchState.active=true after /")
	}
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	if got := string(m.fileSearchState.query); got != "alpha" {
		t.Errorf("query = %q, want %q", got, "alpha")
	}
}

// TestFileSearch_BackspaceTrims: Backspace drops the last rune.
func TestFileSearch_BackspaceTrims(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = upd.(model)
	if got := string(m.fileSearchState.query); got != "alph" {
		t.Errorf("query = %q, want %q", got, "alph")
	}
}

// TestFileSearch_EscClears: Esc closes the prompt.
func TestFileSearch_EscClears(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "hi" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = upd.(model)
	if m.fileSearchState.active {
		t.Errorf("expected fileSearchState.active=false after Esc")
	}
	if len(m.fileSearchState.query) != 0 {
		t.Errorf("expected query cleared; got %q", string(m.fileSearchState.query))
	}
}
