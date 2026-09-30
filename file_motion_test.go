package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// writeUTF8Fixture writes a fixture file with the given content
// and returns its (root, relative-path). Used by the rune-clamp
// motion tests below.
func writeUTF8Fixture(t *testing.T, rel, content string) (string, string) {
	t.Helper()
	root := t.TempDir()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, rel
}

// TestFileView_JPreservesPreferredColumn: pressing `l` enough
// times to set `preferred`, then `j`, lands on the next line at
// `min(preferred, len(line))`. Spec D3 / scenario "j preserves
// the column when possible".
func TestFileView_JPreservesPreferredColumn(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	m := openFileInFixture(t, root, rel)
	// l l l l → preferred becomes 4 (the highest column reached)
	for range 4 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.preferred != 4 {
		t.Fatalf("setup: preferred = %d; want 4", m.fileViewer.preferred)
	}
	// j lands on "beta" (4 bytes); charPos becomes 4.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != 2 {
		t.Errorf("after j: cursor = %d; want 2", m.fileViewer.cursor)
	}
	if m.fileViewer.charPos != 4 {
		t.Errorf("after j (beta): charPos = %d; want 4", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 4 {
		t.Errorf("after j: preferred = %d; want 4 (unchanged)", m.fileViewer.preferred)
	}
}

// TestFileView_JClampsOnShortLine: when the destination line is
// shorter than preferred, j clamps charPos to the new line's
// byte length but holds preferred. Spec D3 / scenario "j clamps
// the column when the new line is shorter".
func TestFileView_JClampsOnShortLine(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbe\ngamma\n")
	m := openFileInFixture(t, root, rel)
	for range 5 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.preferred != 5 {
		t.Fatalf("setup: preferred = %d; want 5", m.fileViewer.preferred)
	}
	// j to "be" (2 bytes); charPos clamps to 2, preferred stays 5.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 2 {
		t.Errorf("after j (be): charPos = %d; want 2 (clamped)", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 5 {
		t.Errorf("after j: preferred = %d; want 5 (unchanged)", m.fileViewer.preferred)
	}
	// j to "gamma" (5 bytes); charPos jumps back to 5.
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 5 {
		t.Errorf("after j (gamma): charPos = %d; want 5 (preferred restored)", m.fileViewer.charPos)
	}
}

// TestFileView_JClampsToRuneBoundary: j lands on a line with a
// multi-byte rune at the preferred column; charPos snaps back
// to the rune's start. Spec D3 / scenario "Rune boundary
// preservation on j".
func TestFileView_JClampsToRuneBoundary(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\néé\n")
	m := openFileInFixture(t, root, rel)
	for range 1 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	// preferred=1, "éé" line is 4 bytes (two 2-byte runes).
	// j sets charPos = min(1, 4) = 1 — which is mid-rune for
	// the first é (bytes 0..1). The clamp snaps charPos back
	// to 0.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 0 {
		t.Errorf("after j onto mid-rune column: charPos = %d; want 0 (rune-snapped)", m.fileViewer.charPos)
	}
}

// TestFileView_LRaisesPreferred: pressing `l` from byte 3 to
// byte 4 raises preferred to 4. Spec scenario "`l` advances the
// column and raises preferred".
func TestFileView_LRaisesPreferred(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	m := openFileInFixture(t, root, rel)
	for range 4 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.charPos != 4 {
		t.Errorf("after 4× l: charPos = %d; want 4", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 4 {
		t.Errorf("after 4× l: preferred = %d; want 4", m.fileViewer.preferred)
	}
}

// TestFileView_HLeavesPreferredIntact: pressing `h` retreats
// charPos but leaves preferred unchanged. Spec scenario "`h`
// retreats the column but holds preferred".
func TestFileView_HLeavesPreferredIntact(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	m := openFileInFixture(t, root, rel)
	for range 4 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	// preferred is now 4. h retreats charPos to 3; preferred stays 4.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 3 {
		t.Errorf("after h: charPos = %d; want 3", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 4 {
		t.Errorf("after h: preferred = %d; want 4 (unchanged)", m.fileViewer.preferred)
	}
}

// TestFileView_LSnapsToRuneBoundary: pressing `l` from byte 0
// of "é" (2 bytes) jumps charPos to 2 (the next rune's start),
// never landing on byte 1. Spec scenario "Rune boundary
// preservation on l".
func TestFileView_LSnapsToRuneBoundary(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "é\n")
	m := openFileInFixture(t, root, rel)
	// Cursor starts at (1, 0). l should advance to byte 2 (past
	// the é) — but len(line) is 2, so it's a no-op. Let me make
	// the line longer so l can advance.
	root2, rel2 := writeUTF8Fixture(t, "main.go", "éa\n")
	m = openFileInFixture(t, root2, rel2)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = updatedModelPtr(upd)
	if m.fileViewer.charPos != 2 {
		t.Errorf("after l past é: charPos = %d; want 2 (rune-aligned)", m.fileViewer.charPos)
	}
}

// TestFileView_VSeedsAnchorAtCursor: pressing `v` (no prior
// movement) seeds the visual anchor at the cursor's exact
// position. Spec scenario "`v` enters char visual at the cursor".
func TestFileView_VSeedsAnchorAtCursor(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	m := openFileInFixture(t, root, rel)
	for range 3 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	if !m.fileViewer.visual.Active {
		t.Fatal("expected visual active after v")
	}
	if m.fileViewer.visual.LineA != 1 || m.fileViewer.visual.CharA != 3 {
		t.Errorf("after v at (1, 3): visual anchor = (%d, %d); want (1, 3)",
			m.fileViewer.visual.LineA, m.fileViewer.visual.CharA)
	}
}

// TestFileView_EscFromVisualStaysInViewer: pressing Esc while
// visual is active clears the visual but keeps stateFileView.
// A second Esc returns to dir nav. Spec scenario "`Esc` from
// char visual stays in the file viewer".
func TestFileView_EscFromVisualStaysInViewer(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	m := openFileInFixture(t, root, rel)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updatedModelPtr(upd)
	if m.fileViewer.visual.Active {
		t.Errorf("after Esc from visual: visual still active")
	}
	if m.state != stateFileView {
		t.Errorf("after Esc from visual: state = %v; want stateFileView", m.state)
	}
}

// TestFileView_CommentSingleLinePointVisualFallsBackToWholeLine:
// visual active with anchor == cursor on a single line; `c`
// opens a whole-line anchor. Spec scenario "`c` with a
// single-line point visual falls back to whole line".
func TestFileView_CommentSingleLinePointVisualFallsBackToWholeLine(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\n")
	m := openFileInFixture(t, root, rel)
	for range 3 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	// v → visual at (1, 3); cursor also at (1, 3).
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	// c opens the composer with the whole-line anchor.
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updatedModelPtr(upd)
	if m.state != stateCommentComposer {
		t.Fatalf("after c: state = %v; want stateCommentComposer", m.state)
	}
	a := m.commentAnchor
	if a.lineStart != 1 || a.lineEnd != 1 {
		t.Errorf("expected line range 1-1; got %d-%d", a.lineStart, a.lineEnd)
	}
	if a.byteA != -1 || a.byteC != -1 {
		t.Errorf("expected byte range 0/0 (whole-line); got %d/%d", a.byteA, a.byteC)
	}
}

// TestFileView_CommentMultiLinePointVisualYieldsLineRange:
// visual active with anchor at (5, 0) and cursor at (7, 0); the
// byte range covers all of line 5 and 6 + empty start of line
// 7. `c` yields the line range 5..7. Spec scenario "`c` with a
// multi-line point visual yields the line range".
func TestFileView_CommentMultiLinePointVisualYieldsLineRange(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\neta\n")
	m := openFileInFixture(t, root, rel)
	// j j j j moves cursor from line 1 to line 5.
	for range 4 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.cursor != 5 {
		t.Fatalf("setup: cursor should be on line 5; got %d", m.fileViewer.cursor)
	}
	// v seeds anchor at (5, 0); cursor also at (5, 0).
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	// j j moves the cursor to (7, 0). anchor stays at (5, 0).
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != 7 {
		t.Fatalf("after v + j j: cursor should be on line 7; got %d", m.fileViewer.cursor)
	}
	// c opens the composer with the line range 5..7.
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updatedModelPtr(upd)
	if m.state != stateCommentComposer {
		t.Fatalf("after c: state = %v; want stateCommentComposer", m.state)
	}
	a := m.commentAnchor
	if a.lineStart != 5 || a.lineEnd != 7 {
		t.Errorf("expected line range 5-7; got %d-%d", a.lineStart, a.lineEnd)
	}
	if a.byteA != -1 || a.byteC != -1 {
		t.Errorf("expected byte range 0/0 (multi-line collapsed); got %d/%d", a.byteA, a.byteC)
	}
}

// TestFileView_CursorAtEndOfLineTrailingBlock: cursor at
// charPos == len(line) renders a trailing inverted space
// (the cursor's "block" sits at the line end). Spec scenario
// "Cursor at end of line renders as a trailing block".
func TestFileView_CursorAtEndOfLineTrailingBlock(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\n")
	m := openFileInFixture(t, root, rel)
	// Move cursor to end of line 1: l l l l l
	for range 5 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	view := m.fileViewer.viewport.View()
	// Look for the trailing-block pattern at end of line 1.
	// The line is "alpha" (5 bytes). Cursor at byte 5 = len(line).
	// Render produces: "alpha\x1b[7m \x1b[27m"
	if !strings.HasSuffix(view, "alpha\x1b[7m \x1b[27m") &&
		!strings.Contains(view, "alpha"+"\x1b[7m \x1b[27m") {
		t.Errorf("expected trailing inverted space at end of line 1; got:\n%q",
			view[:min(120, len(view))])
	}
}

// TestFileView_WAdvancesToNextWord: pressing `w` from the start of
// a word moves the cursor to the start of the next word.
func TestFileView_WAdvancesToNextWord(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "foo bar\n")
	m := openFileInFixture(t, root, rel)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != 1 {
		t.Errorf("after w: cursor = %d; want 1 (same line)", m.fileViewer.cursor)
	}
	if m.fileViewer.charPos != 4 {
		t.Errorf("after w: charPos = %d; want 4 (start of 'bar')", m.fileViewer.charPos)
	}
}

// TestFileView_BCrossesLine: pressing `b` from the first word of
// line 2 jumps to the last word on line 1.
func TestFileView_BCrossesLine(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "foo bar\nbaz qux\n")
	m := openFileInFixture(t, root, rel)
	// j to line 2 (cursor=2, charPos follows preferred)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)
	// b: from 'b' of "baz" (cursor=2, charPos=0) → start of "bar" (cursor=1, charPos=4)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != 1 || m.fileViewer.charPos != 4 {
		t.Errorf("after b from (2,0): cursor=(%d,%d); want (1,4)",
			m.fileViewer.cursor, m.fileViewer.charPos)
	}
}

// TestFileView_WSkipsBlankLine: blank lines in the file are word
// separators.
func TestFileView_WSkipsBlankLine(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "hello\n\nworld\n")
	m := openFileInFixture(t, root, rel)
	// Move to end of "hello" with l l l l l
	for range 5 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	// w: from end of "hello" → start of "world" (line 3, charPos 0)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != 3 || m.fileViewer.charPos != 0 {
		t.Errorf("after w blank-skip: cursor=(%d,%d); want (3,0)",
			m.fileViewer.cursor, m.fileViewer.charPos)
	}
}

// TestFileView_WUpdatesPreferred: pressing `w` raises preferred
// to the new charPos, like `l`.
func TestFileView_WUpdatesPreferred(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "foo bar\n")
	m := openFileInFixture(t, root, rel)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = updatedModelPtr(upd)
	if m.fileViewer.preferred != 4 {
		t.Errorf("after w: preferred = %d; want 4", m.fileViewer.preferred)
	}
}

// TestFileView_BLeavesPreferred: pressing `b` leaves preferred
// unchanged, like `h`.
func TestFileView_BLeavesPreferred(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "foo bar\n")
	m := openFileInFixture(t, root, rel)
	// set preferred to 4 via w
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = updatedModelPtr(upd)
	if m.fileViewer.preferred != 4 {
		t.Fatalf("setup: preferred = %d; want 4", m.fileViewer.preferred)
	}
	// b: cursor on 'b' of "bar" (1,4); preferred stays 4
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updatedModelPtr(upd)
	if m.fileViewer.preferred != 4 {
		t.Errorf("after b: preferred = %d; want 4 (unchanged)", m.fileViewer.preferred)
	}
	if m.fileViewer.charPos != 0 {
		t.Errorf("after b: charPos = %d; want 0 (start of 'foo')", m.fileViewer.charPos)
	}
}

// TestFileView_WNoOpAtEndOfFile: pressing `w` at the last word of
// the last line is a no-op.
func TestFileView_WNoOpAtEndOfFile(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\n")
	m := openFileInFixture(t, root, rel)
	// l l l l l to end of "alpha"
	for range 5 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	beforeCursor, beforePos := m.fileViewer.cursor, m.fileViewer.charPos
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != beforeCursor || m.fileViewer.charPos != beforePos {
		t.Errorf("w at end should no-op; got (%d,%d) want (%d,%d)",
			m.fileViewer.cursor, m.fileViewer.charPos, beforeCursor, beforePos)
	}
}

// TestFileView_BNoOpAtStartOfFile: pressing `b` at the first rune
// of the first line is a no-op.
func TestFileView_BNoOpAtStartOfFile(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	m := openFileInFixture(t, root, rel)
	beforeCursor, beforePos := m.fileViewer.cursor, m.fileViewer.charPos
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != beforeCursor || m.fileViewer.charPos != beforePos {
		t.Errorf("b at start should no-op; got (%d,%d) want (%d,%d)",
			m.fileViewer.cursor, m.fileViewer.charPos, beforeCursor, beforePos)
	}
}

// TestFileView_VWExtendsSelection: pressing `v` then `w` moves the
// cursor while the anchor stays put.
func TestFileView_VWExtendsSelection(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "foo bar\n")
	m := openFileInFixture(t, root, rel)
	// Enter visual
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updatedModelPtr(upd)
	if !m.fileViewer.visual.Active {
		t.Fatalf("setup: visual not active")
	}
	if m.fileViewer.visual.LineA != 1 || m.fileViewer.visual.CharA != 0 {
		t.Fatalf("setup: anchor = (%d,%d); want (1,0)",
			m.fileViewer.visual.LineA, m.fileViewer.visual.CharA)
	}
	// w moves cursor to (1, 4); anchor stays (1, 0)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m = updatedModelPtr(upd)
	if m.fileViewer.visual.LineA != 1 || m.fileViewer.visual.CharA != 0 {
		t.Errorf("after w in visual: anchor = (%d,%d); want (1,0) (unchanged)",
			m.fileViewer.visual.LineA, m.fileViewer.visual.CharA)
	}
	if m.fileViewer.cursor != 1 || m.fileViewer.charPos != 4 {
		t.Errorf("after w in visual: cursor = (%d,%d); want (1,4)",
			m.fileViewer.cursor, m.fileViewer.charPos)
	}
}
