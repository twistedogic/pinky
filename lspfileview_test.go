package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	pinkylsp "github.com/twistedogic/pinky/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// fakeLSPManager is a tiny in-memory stub of the LSP Manager
// surface the model layer touches. It records every method call
// so the test can assert didOpen / didClose fire in the right
// order. ponytail: the model only depends on the Manager type's
// methods (DidOpen / DidClose / Dispatch / ServerStatus / Shutdown).
// Swapping in a fake keeps the model layer testable without
// spinning up gopls.
type fakeLSPManager struct {
	calls  []string
	status pinkylsp.ServerStatus
}

func (f *fakeLSPManager) record(method string) {
	f.calls = append(f.calls, method)
}

func (f *fakeLSPManager) DidOpen(_ context.Context, _, _ string) { f.record("DidOpen") }
func (f *fakeLSPManager) DidClose(_ context.Context, _ string)  { f.record("DidClose") }
func (f *fakeLSPManager) ServerStatus(string) pinkylsp.ServerStatus {
	f.record("ServerStatus")
	return f.status
}

// fakeBridge records d / R / K keypress routing into the bridge.
// The model layer only calls RequestDefinition / RequestReferences
// / RequestHover; this satisfies the lspBridge interface.
// Ponytail: the last (path, line, char) tuple is captured so
// tests can assert the bridge was called at the word under the
// cursor, not at the cursor's exact charPos.
type fakeBridge struct {
	defCalls   int
	refCalls   int
	hoverCalls int
	lastPath   string
	lastLine   int
	lastChar   int
}

func (b *fakeBridge) RequestDefinition(_ context.Context, path string, line, char int) tea.Cmd {
	b.defCalls++
	b.lastPath, b.lastLine, b.lastChar = path, line, char
	return nil
}
func (b *fakeBridge) RequestReferences(_ context.Context, path string, line, char int) tea.Cmd {
	b.refCalls++
	b.lastPath, b.lastLine, b.lastChar = path, line, char
	return nil
}
func (b *fakeBridge) RequestHover(_ context.Context, path string, line, char int) tea.Cmd {
	b.hoverCalls++
	b.lastPath, b.lastLine, b.lastChar = path, line, char
	return nil
}

// pathFromURI builds the absolute path the location tests use to
// construct file:// URIs. Helper extracted so the test bodies
// stay readable.
func pathFromURI(t *testing.T, base, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(base, rel))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// TestFileView_Definition_OneResult_Jumps: a LocationsMsg with
// exactly one location moves the cursor to the location's
// (line, char). Spec D4 / scenario "`d` on a symbol with one
// definition jumps".
func TestFileView_Definition_OneResult_Jumps(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	target := pathFromURI(t, m.fileRoot, "src/main.go")
	loc := pinkylsp.Location{
		URI: protocol.DocumentURI("file://" + target),
		Range: protocol.Range{
			Start: protocol.Position{Line: 1, Character: 2},
		},
	}
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindDefinition,
		Locations: []pinkylsp.Location{loc},
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.fileViewer.cursor != 2 {
		t.Errorf("after one-result definition: cursor = %d; want 2", m.fileViewer.cursor)
	}
	if m.fileViewer.charPos != 2 {
		t.Errorf("after one-result definition: charPos = %d; want 2", m.fileViewer.charPos)
	}
}

// TestFileView_Definition_NResults_Picker: a LocationsMsg with
// N>1 locations transitions to stateLSPPicker with the picker
// populated. Spec D4 / scenario "`d` on an overloaded symbol
// opens the picker".
func TestFileView_Definition_NResults_Picker(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	target := pathFromURI(t, m.fileRoot, "src/main.go")
	loc := func(line uint32) pinkylsp.Location {
		return pinkylsp.Location{
			URI:   protocol.DocumentURI("file://" + target),
			Range: protocol.Range{Start: protocol.Position{Line: line, Character: 0}},
		}
	}
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindDefinition,
		Locations: []pinkylsp.Location{loc(0), loc(2), loc(4)},
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.state != stateLSPPicker {
		t.Errorf("after N>1 definition: state = %v; want stateLSPPicker", m.state)
	}
	if len(m.lspPicker.locations) != 3 {
		t.Errorf("picker locations: got %d; want 3", len(m.lspPicker.locations))
	}
}

// TestFileView_Definition_ZeroResults_Silent: an empty LocationsMsg
// leaves stateFileView unchanged. Spec D4 / scenario "`d` on
// whitespace or punctuation is silent".
func TestFileView_Definition_ZeroResults_Silent(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	before := m.fileViewer.cursor
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindDefinition,
		Locations: nil,
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.state != stateFileView {
		t.Errorf("after 0-result definition: state = %v; want stateFileView", m.state)
	}
	if m.fileViewer.cursor != before {
		t.Errorf("cursor moved on silent definition: got %d; want %d", m.fileViewer.cursor, before)
	}
}

// TestFileView_References_AlwaysPicker: a single references
// result goes through the picker (no auto-jump). Spec D5.
func TestFileView_References_AlwaysPicker(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	target := pathFromURI(t, m.fileRoot, "src/main.go")
	loc := pinkylsp.Location{
		URI:   protocol.DocumentURI("file://" + target),
		Range: protocol.Range{Start: protocol.Position{Line: 0, Character: 0}},
	}
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindReferences,
		Locations: []pinkylsp.Location{loc},
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.state != stateLSPPicker {
		t.Errorf("after 1-result references: state = %v; want stateLSPPicker", m.state)
	}
	if m.lspPicker.label != "references" {
		t.Errorf("picker label: got %q; want references", m.lspPicker.label)
	}
}

// TestFileView_Hover_ShowsModal: a non-empty HoverMsg opens the
// hover modal with the content visible in its viewport. Spec D8 /
// scenario "`K` on an identifier renders the hover popup".
func TestFileView_Hover_ShowsModal(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "func Foo() error"})
	m = updatedModelPtr(upd)
	if !m.hoverModal.visible {
		t.Fatal("hover modal not visible after HoverMsg")
	}
	if got := strings.TrimSpace(m.hoverModal.viewport.View()); !strings.Contains(got, "func Foo() error") {
		t.Errorf("hover modal content missing body; got %q", got)
	}
}

// TestFileView_Hover_ShowsFullContent: multi-line hover content
// is rendered without truncation. Spec D8 — the modal is
// scrollable; truncation lives in the viewport, not in the
// content path.
func TestFileView_Hover_ShowsFullContent(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "first line\nsecond line\nthird"})
	m = updatedModelPtr(upd)
	if !m.hoverModal.visible {
		t.Fatal("hover modal not visible after HoverMsg")
	}
	if got := m.hoverModal.viewport.View(); !strings.Contains(got, "second line") {
		t.Errorf("hover modal dropped second line; got %q", got)
	}
}

// TestFileView_HoverModal_DismissedByAnyKey: any non-scroll key
// dismisses the hover modal. Matches the design choice that the
// popup behaves like a fzf preview / vim quickfix window — first
// keypress (other than the scroll keys) drops it.
func TestFileView_HoverModal_DismissedByAnyKey(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "func Foo()"})
	m = updatedModelPtr(upd)
	if !m.hoverModal.visible {
		t.Fatal("setup: hover modal not visible after HoverMsg")
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updatedModelPtr(upd)
	if m.hoverModal.visible {
		t.Errorf("hover modal still visible after `a`; want dismissed")
	}
}

// TestFileView_HoverModal_ScrollKeysScrollNotDismiss: the modal
// has its own viewport, so j/k and PgDn/PgUp scroll it instead
// of dismissing the popup or moving the file cursor. Without
// this carve-out the modal isn't really scrollable.
func TestFileView_HoverModal_ScrollKeysScrollNotDismiss(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	// Long content so j has something to scroll past.
	lines := []string{}
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: strings.Join(lines, "\n")})
	m = updatedModelPtr(upd)
	before := m.hoverModal.viewport.YOffset()

	upd, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updatedModelPtr(upd)

	if !m.hoverModal.visible {
		t.Fatal("hover modal dismissed by j; want it to scroll instead")
	}
	if m.hoverModal.viewport.YOffset() <= before {
		t.Errorf("j did not scroll modal: yOffset before=%d after=%d", before, m.hoverModal.viewport.YOffset())
	}
}

// TestFileView_HoverModal_RefiresReplace: pressing K while the
// modal is open replaces the content. (The handler dismisses the
// old modal then issues a new query; the new HoverMsg populates
// a fresh modal.)
func TestFileView_HoverModal_RefiresReplace(t *testing.T) {
	m := attachFileFixture(t)
	m.lsphub = &fakeBridge{}
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "func Old()"})
	m = updatedModelPtr(upd)
	// K while modal open → dismiss + new query.
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'K', Text: "K"})
	m = updatedModelPtr(upd)
	// Simulate the reply arriving.
	upd, _ = m.Update(pinkylsp.HoverMsg{Contents: "func New()"})
	m = updatedModelPtr(upd)
	if got := m.hoverModal.viewport.View(); !strings.Contains(got, "func New()") {
		t.Errorf("after re-K, modal content missing new body; got %q", got)
	}
	if strings.Contains(m.hoverModal.viewport.View(), "func Old()") {
		t.Errorf("old hover content still present after re-K")
	}
}

// TestFileView_HoverModal_RendersIntoView: after a HoverMsg, the
// modal's bordered box appears in the rendered View(). Verifies
// overlay wiring (not just state) end-to-end.
func TestFileView_HoverModal_RendersIntoView(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "func Foo() error"})
	m = updatedModelPtr(upd)
	rendered := m.View().Content
	if !strings.Contains(rendered, "func Foo() error") {
		t.Errorf("View() did not include hover content; got:\n%s", rendered)
	}
	// Border characters from lipgloss.RoundedBorder.
	if !strings.Contains(rendered, "╭") && !strings.Contains(rendered, "+") {
		t.Errorf("View() did not include a border; got:\n%s", rendered)
	}
}

// TestFileView_HoverModal_EscDismissesNotExits: when the hover
// modal is active, Esc must dismiss the modal AND keep the file
// viewer (not leave to dir nav). Otherwise reading a hover is a
// trap — pressing Esc to close the popup also navigates out of
// the file.
func TestFileView_HoverModal_EscDismissesNotExits(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "func Foo()"})
	m = updatedModelPtr(upd)
	if !m.hoverModal.visible {
		t.Fatal("setup: modal not visible")
	}
	if m.state != stateFileView {
		t.Fatalf("setup: expected stateFileView; got %v", m.state)
	}

	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updatedModelPtr(upd)

	if m.hoverModal.visible {
		t.Errorf("Esc should dismiss modal; still visible")
	}
	if m.state != stateFileView {
		t.Errorf("Esc should keep stateFileView; got %v", m.state)
	}
}

// TestFileView_HoverModal_QDismissesNotQuits: same as Esc — when the
// hover modal is active, q must dismiss the modal rather than
// quitting the app. Without this carve-out reading a hover means
// "press q to close it" silently kills pinky.
func TestFileView_HoverModal_QDismissesNotQuits(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.HoverMsg{Contents: "func Foo()"})
	m = updatedModelPtr(upd)
	if !m.hoverModal.visible {
		t.Fatal("setup: modal not visible")
	}

	upd, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = updatedModelPtr(upd)

	if m.hoverModal.visible {
		t.Errorf("q should dismiss modal; still visible")
	}
	if cmd != nil {
		// tea.Quit is the only cmd handleFileViewKey produces for
		// 'q'. Any non-nil cmd means the quit path fired.
		t.Errorf("q should not return a cmd (quit path); got %T", cmd)
	}
	if m.state != stateFileView {
		t.Errorf("q should keep stateFileView; got %v", m.state)
	}
}

// TestFileView_MissingServerHint_StaysInFooter: a missing-server
// reply still sets the footer hint (NOT the modal) — errors stay
// visible until acknowledged, while hover is a transient popup.
func TestFileView_MissingServerHint_StaysInFooter(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		ServerMissing: true,
		InstallHint:   "go install golang.org/x/tools/gopls@latest",
		WorkDir:       m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.missingServerHint == "" {
		t.Errorf("expected missingServerHint to be set on the model")
	}
	if m.hoverModal.visible {
		t.Errorf("missing-server hint should not open the hover modal")
	}
}

// TestFileView_MissingServerHint_ShownOnce: a LocationsMsg with
// ServerMissing=true sets the install hint on the model.
func TestFileView_MissingServerHint_ShownOnce(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		ServerMissing: true,
		InstallHint:   "go install golang.org/x/tools/gopls@latest",
		WorkDir:       m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.missingServerHint == "" {
		t.Errorf("expected missingServerHint to be set on the model")
	}
}

// TestFileView_DidOpenOnEnter / DidCloseOnEsc: stub DidOpen /
// DidClose on the model so we can assert the lifecycle events
// fire when expected. ponytail: the model layer accepts an
// lspManager interface; for tests we substitute the fake.
func TestFileView_DidOpenOnEnter(t *testing.T) {
	m := attachFileFixture(t)
	fm := &fakeLSPManager{}
	m.lsp = fm
	openFile(t, m, "src/main.go")
	if !slices.Contains(fm.calls, "DidOpen") {
		t.Errorf("expected DidOpen in calls; got %v", fm.calls)
	}
}

func TestFileView_DidCloseOnEsc(t *testing.T) {
	m := attachFileFixture(t)
	fm := &fakeLSPManager{}
	m.lsp = fm
	openFile(t, m, "src/main.go")
	// Esc to dir nav triggers exitFileViewer, which sends DidClose.
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updatedModelPtr(upd)
	if !slices.Contains(fm.calls, "DidClose") {
		t.Errorf("expected DidClose in calls; got %v", fm.calls)
	}
}

// TestFileView_StatusLine_LSPChipReady: in the file viewer, the
// status line carries an `[LSP gopls ●]` chip when the manager
// reports ServerReady for the open file. Tells the user d/R/K
// queries are live, even before any key is pressed.
func TestFileView_StatusLine_LSPChipReady(t *testing.T) {
	m := attachFileFixture(t)
	fm := &fakeLSPManager{status: pinkylsp.ServerStatus{
		LangID: "gopls", Command: "gopls", State: pinkylsp.ServerReady,
	}}
	m.lsp = fm
	openFile(t, m, "src/main.go")

	if got := ansi.Strip(m.statusLine()); !strings.Contains(got, "[LSP gopls ●]") {
		t.Errorf("status line missing ready chip; got %q", got)
	}
}

// TestFileView_StatusLine_LSPChipMissing: a ServerMissing state
// surfaces `✗` so the user can see why d/R/K are silent (the
// gopls binary isn't on PATH).
func TestFileView_StatusLine_LSPChipMissing(t *testing.T) {
	m := attachFileFixture(t)
	fm := &fakeLSPManager{status: pinkylsp.ServerStatus{
		LangID: "gopls", Command: "gopls", State: pinkylsp.ServerMissing,
	}}
	m.lsp = fm
	openFile(t, m, "src/main.go")

	if got := ansi.Strip(m.statusLine()); !strings.Contains(got, "[LSP gopls ✗]") {
		t.Errorf("status line missing missing-state chip; got %q", got)
	}
}

// TestFileView_StatusLine_NoLSPOutsideFileView: the chip only
// renders in stateFileView; outside the file viewer the status
// line stays slim. Avoids cluttering the nav / compose bars.
func TestFileView_StatusLine_NoLSPOutsideFileView(t *testing.T) {
	m := attachFileFixture(t)
	fm := &fakeLSPManager{status: pinkylsp.ServerStatus{
		LangID: "gopls", Command: "gopls", State: pinkylsp.ServerReady,
	}}
	m.lsp = fm
	m.state = stateFileNav // back out of the file viewer

	if got := ansi.Strip(m.statusLine()); strings.Contains(got, "LSP") {
		t.Errorf("status line should not show LSP chip outside file view; got %q", got)
	}
}

// TestLSPPicker_JumpsOnSameFile: pressing Enter on a same-file
// location moves the cursor.
func TestLSPPicker_JumpsOnSameFile(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	target := pathFromURI(t, m.fileRoot, "src/main.go")
	loc := pinkylsp.Location{
		URI:   protocol.DocumentURI("file://" + target),
		Range: protocol.Range{Start: protocol.Position{Line: 2, Character: 1}},
	}
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindReferences,
		Locations: []pinkylsp.Location{loc},
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.state != stateLSPPicker {
		t.Fatalf("setup: expected stateLSPPicker; got %v", m.state)
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updatedModelPtr(upd)
	if m.state != stateFileView {
		t.Errorf("after Enter: state = %v; want stateFileView", m.state)
	}
	if m.fileViewer.cursor != 3 {
		t.Errorf("after Enter: cursor = %d; want 3", m.fileViewer.cursor)
	}
}

// TestLSPPicker_OpensNewFileOnCrossFile: pressing Enter on a
// cross-file location closes the current viewer and opens the
// new one.
func TestLSPPicker_OpensNewFileOnCrossFile(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	// Cross-file target: src/pkg/a.go
	target := pathFromURI(t, m.fileRoot, "src/pkg/a.go")
	loc := pinkylsp.Location{
		URI:   protocol.DocumentURI("file://" + target),
		Range: protocol.Range{Start: protocol.Position{Line: 0, Character: 0}},
	}
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindReferences,
		Locations: []pinkylsp.Location{loc},
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	if m.state != stateLSPPicker {
		t.Fatalf("setup: expected stateLSPPicker; got %v", m.state)
	}
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updatedModelPtr(upd)
	if m.fileViewer.path != "src/pkg/a.go" {
		t.Errorf("after cross-file Enter: path = %q; want src/pkg/a.go", m.fileViewer.path)
	}
}

// TestLSPPicker_EscDismissesWithoutJump: pressing Esc from the
// picker returns to stateFileView with the cursor unchanged.
func TestLSPPicker_EscDismissesWithoutJump(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	beforeCursor := m.fileViewer.cursor
	target := pathFromURI(t, m.fileRoot, "src/main.go")
	loc := pinkylsp.Location{
		URI:   protocol.DocumentURI("file://" + target),
		Range: protocol.Range{Start: protocol.Position{Line: 2, Character: 1}},
	}
	upd, _ := m.Update(pinkylsp.LocationsMsg{
		Kind:      pinkylsp.KindReferences,
		Locations: []pinkylsp.Location{loc},
		WorkDir:   m.fileRoot,
	})
	m = updatedModelPtr(upd)
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updatedModelPtr(upd)
	if m.state != stateFileView {
		t.Errorf("after Esc: state = %v; want stateFileView", m.state)
	}
	if m.fileViewer.cursor != beforeCursor {
		t.Errorf("after Esc: cursor = %d; want %d", m.fileViewer.cursor, beforeCursor)
	}
}

// TestFileView_D_R_K_RoutingKeys: pressing d / R / K in
// stateFileView goes through key.Matches against the LSP
// bindings. The fake bridge records the call.
func TestFileView_D_R_K_RoutingKeys(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	fb := &fakeBridge{}
	m.lsphub = fb
	_, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if fb.defCalls != 1 {
		t.Errorf("after d: defCalls = %d; want 1", fb.defCalls)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if fb.refCalls != 1 {
		t.Errorf("after R: refCalls = %d; want 1", fb.refCalls)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: 'K', Text: "K"})
	if fb.hoverCalls != 1 {
		t.Errorf("after K: hoverCalls = %d; want 1", fb.hoverCalls)
	}
}

// TestFileView_D_FiresAtWordUnderCursor: with the cursor on
// the middle of an identifier, `d` fires the bridge at the
// identifier's start byte — not the cursor's exact position.
// File content is "alpha\nbeta\ngamma\n"; with cursor advanced
// 3 bytes into line 1 (on 'h' of "alpha"), the bridge should see
// (line=1, char=0). Spec D4 / scenario "`d` on a word with
// one definition jumps" (word-routing semantics).
func TestFileView_D_FiresAtWordUnderCursor(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	fb := &fakeBridge{}
	m.lsphub = fb
	// Move cursor 3 bytes into line 1 (on 'h' of "alpha").
	for range 3 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.charPos != 3 {
		t.Fatalf("setup: charPos = %d; want 3", m.fileViewer.charPos)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if fb.defCalls != 1 {
		t.Fatalf("after d: defCalls = %d; want 1", fb.defCalls)
	}
	if fb.lastLine != 1 || fb.lastChar != 0 {
		t.Errorf("after d at (1, 3): bridge got (line=%d, char=%d); want (1, 0) (start of 'alpha')",
			fb.lastLine, fb.lastChar)
	}
}

// TestFileView_D_OnWhitespaceFiresAtCharPos: cursor on
// whitespace, `d` fires at the exact charPos (no nearest-word
// search). The server is expected to return 0 results.
func TestFileView_D_OnWhitespaceFiresAtCharPos(t *testing.T) {
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	fb := &fakeBridge{}
	m.lsphub = fb
	// Move cursor 5 bytes in (the newline at end of line 1).
	for range 5 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	// charPos is clamped to len("alpha") = 5; newline not in line.
	if m.fileViewer.charPos != 5 {
		t.Fatalf("setup: charPos = %d; want 5 (line end)", m.fileViewer.charPos)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if fb.lastChar != 5 {
		t.Errorf("after d at line end: bridge char = %d; want 5 (charPos unchanged)", fb.lastChar)
	}
}