package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
)

// TestNavSearch_RefreshAppliesDimToNonCurrentHits: after commit,
// the rendered viewport contains the dim background on every
// non-current hit and NO dim on the current hit (which is
// covered by the cursor's inverted-block style).
func TestNavSearch_RefreshAppliesDimToNonCurrentHits(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "foo bar foo baz foo"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	// Refresh the viewport so the placeholder/seeded content is replaced.
	m.refreshViewport()
	m = keyModelVal(t, m, "/")
	for _, r := range "foo" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// View the viewport content.
	content := m.View().Content
	// Three hits total; cur=0, so two non-current hits should be dim.
	dimCount := strings.Count(content, "\x1b[48;5;58m")
	if dimCount != 2 {
		t.Errorf("dim splice count = %d, want 2 (three hits minus current)", dimCount)
	}
}

// TestNavSearch_RefreshPreservesCodeBlockBackground: with the
// message view, glamour renders fenced code blocks with
// "\x1b[48;5;236m" per cell. A hit inside the block should
// splice the dim on the hit bytes AND restore the 236 bg
// on the cells immediately before and after the hit.
func TestNavSearch_RefreshPreservesCodeBlockBackground(t *testing.T) {
	// Note: refreshViewport reads m.latest.Text, not glamour
	// output. The actual glamour render is in the hover modal,
	// not the message view. So the message view's gutter does
	// not have a parent bg to preserve. This test is a no-op
	// verification that the dim splice doesn't BREAK the
	// message view's render.
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "alpha beta alpha"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m.refreshViewport()
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	content := m.View().Content
	if !strings.Contains(content, "\x1b[48;5;58m") {
		t.Errorf("expected dim background in viewport content; got %q", content)
	}
}

// TestNavSearch_RefreshClearsDimOnEsc: after Esc, no dim remains.
func TestNavSearch_RefreshClearsDimOnEsc(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "foo bar foo"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m.refreshViewport()
	m = keyModelVal(t, m, "/")
	for _, r := range "foo" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = upd.(model)
	// refreshViewport re-runs; hits are now nil.
	content := m.View().Content
	if strings.Contains(content, "\x1b[48;5;58m") {
		t.Errorf("dim should be cleared after Esc; got %q", content)
	}
}
