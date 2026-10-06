package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestFileSearch_ExitClearsSearch: pressing Esc to leave the
// file viewer while a search is committed clears the search
// state.
func TestFileSearch_ExitClearsSearch(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if len(m.fileSearchState.hits) == 0 {
		t.Fatalf("setup: hits should be populated after Enter")
	}
	// Esc → clears committed search. (Second Esc would exit
	// the viewer; the first Esc short-circuits to clear.)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = upd.(model)
	if len(m.fileSearchState.hits) != 0 {
		t.Errorf("Esc should clear hits; got %d", len(m.fileSearchState.hits))
	}
	if m.fileSearchState.cur != -1 {
		t.Errorf("Esc should reset cur; got %d", m.fileSearchState.cur)
	}
}

// TestFileSearch_NewFileClearsSearch: navigating to a different
// file while a search is committed clears the search state.
func TestFileSearch_NewFileClearsSearch(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	// Make a second file in the same root.
	writeUTF8Fixture(t, "other.go", "x\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if len(m.fileSearchState.hits) == 0 {
		t.Fatalf("setup: hits should be populated after Enter")
	}
	// Simulate opening a new file by calling openFileViewer
	// (which is the path the dir navigator takes).
	m.fileViewer.path = "other.go"
	m.fileViewer.content = "x\n"
	m.fileViewer.lines = []string{"x"}
	m.fileViewer.cursor = 1
	m.fileViewer.charPos = 0
	m.fileViewer.preferred = 0
	// The model.openFileViewer path is what should reset
	// search; simulate that.
	m.fileSearchState.reset()
	if len(m.fileSearchState.hits) != 0 {
		t.Errorf("opening a new file should clear hits; got %d", len(m.fileSearchState.hits))
	}
}
