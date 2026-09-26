package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
	write("src/pkg/a.go", "x\n")
	write("README.md", "hi\n")
	// line count = 3 for main.go
	return root, "src/main.go", 3
}

// attachFileFixture wires a model to the fixture tree (no real
// tmux, no polling) and seeds the dir navigator's flat tree via
// workspace.Walk. Everything is synchronous; no Cmd pump needed.
func attachFileFixture(t *testing.T) *model {
	t.Helper()
	root, _, _ := fileFixtureTree(t)
	m := newIdleModelForKeymap(t)
	m.fileRoot = root
	m.enterFileNav()
	m.tab = tabFiles
	if m.state != stateFileNav {
		t.Fatalf("enterFileNav should land in stateFileNav; got %v", m.state)
	}
	return &m
}

// openFile expands every collapsed dir along rel's path, then
// moves the cursor to rel and presses Enter to transition into
// stateFileView. Fails the test if rel isn't in the tree at all.
func openFile(t *testing.T, m *model, rel string) {
	t.Helper()
	// Expand each intermediate dir so the file becomes visible.
	dir := filepath.Dir(rel)
	for dir != "" && dir != "." {
		delete(m.fileCollapsed, dir)
		dir = filepath.Dir(dir)
	}
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == rel {
			m.fileCursor = i
			upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			*m = upd.(model)
			if m.state != stateFileView {
				t.Fatalf("expected stateFileView after opening %q; got %v", rel, m.state)
			}
			return
		}
	}
	t.Fatalf("%q not in visible tree (have %v)", rel, visiblePathList(visible))
}

func visiblePathList(es []workspace.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Path
	}
	return out
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

	tab := tea.KeyPressMsg{Code: tea.KeyTab}
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

// TestFileNav_TreeContainsAllEntries: the flat tree produced by
// workspace.Walk contains every entry under fileRoot (collapsed
// or not). Verifies the walker, not the collapse state.
func TestFileNav_TreeContainsAllEntries(t *testing.T) {
	m := attachFileFixture(t)
	got := visiblePathList(m.fileEntries)
	for _, want := range []string{"README.md", "src", "src/main.go", "src/pkg", "src/pkg/a.go"} {
		var found bool
		for _, p := range got {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in flat tree; got %v", want, got)
		}
	}
	// And the visible list starts collapsed: only top-level
	// entries are visible until the user presses `l`.
	visible := visiblePathList(m.visibleFileEntries())
	for _, p := range []string{"src/main.go", "src/pkg", "src/pkg/a.go"} {
		for _, vp := range visible {
			if vp == p {
				t.Errorf("%q should be hidden initially; visible=%v", p, visible)
			}
		}
	}
}

// TestFileNav_HLCollapseExpand: dirs start collapsed. Pressing
// `l` on `src` expands it (src/main.go becomes visible); pressing
// `l` again jumps to the first visible child. Pressing `h` on a
// child entry jumps to the parent dir.
func TestFileNav_HLCollapseExpand(t *testing.T) {
	m := attachFileFixture(t)

	// Setup: dirs start collapsed, so `src` is visible but its
	// children are not.
	initial := visiblePathList(m.visibleFileEntries())
	for _, p := range initial {
		if p == "src/main.go" || p == "src/pkg" || p == "src/pkg/a.go" {
			t.Fatalf("setup: %q should be hidden initially (collapsed by default); got %v", p, initial)
		}
	}

	// Move cursor to `src`.
	srcIdx := -1
	for i, e := range m.visibleFileEntries() {
		if e.Path == "src" {
			srcIdx = i
			break
		}
	}
	if srcIdx < 0 {
		t.Fatalf("setup: `src` not in visible tree")
	}
	m.fileCursor = srcIdx

	// `l` on collapsed `src`: expands it.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	um := upd.(model)
	if um.fileCollapsed["src"] {
		t.Errorf("`l` on collapsed `src` should expand it; fileCollapsed=%v", um.fileCollapsed)
	}
	gotPaths := visiblePathList(um.visibleFileEntries())
	var hasMain bool
	for _, p := range gotPaths {
		if p == "src/main.go" {
			hasMain = true
			break
		}
	}
	if !hasMain {
		t.Errorf("after expand: src/main.go should be visible; got %v", gotPaths)
	}

	// `l` again on expanded `src`: jumps to first visible child
	// (src/main.go). Cursor moves, src stays expanded.
	upd, _ = um.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	um = upd.(model)
	if um.fileCollapsed["src"] {
		t.Errorf("`l` should jump to child, not collapse; fileCollapsed=%v", um.fileCollapsed)
	}
	cursorEntry := um.visibleFileEntries()[um.fileCursor]
	if cursorEntry.Path != "src/main.go" {
		t.Errorf("`l` on expanded `src` should jump cursor to first child src/main.go; got %q", cursorEntry.Path)
	}

	// `h` on src/main.go: jumps to parent dir `src` (file, so
	// no collapse applies).
	upd, _ = um.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	um = upd.(model)
	parent := um.visibleFileEntries()[um.fileCursor]
	if parent.Path != "src" {
		t.Errorf("`h` on src/main.go should jump cursor to parent `src`; got %q", parent.Path)
	}
}

// TestFileNav_HCollapsesExpandedDir: `h` on an expanded dir
// collapses it without moving the cursor.
func TestFileNav_HCollapsesExpandedDir(t *testing.T) {
	m := attachFileFixture(t)
	// Expand src first.
	for i, e := range m.visibleFileEntries() {
		if e.Path == "src" {
			m.fileCursor = i
			break
		}
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	um := upd.(model)
	if um.fileCollapsed["src"] {
		t.Fatalf("setup: src should be expanded after l")
	}
	cursorBefore := um.fileCursor

	// `h` on expanded dir: collapses, cursor stays.
	upd, _ = um.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	um = upd.(model)
	if !um.fileCollapsed["src"] {
		t.Errorf("`h` on expanded `src` should collapse it; fileCollapsed=%v", um.fileCollapsed)
	}
	if um.fileCursor != cursorBefore {
		t.Errorf("`h` should not move cursor; was %d, now %d", cursorBefore, um.fileCursor)
	}
}

// TestFileNav_OpenFileOpensViewer: pressing Enter on src/main.go
// in the tree transitions to stateFileView with the path set.
func TestFileNav_OpenFileOpensViewer(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	if m.fileViewer.path != "src/main.go" {
		t.Errorf("expected viewer path 'src/main.go'; got %q", m.fileViewer.path)
	}
}

// TestFileView_CommentWholeFile: in the file viewer, pressing `c`
// then typing + Enter saves a file-kind comment covering the
// whole file (line range 1..N).
func TestFileView_CommentWholeFile(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")

	c := tea.KeyPressMsg{Code: 'c', Text: "c"}
	updated, _ := m.Update(c)
	um := updated.(model)
	if um.state != stateCommentComposer {
		t.Fatalf("after `c`: expected stateCommentComposer; got %v", um.state)
	}

	um.commentTa.SetValue("rename alpha")
	save := tea.KeyPressMsg{Code: tea.KeyEnter}
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
	openFile(t, m, "src/main.go")

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	um := upd.(model)
	upd, _ = um.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	um = upd.(model)
	if !um.fileViewer.visual.Active {
		t.Fatalf("visual mode should be active")
	}
	upd, _ = um.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	um = upd.(model)
	if um.state != stateCommentComposer {
		t.Fatalf("expected composer; got %v", um.state)
	}
	um.commentTa.SetValue("wrap ctx")
	upd, _ = um.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	openFile(t, m, "src/main.go")
	if m.fileViewer.content != "alpha\nbeta\ngamma\n" {
		t.Fatalf("viewer content wrong: %q", m.fileViewer.content)
	}

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	um := upd.(model)
	for range 3 {
		upd, _ = um.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		um = upd.(model)
	}

	upd, _ = um.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	um = upd.(model)
	um.commentTa.SetValue("trim prefix")
	upd, _ = um.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	s := tea.KeyPressMsg{Code: 's', Text: "s"}
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

// TestFileNav_EscReturnsToMessage: pressing Esc from stateFileNav
// returns to the message tab.
func TestFileNav_EscReturnsToMessage(t *testing.T) {
	m := attachFileFixture(t)
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
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
	openFile(t, m, "src/main.go")
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	updated, _ := m.Update(esc)
	um := updated.(model)
	if um.state != stateFileNav {
		t.Errorf("expected stateFileNav; got %v", um.state)
	}
}

// TestFileNav_COnDirectoryIsNoop: pressing `c` on a directory in
// stateFileNav must not open the comment composer.
func TestFileNav_COnDirectoryIsNoop(t *testing.T) {
	m := attachFileFixture(t)
	visible := m.visibleFileEntries()
	srcIdx := -1
	for i, e := range visible {
		if e.Path == "src" {
			srcIdx = i
			break
		}
	}
	if srcIdx < 0 {
		t.Fatalf("setup: `src` not visible")
	}
	m.fileCursor = srcIdx
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	um := upd.(model)
	if um.state != stateFileNav {
		t.Errorf("`c` on dir should be no-op; got state=%v", um.state)
	}
	if len(um.comments) != 0 {
		t.Errorf("`c` on dir should not stage a comment; got %d", len(um.comments))
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

// openFileInFixture wires the fixture into the model and loads
// the requested file into stateFileView.
func openFileInFixture(t *testing.T, root, rel string) *model {
	t.Helper()
	m := newIdleModelForKeymap(t)
	m.fileRoot = root
	m.enterFileNav()
	m.tab = tabFiles
	openFile(t, &m, rel)
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
	start := m.fileViewer.viewport.YOffset()
	for range 30 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		mv := updated.(model)
		m = &mv
	}
	if m.fileViewer.viewport.YOffset() <= start {
		t.Errorf("expected viewport YOffset to advance after 30 j presses; start=%d now=%d",
			start, m.fileViewer.viewport.YOffset(),
		)
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
	start := m.fileViewer.viewport.YOffset()
	startCursor := m.fileViewer.cursor
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	mv := updated.(model)
	m = &mv
	if m.fileViewer.viewport.YOffset() <= start {
		t.Errorf("expected PageDown to advance YOffset; start=%d now=%d",
			start, m.fileViewer.viewport.YOffset(),
		)
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
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	mv := updated.(model)
	m = &mv
	if !m.fileViewer.viewport.AtBottom() {
		t.Errorf("End should scroll viewport to bottom; YOffset=%d", m.fileViewer.viewport.YOffset(),
		)
	}
}

// TestFileView_VTogglesHighlight: pressing `v` then `l` must
// make the viewport content include the cyan selection highlight.
func TestFileView_VTogglesHighlight(t *testing.T) {
	root, rel := longFileFixture(t, 3)
	m := openFileInFixture(t, root, rel)
	before := m.fileViewer.viewport.View()
	if strings.Contains(before, "\x1b[38;5;51m") {
		t.Fatalf("setup: highlight already present before `v`")
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	mv := upd.(model)
	if !mv.fileViewer.visual.Active {
		t.Fatalf("visual mode should be active after `v`")
	}
	upd, _ = mv.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
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
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	mv := upd.(model)
	upd, _ = mv.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	mv = upd.(model)
	if !strings.Contains(mv.fileViewer.viewport.View(), "\x1b[38;5;51m") {
		t.Fatalf("setup: highlight should be present after `v` then `l`")
	}
	upd, _ = mv.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
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
// pressing `l` extends the inline selection by one rune.
func TestFileView_LMovesVisualHighlight(t *testing.T) {
	root, rel := longFileFixture(t, 3)
	m := openFileInFixture(t, root, rel)
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	mv := upd.(model)
	upd, _ = mv.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
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
// gutter appears on the commented line.
func TestFileView_NewCommentShowsYellowGutter(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	um := upd.(model)
	if um.state != stateCommentComposer {
		t.Fatalf("setup: expected composer; got %v", um.state)
	}
	um.commentTa.SetValue("rename alpha")
	upd, _ = um.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	um = upd.(model)
	if um.state != stateFileView {
		t.Fatalf("after save: expected stateFileView; got %v", um.state)
	}
	view := um.fileViewer.viewport.View()
	if !strings.Contains(view, "\x1b[38;5;228m") {
		t.Errorf("expected yellow ▍ gutter in viewport after saving comment; got:\n%s", view)
	}
}

// TestFileView_CursorGutterFollowsCursor: every refresh paints a
// green ▍ selector gutter on the cursor's line. Moving the cursor
// moves the gutter.
func TestFileView_CursorGutterFollowsCursor(t *testing.T) {
	const green = "\x1b[38;5;42m"
	m := *attachFileFixture(t)
	openFile(t, &m, "src/main.go")
	if m.fileViewer.cursor != 1 {
		t.Fatalf("setup: cursor should start at 1; got %d", m.fileViewer.cursor)
	}
	view := m.fileViewer.viewport.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], green+"▍") {
		t.Errorf("expected green ▍ selector gutter at start of viewport; got first 80 chars:\n%q",
			view[:min(80, len(view))])
	}

	// Move cursor down; the gutter must move with it.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = upd.(model)
	if m.fileViewer.cursor != 2 {
		t.Fatalf("after j: cursor should be 2; got %d", m.fileViewer.cursor)
	}
	view = m.fileViewer.viewport.View()
	lines = strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("viewport too short to test line 2; got %d lines", len(lines))
	}
	// Line 1 no longer has the gutter, line 2 does.
	if strings.HasPrefix(lines[0], green+"▍") {
		t.Errorf("after j: line 1 should no longer have gutter; got first 60 chars:\n%q",
			lines[0][:min(60, len(lines[0]))])
	}
	if !strings.HasPrefix(lines[1], green+"▍") {
		t.Errorf("after j: line 2 should have green ▍ gutter; got first 60 chars:\n%q",
			lines[1][:min(60, len(lines[1]))])
	}
}

// TestFileView_ComposerKeepsFileContext: pressing `c` in the file
// viewer must NOT swap the viewport out from under the user. The
// comment composer overlays the file viewer, not the message
// viewport — otherwise pressing `c` feels like teleporting back to
// the last agent message.
func TestFileView_ComposerKeepsFileContext(t *testing.T) {
	m := *attachFileFixture(t)
	m.width = 80
	m.height = 24
	openFile(t, &m, "src/main.go")
	if m.state != stateFileView {
		t.Fatalf("setup: expected stateFileView, got %v", m.state)
	}

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = upd.(model)
	if m.state != stateCommentComposer {
		t.Fatalf("after c: expected stateCommentComposer; got %v", m.state)
	}
	if m.commentAnchor.kind != 1 { // render.CommentFile
		t.Fatalf("after c: anchor kind should be CommentFile; got %v", m.commentAnchor.kind)
	}

	view := m.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "alpha") {
		t.Errorf("file content should remain visible during file-kind composer; got:\n%q",
			plain)
	}
}

// TestFileNav_SearchActivatesAndFilters: pressing `/` activates
// fuzzy search; typing characters filters visible entries by
// subsequence match against the path. Collapsed dirs are
// overridden during search so a hidden nested file becomes visible.
func TestFileNav_SearchActivatesAndFilters(t *testing.T) {
	m := *attachFileFixture(t)
	m.width = 80
	m.height = 24

	// Activate search.
	upd, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = upd.(model)
	if !m.fileSearchActive {
		t.Fatalf("after /: expected fileSearchActive=true; got false")
	}

	// Type "pkg" — should match src/pkg and src/pkg/a.go (both have "pkg" in path).
	for _, r := range "pkg" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	visible := m.visibleFileEntries()
	wantPaths := map[string]bool{"src/pkg": true, "src/pkg/a.go": true}
	if len(visible) != len(wantPaths) {
		t.Fatalf("after typing pkg: expected %d matches; got %d (%v)",
			len(wantPaths), len(visible), visiblePathList(visible))
	}
	for _, e := range visible {
		if !wantPaths[e.Path] {
			t.Errorf("unexpected match %q", e.Path)
		}
	}

	// src/pkg/a.go is hidden in the unfiltered tree (src is
	// collapsed); search must surface it anyway.
	var sawAgo bool
	for _, e := range visible {
		if e.Path == "src/pkg/a.go" {
			sawAgo = true
		}
	}
	if !sawAgo {
		t.Errorf("search should surface hidden nested files (src/pkg/a.go); got %v",
			visiblePathList(visible))
	}
}

// TestFileNav_SearchFuzzySubsequence: fuzzy matching is
// subsequence (chars in order), case-insensitive. "mig" matches
// "src/main.go" (m, i, g all present in order) but not "src/pkg".
func TestFileNav_SearchFuzzySubsequence(t *testing.T) {
	m := *attachFileFixture(t)
	upd, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = upd.(model)
	for _, r := range "MIG" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	visible := m.visibleFileEntries()
	if len(visible) != 1 {
		t.Fatalf("expected 1 match for 'MIG'; got %d (%v)", len(visible), visiblePathList(visible))
	}
	if visible[0].Path != "src/main.go" {
		t.Errorf("expected src/main.go; got %q", visible[0].Path)
	}

	// "xyz" matches nothing.
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = upd.(model)
	for _, r := range "xyz" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	if v := m.visibleFileEntries(); len(v) != 0 {
		t.Errorf("expected 0 matches for 'xyz'; got %v", visiblePathList(v))
	}
}

// TestFileNav_SearchBackspace: backspace trims the query and
// the visible list grows back.
func TestFileNav_SearchBackspace(t *testing.T) {
	m := *attachFileFixture(t)
	upd, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = upd.(model)
	for _, r := range "pkg" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = upd.(model)
	if string(m.fileSearch) != "pk" {
		t.Errorf("after backspace: query=%q want %q", string(m.fileSearch), "pk")
	}
	visible := m.visibleFileEntries()
	// "pk" matches src/pkg (p,k) and src/pkg/a.go (p,k), and README.md (none).
	if len(visible) != 2 {
		t.Errorf("expected 2 matches for 'pk'; got %d (%v)", len(visible), visiblePathList(visible))
	}
}

// TestFileNav_SearchEscCancels: Esc turns off search and keeps
// the user's view (the search input disappears, no filter applied).
func TestFileNav_SearchEscCancels(t *testing.T) {
	m := *attachFileFixture(t)
	upd, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = upd.(model)
	for _, r := range "pkg" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = upd.(model)
	if m.fileSearchActive {
		t.Errorf("after Esc: fileSearchActive should be false")
	}
	if len(m.fileSearch) != 0 {
		t.Errorf("after Esc: query should be empty; got %q", string(m.fileSearch))
	}
	// Unfiltered tree (collapse restored): only README.md + src visible.
	visible := m.visibleFileEntries()
	if len(visible) != 2 {
		t.Errorf("after Esc: expected unfiltered visible; got %v", visiblePathList(visible))
	}
}

// TestFileNav_SearchEnterOpensFile: pressing Enter on a matched
// file opens the viewer; search is exited.
func TestFileNav_SearchEnterOpensFile(t *testing.T) {
	m := *attachFileFixture(t)
	upd, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = upd.(model)
	for _, r := range "main" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	if m.state != stateFileView {
		t.Errorf("after Enter in search: expected stateFileView; got %v", m.state)
	}
	if m.fileViewer.path != "src/main.go" {
		t.Errorf("expected viewer path 'src/main.go'; got %q", m.fileViewer.path)
	}
	if m.fileSearchActive {
		t.Errorf("after Enter: search should be exited")
	}
}

// TestFileNav_SearchArrowKeysNavigate: in search mode, j/k are
// typed into the query (muscle memory gives way); Up/Down arrows
// navigate the filtered list.
func TestFileNav_SearchArrowKeysNavigate(t *testing.T) {
	m := *attachFileFixture(t)
	upd, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = upd.(model)
	for _, r := range "src" {
		upd, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = upd.(model)
	}
	visible := m.visibleFileEntries()
	if len(visible) < 2 {
		t.Fatalf("setup: expected >= 2 matches for 'src'; got %d", len(visible))
	}

	start := m.fileCursor
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = upd.(model)
	if m.fileCursor != start+1 {
		t.Errorf("Down arrow should advance cursor; start=%d now=%d", start, m.fileCursor)
	}
	if string(m.fileSearch) != "src" {
		t.Errorf("Down arrow should not modify query; got %q", string(m.fileSearch))
	}
}

// TestFileView_WholeFileCommentUsesLineRange: pressing `c` in
// stateFileView without visual mode (whole-file anchor) must
// produce a `file` (not `file-inline`) comment with char range
// -1, not a degenerate inline byte-0 selection that yields an
// empty excerpt.
func TestFileView_WholeFileCommentUsesLineRange(t *testing.T) {
	m := *attachFileFixture(t)
	openFile(t, &m, "src/main.go")

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = upd.(model)
	if m.state != stateCommentComposer {
		t.Fatalf("after c: expected stateCommentComposer; got %v", m.state)
	}
	if m.commentAnchor.charA != -1 || m.commentAnchor.charC != -1 {
		t.Errorf("whole-file anchor should have charA=charC=-1 (line range); got (%d,%d)",
			m.commentAnchor.charA, m.commentAnchor.charC)
	}

	m.commentTa.SetValue("rename alpha")
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	c0 := m.comments[0]
	if c0.CharStart != -1 || c0.CharEnd != -1 {
		t.Errorf("saved whole-file comment should have CharStart=CharEnd=-1; got (%d,%d)",
			c0.CharStart, c0.CharEnd)
	}
	if c0.Source != "" {
		t.Errorf("saved whole-file comment should have empty Source; got %q", c0.Source)
	}

	// Flush and verify the appendix uses the `file` marker, not
	// `file-inline ""`.
	prev := sendToPane
	var sent string
	sendToPane = func(_, text string) error { sent = text; return nil }
	t.Cleanup(func() { sendToPane = prev })
	upd, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = upd.(model)
	if !strings.Contains(sent, "- file ") || strings.Contains(sent, "file-inline") {
		t.Errorf("appendix should use `file` marker, not `file-inline`; payload:\n%s", sent)
	}
	if strings.Contains(sent, `""`) {
		t.Errorf("appendix should not contain an empty excerpt; payload:\n%s", sent)
	}
}

// TestFileNav_WholeFileCommentUsesLineRange: pressing `c` on a
// file in the dir navigator (stateFileNav) must produce a line-
// range file-kind comment, same as the file-viewer's whole-file
// path. Pre-fix this also yielded `file-inline ""`.
func TestFileNav_WholeFileCommentUsesLineRange(t *testing.T) {
	m := *attachFileFixture(t)
	// Move cursor onto src/main.go.
	visible := m.visibleFileEntries()
	for i, e := range visible {
		if e.Path == "src/main.go" {
			m.fileCursor = i
			break
		}
	}

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = upd.(model)
	if m.state != stateCommentComposer {
		t.Fatalf("after c: expected stateCommentComposer; got %v", m.state)
	}
	if m.commentAnchor.charA != -1 || m.commentAnchor.charC != -1 {
		t.Errorf("file-nav whole-file anchor should have charA=charC=-1; got (%d,%d)",
			m.commentAnchor.charA, m.commentAnchor.charC)
	}
}

// TestFileView_VisualMultiLineStaysLineRange: pressing `v` then
// `j` `k` (multi-line visual) then `c` must produce a line-range
// file comment, not a degenerate inline selection.
func TestFileView_VisualMultiLineStaysLineRange(t *testing.T) {
	m := *attachFileFixture(t)
	openFile(t, &m, "src/main.go")

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = upd.(model)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = upd.(model)
	if m.commentAnchor.charA != -1 || m.commentAnchor.charC != -1 {
		t.Errorf("multi-line visual should stay line-range; got charA=%d charC=%d",
			m.commentAnchor.charA, m.commentAnchor.charC)
	}
	if m.commentAnchor.lineStart != 1 || m.commentAnchor.lineEnd != 2 {
		t.Errorf("multi-line visual (line 1 + j to 2) should give 1..2; got %d..%d",
			m.commentAnchor.lineStart, m.commentAnchor.lineEnd)
	}
}
