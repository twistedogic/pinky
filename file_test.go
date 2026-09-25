package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
	"github.com/twistedogic/pinky/internal/workspace"
)

// fileFixtureTree returns a temp workspace root containing a small
// tree and a relative path under it. Returns (root, fileRelPath,
// fileLineCount).
func fileFixtureTree(t *testing.T) (string, string, int) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/main.go", "alpha\nbeta\ngamma\n")
	write("README.md", "hi\n")
	// line count = 3 for main.go
	return root, "src/main.go", 3
}

// attachFileFixture wires a model to the fixture tree (no real
// tmux, no polling).
func attachFileFixture(t *testing.T) *model {
	t.Helper()
	root, _, _ := fileFixtureTree(t)
	m := newIdleModelForKeymap(t)
	m.fileRoot = root
	entries, err := workspace.Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	m.fileEntries = entries
	m.fileCollapsed = map[string]bool{}
	m.state = stateFileNav
	m.tab = tabFiles
	return &m
}

// TestTab_TogglesBetweenMessageAndFiles: Tab from stateNav enters
// stateFileNav; Tab back returns to stateNav.
func TestTab_TogglesBetweenMessageAndFiles(t *testing.T) {
	root, _, _ := fileFixtureTree(t)
	m := newIdleModelForKeymap(t)
	m.fileRoot = root
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "msg"}
	m.refreshViewport()

	tab := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(tab)
	um := updated.(model)
	if um.tab != tabFiles {
		t.Errorf("expected tabFiles; got %v", um.tab)
	}
	if um.state != stateFileNav {
		t.Errorf("expected stateFileNav; got %v", um.state)
	}

	updated, _ = um.Update(tab)
	um2 := updated.(model)
	if um2.tab != tabMessage {
		t.Errorf("expected tabMessage; got %v", um2.tab)
	}
	if um2.state != stateNav {
		t.Errorf("expected stateNav; got %v", um2.state)
	}
}

// TestFileNav_OpenFileOpensViewer: pressing Enter on a file
// transitions to stateFileView with the path set.
func TestFileNav_OpenFileOpensViewer(t *testing.T) {
	m := attachFileFixture(t)
	// Find the index of main.go in the visible entries.
	visible := m.visibleFileEntries()
	idx := -1
	for i, e := range visible {
		if e.Path == "src/main.go" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("src/main.go not in visible tree")
	}
	m.fileCursor = idx

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(enter)
	um := updated.(model)
	if um.state != stateFileView {
		t.Errorf("expected stateFileView; got %v", um.state)
	}
	if um.fileViewer.path != "src/main.go" {
		t.Errorf("expected viewer path 'src/main.go'; got %q", um.fileViewer.path)
	}
}

// TestFileView_CommentWholeFile: in the file viewer, pressing `c`
// then typing + Enter saves a file-kind comment covering the
// whole file (line range 1..N).
func TestFileView_CommentWholeFile(t *testing.T) {
	m := attachFileFixture(t)
	// jump to viewer for src/main.go
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src/main.go" {
			m.fileCursor = i
			break
		}
	}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(enter)
	um := updated.(model)
	if um.state != stateFileView {
		t.Fatalf("setup: expected stateFileView; got %v", um.state)
	}

	// Press `c` to open composer.
	c := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	updated, _ = um.Update(c)
	um = updated.(model)
	if um.state != stateCommentComposer {
		t.Fatalf("after `c`: expected stateCommentComposer; got %v", um.state)
	}

	// Type + Enter.
	um.commentTa.SetValue("rename alpha")
	save := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ = um.Update(save)
	um = updated.(model)
	if len(um.comments) != 1 {
		t.Fatalf("expected 1 comment; got %d", len(um.comments))
	}
	c0 := um.comments[0]
	if c0.Kind != render.CommentFile {
		t.Errorf("expected Kind=CommentFile; got %v", c0.Kind)
	}
	if c0.Path != "src/main.go" {
		t.Errorf("expected Path=src/main.go; got %q", c0.Path)
	}
	if c0.LineStart != 1 || c0.LineEnd != 3 {
		t.Errorf("expected line range 1-3; got %d-%d", c0.LineStart, c0.LineEnd)
	}
}

// TestFileView_CommentSelection: in the file viewer with visual
// mode active and a multi-line selection, `c` saves a file-kind
// comment with the line range.
func TestFileView_CommentSelection(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src/main.go" {
			m.fileCursor = i
			break
		}
	}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(enter)
	um := updated.(model)

	// v → j (visual mode, extend line range).
	upd, _ := um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	um = upd.(model)
	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	um = upd.(model)
	if !um.fileViewer.visual.Active {
		t.Fatalf("visual mode should be active")
	}
	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	um = upd.(model)
	if um.state != stateCommentComposer {
		t.Fatalf("expected composer; got %v", um.state)
	}
	um.commentTa.SetValue("wrap ctx")
	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = upd.(model)
	if len(um.comments) != 1 {
		t.Fatalf("expected 1 comment; got %d", len(um.comments))
	}
	c0 := um.comments[0]
	if c0.Kind != render.CommentFile || c0.Path != "src/main.go" {
		t.Errorf("expected file-kind on src/main.go; got kind=%v path=%q", c0.Kind, c0.Path)
	}
	if c0.LineStart != 1 || c0.LineEnd != 2 {
		t.Errorf("expected line range 1-2 (visual extended by j); got %d-%d", c0.LineStart, c0.LineEnd)
	}
}

// TestFileView_CommentInlineCharRange: in the file viewer with
// visual mode and a single-line inline selection (`v` then `l`
// several times), `c` saves a file-kind comment with CharStart
// /CharEnd set to the byte offsets.
func TestFileView_CommentInlineCharRange(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src/main.go" {
			m.fileCursor = i
			break
		}
	}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(enter)
	um := updated.(model)
	if um.fileViewer.content != "alpha\nbeta\ngamma\n" {
		t.Fatalf("viewer content wrong: %q", um.fileViewer.content)
	}

	upd, _ := um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	um = upd.(model)
	// Three `l` presses: advance the cursor 3 runes (alpha = 5 bytes,
	// but `l` is rune-granular, so we expect 3 rune advances from char 0
	// → through 'a','l','p' → ends at byte offset 3).
	for i := 0; i < 3; i++ {
		upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		um = upd.(model)
	}

	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	um = upd.(model)
	um.commentTa.SetValue("trim prefix")
	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = upd.(model)
	if len(um.comments) != 1 {
		t.Fatalf("expected 1 comment; got %d", len(um.comments))
	}
	c0 := um.comments[0]
	if c0.Kind != render.CommentFile {
		t.Errorf("expected file-kind; got %v", c0.Kind)
	}
	if c0.LineStart != 1 || c0.LineEnd != 1 {
		t.Errorf("expected single-line selection; got %d-%d", c0.LineStart, c0.LineEnd)
	}
	if c0.CharStart != 0 || c0.CharEnd != 3 {
		t.Errorf("expected char range 0-3; got %d-%d", c0.CharStart, c0.CharEnd)
	}
	if c0.Source != "alp" {
		t.Errorf("expected source excerpt 'alp'; got %q", c0.Source)
	}
}

// TestUnifiedFlush_SendsAllKinds: one block-kind and one
// file-kind comment, `s` flushes both into one inject payload.
func TestUnifiedFlush_SendsAllKinds(t *testing.T) {
	prev := sendToPane
	var sent string
	sendToPane = func(_, text string) error { sent = text; return nil }
	t.Cleanup(func() { sendToPane = prev })

	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "alpha block"}
	m.refreshViewport()
	initCommentTAForTest(&m)
	m.comments = append(m.comments,
		render.Comment{Kind: render.CommentBlock, BlockIdx: 0, CharStart: -1, Text: "block note", CreatedAt: time.Now()},
		render.Comment{Kind: render.CommentFile, Path: "src/main.go", LineStart: 1, LineEnd: 3,
			CharStart: -1, CharEnd: -1, Text: "file note", CreatedAt: time.Now().Add(time.Second)},
	)
	s := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	updated, _ := m.Update(s)
	um := updated.(model)
	if sent == "" {
		t.Fatal("expected sendToPane to be called")
	}
	for _, want := range []string{"2 comments:", "block", "file", "src/main.go", "block note", "file note"} {
		if !strings.Contains(sent, want) {
			t.Errorf("missing %q in payload:\n%s", want, sent)
		}
	}
	if len(um.comments) != 0 {
		t.Errorf("expected comments cleared; got %d", len(um.comments))
	}
}

// TestFileNav_COnDirectoryIsNoop: `c` on a directory in stateFileNav
// does nothing (directories can't be commented).
func TestFileNav_COnDirectoryIsNoop(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src" && e.IsDir {
			m.fileCursor = i
			break
		}
	}
	c := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	updated, _ := m.Update(c)
	um := updated.(model)
	if um.state != stateFileNav {
		t.Errorf("expected stateFileNav; got %v", um.state)
	}
	if len(um.comments) != 0 {
		t.Errorf("expected no comments; got %d", len(um.comments))
	}
}

// TestFileNav_HLCollapseExpand: h collapses an expanded dir; l
// expands a collapsed one.
func TestFileNav_HLCollapseExpand(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src" && e.IsDir {
			m.fileCursor = i
			break
		}
	}
	if m.fileCollapsed["src"] {
		t.Fatalf("setup: src should not start collapsed")
	}
	h := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	updated, _ := m.Update(h)
	um := updated.(model)
	if !um.fileCollapsed["src"] {
		t.Errorf("expected src collapsed after `h`")
	}
	l := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	updated, _ = um.Update(l)
	um = updated.(model)
	if um.fileCollapsed["src"] {
		t.Errorf("expected src expanded after `l`")
	}
}

// TestFileNav_EscReturnsToMessage: pressing Esc from stateFileNav
// returns to the message tab.
func TestFileNav_EscReturnsToMessage(t *testing.T) {
	m := attachFileFixture(t)
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(esc)
	um := updated.(model)
	if um.tab != tabMessage {
		t.Errorf("expected tabMessage; got %v", um.tab)
	}
	if um.state != stateNav {
		t.Errorf("expected stateNav; got %v", um.state)
	}
}

// TestFileView_EscReturnsToDirNav: pressing Esc from stateFileView
// returns to stateFileNav.
func TestFileView_EscReturnsToDirNav(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src/main.go" {
			m.fileCursor = i
			break
		}
	}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(enter)
	um := updated.(model)
	if um.state != stateFileView {
		t.Fatalf("setup: expected stateFileView; got %v", um.state)
	}
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ = um.Update(esc)
	um = updated.(model)
	if um.state != stateFileNav {
		t.Errorf("expected stateFileNav; got %v", um.state)
	}
}

// longFileFixture writes a fixture file with N lines into the
// temp workspace and returns its relative path. Used by the
// viewport scroll tests below.
func longFileFixture(t *testing.T, n int) (root, rel string) {
	t.Helper()
	root = t.TempDir()
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	rel = "big.txt"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, rel
}

// openFileInFixture wires the fixture into the model and walks
// the workspace, so the file is reachable via the file tree.
// Returns the model already transitioned to stateFileView with the
// file loaded.
func openFileInFixture(t *testing.T, root, rel string) *model {
	t.Helper()
	m := newIdleModelForKeymap(t)
	m.fileRoot = root
	entries, err := workspace.Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	m.fileEntries = entries
	m.fileCollapsed = map[string]bool{}
	m.state = stateFileNav
	m.tab = tabFiles
	m.openFileViewer(rel)
	return &m
}

// TestFileView_LongFileShowsPositionIndicator: opening a file
// taller than the viewport renders a `lines N-M of K` indicator
// in the header.
func TestFileView_LongFileShowsPositionIndicator(t *testing.T) {
	root, rel := longFileFixture(t, 100)
	m := openFileInFixture(t, root, rel)
	view := m.fileViewView()
	if !strings.Contains(view, "lines 1-") {
		t.Errorf("expected position indicator in long-file header; got:\n%s", view)
	}
	if !strings.Contains(view, "of 100") {
		t.Errorf("expected `of 100` in indicator; got:\n%s", view)
	}
}

// TestFileView_ShortFileOmitsIndicator: a file that fits in the
// viewport renders no `lines N-M of K` indicator.
func TestFileView_ShortFileOmitsIndicator(t *testing.T) {
	root, rel := longFileFixture(t, 3)
	m := openFileInFixture(t, root, rel)
	view := m.fileViewView()
	if strings.Contains(view, "lines ") {
		t.Errorf("short file should not show indicator; got:\n%s", view)
	}
}

// TestFileView_JScrollsViewport: pressing `j` repeatedly moves
// the cursor and scrolls the file viewport so the cursor stays
// visible.
func TestFileView_JScrollsViewport(t *testing.T) {
	root, rel := longFileFixture(t, 100)
	m := openFileInFixture(t, root, rel)
	start := m.fileViewer.viewport.YOffset
	// Move cursor down well past the viewport height.
	for i := 0; i < 30; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		mv := updated.(model)
		m = &mv
	}
	if m.fileViewer.viewport.YOffset <= start {
		t.Errorf("expected viewport YOffset to advance after 30 j presses; start=%d now=%d",
			start, m.fileViewer.viewport.YOffset)
	}
	if m.fileViewer.cursor != 31 {
		t.Errorf("expected cursor at line 31; got %d", m.fileViewer.cursor)
	}
}

// TestFileView_PageDownScrolls: pressing PageDown scrolls the
// file viewport without moving the cursor.
func TestFileView_PageDownScrolls(t *testing.T) {
	root, rel := longFileFixture(t, 100)
	m := openFileInFixture(t, root, rel)
	start := m.fileViewer.viewport.YOffset
	startCursor := m.fileViewer.cursor
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	mv := updated.(model)
	m = &mv
	if m.fileViewer.viewport.YOffset <= start {
		t.Errorf("expected PageDown to advance YOffset; start=%d now=%d",
			start, m.fileViewer.viewport.YOffset)
	}
	if m.fileViewer.cursor != startCursor {
		t.Errorf("PageDown should not move cursor; was %d now %d",
			startCursor, m.fileViewer.cursor)
	}
}

// TestFileView_EndLandsAtBottom: pressing End scrolls the file
// viewport to the last line.
func TestFileView_EndLandsAtBottom(t *testing.T) {
	root, rel := longFileFixture(t, 100)
	m := openFileInFixture(t, root, rel)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	mv := updated.(model)
	m = &mv
	if !m.fileViewer.viewport.AtBottom() {
		t.Errorf("End should scroll viewport to bottom; YOffset=%d", m.fileViewer.viewport.YOffset)
	}
}

// TestFileView_VTogglesHighlight: pressing `v` then `l` must
// make the viewport content include the cyan selection
// highlight. Before the viewport refactor the renderer re-ran on
// every View(); now the body lives in m.fileViewer.viewport's
// cache, so both the `v` toggle and the `l` move must call
// refreshFileView() to push the new selection into the cache.
// `v` alone leaves the selection degenerate (CharA==CharC==0),
// so we extend it with `l` to verify a real highlight range.
func TestFileView_VTogglesHighlight(t *testing.T) {
	root, rel := longFileFixture(t, 3)
	m := openFileInFixture(t, root, rel)
	before := m.fileViewer.viewport.View()
	if strings.Contains(before, "\x1b[38;5;51m") {
		t.Fatalf("setup: highlight already present before `v`")
	}
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	mv := upd.(model)
	if !mv.fileViewer.visual.Active {
		t.Fatalf("visual mode should be active after `v`")
	}
	upd, _ = mv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	mv = upd.(model)
	if mv.fileViewer.visual.CharC == 0 {
		t.Fatalf("setup: `l` should have advanced CharC from 0")
	}
	after := mv.fileViewer.viewport.View()
	if !strings.Contains(after, "\x1b[38;5;51m") {
		t.Errorf("expected cyan selection highlight after `v` then `l`; got:\n%s", after)
	}
}

// TestFileView_EscClearsVisualHighlight: pressing `v` then `l`
// to make a non-degenerate selection, then Esc, must remove the
// cyan selection highlight from the viewport's cached content.
func TestFileView_EscClearsVisualHighlight(t *testing.T) {
	root, rel := longFileFixture(t, 3)
	m := openFileInFixture(t, root, rel)
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	mv := upd.(model)
	upd, _ = mv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	mv = upd.(model)
	if !strings.Contains(mv.fileViewer.viewport.View(), "\x1b[38;5;51m") {
		t.Fatalf("setup: highlight should be present after `v` then `l`")
	}
	upd, _ = mv.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mv = upd.(model)
	if mv.fileViewer.visual.Active {
		t.Fatalf("visual mode should be inactive after Esc")
	}
	if strings.Contains(mv.fileViewer.viewport.View(), "\x1b[38;5;51m") {
		t.Errorf("expected cyan highlight cleared after Esc; got:\n%s",
			mv.fileViewer.viewport.View())
	}
}

// TestFileView_LMovesVisualHighlight: after entering visual mode,
// pressing `l` extends the inline selection by one rune. The
// viewport's cached content must reflect the new CharC — the
// cyan range must grow.
func TestFileView_LMovesVisualHighlight(t *testing.T) {
	root, rel := longFileFixture(t, 3)
	m := openFileInFixture(t, root, rel)
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	mv := upd.(model)
	upd, _ = mv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	mv = upd.(model)
	if mv.fileViewer.visual.CharC == 0 {
		t.Fatalf("setup: `l` should have advanced CharC from 0")
	}
	view := mv.fileViewer.viewport.View()
	if !strings.Contains(view, "\x1b[38;5;51m") {
		t.Errorf("expected cyan highlight after `l`; got:\n%s", view)
	}
}

// TestFileView_NewCommentShowsYellowGutter: saving a file-kind
// comment must refresh the viewport cache so the new yellow ▍
// gutter appears on the commented line. Before the fix, the
// gutter only appeared after the next cursor move triggered
// refreshFileView().
func TestFileView_NewCommentShowsYellowGutter(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src/main.go" {
			m.fileCursor = i
			break
		}
	}
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := upd.(model)
	if um.state != stateFileView {
		t.Fatalf("setup: expected stateFileView; got %v", um.state)
	}
	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	um = upd.(model)
	if um.state != stateCommentComposer {
		t.Fatalf("setup: expected composer; got %v", um.state)
	}
	um.commentTa.SetValue("rename alpha")
	upd, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = upd.(model)
	if um.state != stateFileView {
		t.Fatalf("after save: expected stateFileView; got %v", um.state)
	}
	view := um.fileViewer.viewport.View()
	if !strings.Contains(view, "\x1b[38;5;228m") {
		t.Errorf("expected yellow ▍ gutter in viewport after saving comment; got:\n%s", view)
	}
}
