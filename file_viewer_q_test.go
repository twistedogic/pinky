package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestFileView_QReturnsToFileNav: pressing q in the file viewer
// returns to the file navigator (stateFileNav). Hard quit is no
// longer the right action — the user is mid-review and may want to
// open a different one. Mirrors the Esc -> exitFileViewer path.
func TestFileView_QReturnsToFileNav(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	if m.state != stateFileView {
		t.Fatalf("setup: expected stateFileView; got %v", m.state)
	}

	upd, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = updatedModelPtr(upd)

	if cmd != nil {
		t.Errorf("q should return to nav (no cmd); got %T", cmd)
	}
	if m.state != stateFileNav {
		t.Errorf("q should land in stateFileNav; got %v", m.state)
	}
}

// TestFileView_QExitsVisualToo: q in the file viewer with visual
// mode active still returns to the file nav (visual is cleared as
// part of exitFileViewer; user doesn't need two presses).
func TestFileView_QExitsVisualToo(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	if !m.fileViewer.visual.Active {
		t.Fatalf("setup: expected visual active after v")
	}

	upd, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = updatedModelPtr(upd)

	if m.state != stateFileNav {
		t.Errorf("q should land in stateFileNav; got %v", m.state)
	}
	if m.fileViewer.visual.Active {
		t.Errorf("visual should be cleared after q; still active")
	}
}