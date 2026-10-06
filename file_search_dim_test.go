package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestFileSearch_DimOnNonCurrentHits: after committing a search,
// the rendered file viewer contains the dim background on every
// non-current hit.
func TestFileSearch_DimOnNonCurrentHits(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta alpha\nalpha gamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	view := m.fileViewView()
	dimCount := strings.Count(view, "\x1b[48;5;58m")
	// 3 hits total; cur=0 → 2 non-current.
	if dimCount != 2 {
		t.Errorf("dim count = %d, want 2 (three hits minus current)", dimCount)
	}
}

// TestFileSearch_MultipleHitsOnOneLine: two hits on the same
// line, neither current (cur=0, hits on lines 0 and 1; cur=0
// is on line 0, but the dim splice only applies to non-current
// hits). Two hits on different lines; if both are non-current,
// both get dim.
func TestFileSearch_MultipleHitsOnOneLine(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha alpha\nbeta\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// Two hits on line 0; cur=0 (first hit), so the second
	// should be dim.
	view := m.fileViewView()
	if !strings.Contains(view, "\x1b[48;5;58m") {
		t.Errorf("expected dim on second 'alpha' on line 1; view:\n%s", view)
	}
}

// TestFileSearch_DimOverlapsSelection: when a hit byte is
// inside an active visual selection, the dim splice and the
// selection's cyan both target the cell; last-write-wins is OK
// (the cell shows one or the other; the cursor's pass at the
// end is the third writer).
func TestFileSearch_DimOverlapsSelection(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha beta gamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	// Enter visual on the first line and select "alpha".
	m.fileViewer.cursor = 1
	m.fileViewer.charPos = 0
	m.fileViewer.visual.Active = true
	m.fileViewer.visual.LineA = 1
	m.fileViewer.visual.CharA = 0
	m.fileViewer.charPos = 5 // select "alpha"
	m.refreshFileView()
	// Now search "alpha" — the hit IS the selection.
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// The current hit is on the selection. The dim shouldn't
	// visibly change the selection (cursor already paints the
	// current hit). Just ensure no panic and view renders.
	view := m.fileViewView()
	if view == "" {
		t.Errorf("expected non-empty view")
	}
}

// TestFileSearch_DimClearsOnEsc: after Esc, no dim remains in
// the rendered file viewer.
func TestFileSearch_DimClearsOnEsc(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = upd.(model)
	view := m.fileViewView()
	if strings.Contains(view, "\x1b[48;5;58m") {
		t.Errorf("dim should be cleared after Esc; view:\n%s", view)
	}
}
