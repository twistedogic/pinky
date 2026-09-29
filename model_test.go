package main

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// initCommentTAForTest mirrors m.initCommentComposer() so tests
// that bypass attach() still have a usable comment textarea.
func initCommentTAForTest(m *model) {
	ta := textarea.New()
	ta.Placeholder = "comment — Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.SetWidth(80)
	m.commentTa = ta
}

// TestFileViewer_ColumnCursor_ClampsAtLineEnd: pressing `j` to
// a shorter line clamps charPos to the new line's byte length.
// Spec D3 / scenario "j clamps the column when the new line is
// shorter". `preferred` must also be set so j/k consult the right
// target; the test sets preferred=5 directly.
func TestFileViewer_ColumnCursor_ClampsAtLineEnd(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	// content: "alpha\nbeta\ngamma\n" — "alpha" has 5 bytes,
	// "beta" has 4. Move cursor to col 5 on line 1 then `j` to
	// land on "beta": charPos should clamp to 4.
	m.fileViewer.charPos = 5
	m.fileViewer.preferred = 5
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 4 {
		t.Errorf("after j (alpha→beta): charPos = %d; want 4", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 5 {
		t.Errorf("after j: preferred = %d; want 5 (unchanged)", m.fileViewer.preferred)
	}
}

// TestFileViewer_ColumnCursor_PreservedAcrossLineMove: pressing
// `j` to a line longer than charPos leaves charPos unchanged.
// Spec D3 / scenario "j preserves the column when possible".
func TestFileViewer_ColumnCursor_PreservedAcrossLineMove(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	m.fileViewer.charPos = 3
	m.fileViewer.preferred = 3
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	// "beta" (4 bytes) and "gamma" (5 bytes) both have ≥3 bytes,
	// so charPos stays at 3.
	if m.fileViewer.charPos != 3 {
		t.Errorf("after j: charPos = %d; want 3", m.fileViewer.charPos)
	}
}

// TestFileViewer_ColumnCursor_PreservesVisualRange: h/l must
// move charPos both inside and outside visual mode, and visual
// mode's CharC must track charPos.
func TestFileViewer_ColumnCursor_PreservesVisualRange(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	// outside visual: `l` advances charPos
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 1 {
		t.Errorf("after l outside visual: charPos = %d; want 1", m.fileViewer.charPos)
	}
	// enter visual: cursor goes to (1, charPos)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	if !m.fileViewer.visual.Active {
		t.Fatal("expected visual mode active after v")
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 2 {
		t.Errorf("after l in visual: charPos = %d; want 2", m.fileViewer.charPos)
	}
}

// TestFileViewer_ColumnCursor_OpensAtZero: openFileViewer sets
// charPos to 0. Spec D3 / scenario "Opening a file sets the
// column to 0".
func TestFileViewer_ColumnCursor_OpensAtZero(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	if m.fileViewer.charPos != 0 {
		t.Errorf("charPos after open = %d; want 0", m.fileViewer.charPos)
	}
}

// updatedModelPtr extracts a *model from a tea.Cmd return value.
// Most Update paths in pinky return `m, nil` (no model value),
// but the *tea.Model return contract is what callers use; this
// helper is the bridge for tests that drive Update through the
// public surface and want to keep mutating m.
func updatedModelPtr(v tea.Model) *model {
	if v == nil {
		return nil
	}
	m, ok := v.(model)
	if !ok {
		return nil
	}
	return &m
}