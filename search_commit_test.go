package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
)

// TestNavSearch_EnterJumpsToFirstHit: typing "world" and Enter
// moves the cursor to the first match's byte.
func TestNavSearch_EnterJumpsToFirstHit(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world foo bar"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	m = keyModelVal(t, m, "/")
	for _, r := range "world" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// "hello world foo bar" — "world" is at byte 6 on line 0.
	if m.cursor.LineIdx != 0 {
		t.Errorf("cursor.LineIdx = %d, want 0", m.cursor.LineIdx)
	}
	if m.cursor.CharPos != 6 {
		t.Errorf("cursor.CharPos = %d, want 6", m.cursor.CharPos)
	}
	if m.navSearch.active {
		t.Errorf("expected active=false after Enter")
	}
	if len(m.navSearch.hits) != 1 {
		t.Errorf("hits = %d, want 1", len(m.navSearch.hits))
	}
	if m.navSearch.cur != 0 {
		t.Errorf("cur = %d, want 0", m.navSearch.cur)
	}
}

// TestNavSearch_EnterWithNoMatchesNoOp: typing "zzz" and Enter
// when no match exists leaves the cursor where it is and sets
// cur = -1.
func TestNavSearch_EnterWithNoMatchesNoOp(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "hello world"},
	}})
	m = updated.(model)
	updated, _ = m.Update(sessionMsg{entries: nil})
	m = updated.(model)
	startLine, startChar := m.cursor.LineIdx, m.cursor.CharPos
	m = keyModelVal(t, m, "/")
	for _, r := range "zzz" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if m.cursor.LineIdx != startLine || m.cursor.CharPos != startChar {
		t.Errorf("cursor moved unexpectedly: was (%d, %d), now (%d, %d)",
			startLine, startChar, m.cursor.LineIdx, m.cursor.CharPos)
	}
	if m.navSearch.cur != -1 {
		t.Errorf("cur = %d, want -1", m.navSearch.cur)
	}
	if len(m.navSearch.hits) != 0 {
		t.Errorf("hits = %d, want 0", len(m.navSearch.hits))
	}
}

// TestFileSearch_EnterJumpsToFirstHitAndResetsPreferred: typing
// "beta" and Enter in the file viewer moves the cursor to the
// "beta" line, resets preferred to the hit's byte.
func TestFileSearch_EnterJumpsToFirstHitAndResetsPreferred(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	// Move cursor to line 1 first, set a non-zero preferred.
	m.fileViewer.cursor = 1
	m.fileViewer.charPos = 0
	m.fileViewer.preferred = 5
	m = keyModelVal(t, m, "/")
	for _, r := range "beta" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// "alpha\nbeta\ngamma\n" — "beta" is at line 2 (1-based) byte 0.
	if m.fileViewer.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.fileViewer.cursor)
	}
	if m.fileViewer.charPos != 0 {
		t.Errorf("charPos = %d, want 0", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 0 {
		t.Errorf("preferred = %d, want 0 (reset to hit byte)", m.fileViewer.preferred)
	}
}

// TestFileSearch_EnterWithNoMatchesNoOp: empty hits leaves
// cursor and preferred unchanged.
func TestFileSearch_EnterWithNoMatchesNoOp(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m.fileViewer.cursor = 1
	m.fileViewer.charPos = 0
	m.fileViewer.preferred = 3
	startLine, startChar, startPref := m.fileViewer.cursor, m.fileViewer.charPos, m.fileViewer.preferred
	m = keyModelVal(t, m, "/")
	for _, r := range "zzz" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if m.fileViewer.cursor != startLine || m.fileViewer.charPos != startChar || m.fileViewer.preferred != startPref {
		t.Errorf("cursor/preferred changed unexpectedly")
	}
	if m.fileSearchState.cur != -1 {
		t.Errorf("cur = %d, want -1", m.fileSearchState.cur)
	}
}
