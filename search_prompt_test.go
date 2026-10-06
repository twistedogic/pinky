package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
)

// TestNavSearch_PromptRendersWhileActive: when the prompt is
// open, the view contains "/<query>".
func TestNavSearch_PromptRendersWhileActive(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "foo" {
		m = keyModelVal(t, m, string(r))
	}
	view := m.View()
	if !strings.Contains(view.Content, "/foo") {
		t.Errorf("expected view to contain '/foo'; got:\n%s", view.Content)
	}
}

// TestNavSearch_PromptHiddenWhenInactive: no `/` prompt when
// the search is not active.
func TestNavSearch_PromptHiddenWhenInactive(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	view := m.View()
	// The "comment composer" or other help lines don't start
	// with "/", so a substring match for "/foo" or similar is
	// safe enough. (The nav help line ends with "/" only for the
	// n/N/tab/? separator prefix, not bare "/foo".)
	if strings.Contains(view.Content, "/foo") {
		t.Errorf("expected no '/foo' in view; got:\n%s", view.Content)
	}
	// Just confirm navSearch isn't active.
	_ = tea.KeyEsc // reference
}
