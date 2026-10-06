package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
)

// TestNavSearch_NewMessageClearsSearch: when a new assistant
// message arrives (Text differs from the prior m.latest.Text),
// the search state is reset.
func TestNavSearch_NewMessageClearsSearch(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "old message"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	// Commit a search.
	m = keyModelVal(t, m, "/")
	for _, r := range "msg" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if len(m.navSearch.hits) == 0 {
		t.Fatalf("setup: hits should be populated after Enter")
	}
	// New message arrives.
	updated, _ = m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "new message"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	if m.navSearch.active {
		t.Errorf("new message should close the prompt")
	}
	if len(m.navSearch.hits) != 0 {
		t.Errorf("new message should clear hits; got %d", len(m.navSearch.hits))
	}
	if m.navSearch.cur != -1 {
		t.Errorf("new message should reset cur; got %d", m.navSearch.cur)
	}
}
