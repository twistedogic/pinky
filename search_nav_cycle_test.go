package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
)

// TestNavSearch_NCyclesFromActiveSearch: when a search is active
// (Enter already committed), n advances the current hit and moves
// the cursor.
func TestNavSearch_NCyclesFromActiveSearch(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "foo bar foo baz foo"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	// Commit a search: type "foo" + Enter.
	m = keyModelVal(t, m, "/")
	for _, r := range "foo" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// Expect: cur=0, cursor on first "foo" (line 0, byte 0).
	if m.navSearch.cur != 0 {
		t.Fatalf("after Enter: cur=%d, want 0", m.navSearch.cur)
	}
	// n: advance.
	m = keyModelVal(t, m, "n")
	if m.navSearch.cur != 1 {
		t.Errorf("after n: cur=%d, want 1", m.navSearch.cur)
	}
	// Second n: advance again.
	m = keyModelVal(t, m, "n")
	if m.navSearch.cur != 2 {
		t.Errorf("after second n: cur=%d, want 2", m.navSearch.cur)
	}
	// Third n: wrap to 0.
	m = keyModelVal(t, m, "n")
	if m.navSearch.cur != 0 {
		t.Errorf("after wrap n: cur=%d, want 0", m.navSearch.cur)
	}
}

// TestNavSearch_NCyclesWrapsForward: same as above but a single
// explicit wrap test.
func TestNavSearch_NCyclesWrapsForward(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "a b a"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "a" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// Two hits in "a b a": at byte 0 and byte 4.
	if len(m.navSearch.hits) != 2 {
		t.Fatalf("hits=%d, want 2", len(m.navSearch.hits))
	}
	// n twice → wrap back to 0.
	m = keyModelVal(t, m, "n")
	if m.navSearch.cur != 1 {
		t.Errorf("n: cur=%d, want 1", m.navSearch.cur)
	}
	m = keyModelVal(t, m, "n")
	if m.navSearch.cur != 0 {
		t.Errorf("wrap: cur=%d, want 0", m.navSearch.cur)
	}
}

// TestNavSearch_NCyclesWrapsBack: N from cur=0 wraps to last.
func TestNavSearch_NCyclesWrapsBack(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "a b a"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "a" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// cur is 0; N goes to last (1).
	m = keyModelVal(t, m, "N")
	if m.navSearch.cur != 1 {
		t.Errorf("N: cur=%d, want 1", m.navSearch.cur)
	}
	m = keyModelVal(t, m, "N")
	if m.navSearch.cur != 0 {
		t.Errorf("N from 0: cur=%d, want 0", m.navSearch.cur)
	}
}

// TestNavSearch_NStillEntersComposeWhenNoSearchActive: n with
// no committed search enters compose mode (the existing
// behaviour).
func TestNavSearch_NStillEntersComposeWhenNoSearchActive(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "n")
	if m.state != stateCompose {
		t.Errorf("expected stateCompose after n (no search); got %d", m.state)
	}
}

// TestNavSearch_JKResetsCurrent: after committing a search,
// pressing j resets cur to -1.
func TestNavSearch_JKResetsCurrent(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "foo bar\nfoo baz"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "foo" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if m.navSearch.cur != 0 {
		t.Fatalf("after Enter: cur=%d, want 0", m.navSearch.cur)
	}
	// j moves cursor down; cur becomes -1.
	m = keyModelVal(t, m, "j")
	if m.navSearch.cur != -1 {
		t.Errorf("after j: cur=%d, want -1", m.navSearch.cur)
	}
	// hits should be preserved.
	if len(m.navSearch.hits) != 2 {
		t.Errorf("hits=%d, want 2 (preserved)", len(m.navSearch.hits))
	}
	// n: advance from the new cursor position.
	m = keyModelVal(t, m, "n")
	if m.navSearch.cur != 1 {
		t.Errorf("after j+n: cur=%d, want 1 (next hit at or after new cursor)", m.navSearch.cur)
	}
}
