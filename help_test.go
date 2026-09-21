package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
)

// TestHelp_AlwaysVisibleInIdleView: the help footer is part of every
// idle-state View, not behind an overlay. The agent message must stay
// readable AND the help line must be present below it.
func TestHelp_AlwaysVisibleInIdleView(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()

	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "Hello") {
		t.Errorf("agent message should remain visible; got first 200 chars:\n%q",
			plain[:min(len(plain), 200)])
	}
	// Short help includes the Compose binding.
	if !strings.Contains(plain, "compose") {
		t.Errorf("expected short help to mention 'compose' in idle view; got:\n%q",
			plain)
	}
}

// TestHelp_AlwaysVisibleInPickerView: even the picker (no viewport)
// gets a help footer so the user knows how to navigate.
func TestHelp_AlwaysVisibleInPickerView(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = statePicking
	m.width = 80
	m.height = 24

	view := m.View()
	plain := stripANSI(view)
	// Picker ShortHelp includes Up/Down/Pick/Help.
	for _, want := range []string{"up", "down", "select", "toggle help"} {
		if !strings.Contains(plain, want) {
			t.Errorf("picker view should include %q in help footer; got:\n%q", want, plain)
		}
	}
}

// TestHelp_AlwaysVisibleInErrorView: error state still surfaces the
// dismiss-any-key hint via the help footer.
func TestHelp_AlwaysVisibleInErrorView(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.err = errTestBoom
	m.state = stateError
	m.width = 80
	m.height = 24

	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(strings.ToLower(plain), "error") {
		t.Errorf("error message should render; got: %q", plain)
	}
	if !strings.Contains(plain, "dismiss") {
		t.Errorf("help footer should mention 'dismiss' in error state; got:\n%q", plain)
	}
}

// TestHelp_QuestionMarkTogglesFullHelp: `?` flips ShowAll; the help
// rendered in View() is the multi-column version when toggled on.
func TestHelp_QuestionMarkTogglesFullHelp(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(model)
	if !got.help.ShowAll {
		t.Fatal("expected ShowAll=true after pressing ?")
	}
	view := got.View()
	plain := stripANSI(view)
	// Full help exposes groups that the short help truncates away:
	// "next comment", "prev comment", etc.
	if !strings.Contains(plain, "next comment") {
		t.Errorf("full help should surface the comments group; got:\n%q", plain)
	}
}

// TestVisual_StatusLineIndicatesActive: pressing V in idle must be
// visible — visual mode flips Mode to SelLine but the viewport
// doesn't otherwise change, so the status line carries a "VISUAL"
// chip. Without this chip the user has no way to tell the keystroke
// was registered.
func TestVisual_StatusLineIndicatesActive(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello"}
	m.refreshViewport()
	m.reflow()

	plain := stripANSI(m.View())
	if strings.Contains(strings.ToUpper(plain), "VISUAL") {
		t.Fatalf("VISUAL indicator should not be present before V; got:\n%s", plain)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	got := updated.(model)
	plain = stripANSI(got.View())
	if !strings.Contains(plain, "VISUAL") {
		t.Errorf("View should contain VISUAL indicator after V; got:\n%s", plain)
	}
}

// TestSubmitComments_ClearsOnSuccess: pressing `s` in idle with
// accumulated comments must submit them as a single redirect and
// clear the comment slice. Tested via submitAllComments so the
// inject call doesn't need a live tmux — the hook variable
// captures what was sent.
func TestSubmitComments_ClearsOnSuccess(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# title\n\nbody one"}
	m.refreshViewport()

	m.comments = []render.Comment{
		{BlockIdx: 0, Text: "fix the title", CreatedAt: time.Now()},
		{BlockIdx: 1, Text: "expand the body", CreatedAt: time.Now().Add(time.Second)},
	}

	var sentPane, sentText string
	prev := sendToPane
	sendToPane = func(pane, text string) error {
		sentPane = pane
		sentText = text
		return nil
	}
	t.Cleanup(func() { sendToPane = prev })

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(model)

	if sentPane != m.pane {
		t.Errorf("send target = %q, want %q", sentPane, m.pane)
	}
	if !strings.Contains(sentText, "fix the title") || !strings.Contains(sentText, "expand the body") {
		t.Errorf("submitted text missing comments:\n%s", sentText)
	}
	if len(got.comments) != 0 {
		t.Errorf("comments should be cleared after successful submit; got %d", len(got.comments))
	}
}

// TestSubmitComments_NoCommentsNoop: pressing `s` in idle with no
// accumulated comments must do nothing — no inject call, no state
// churn.
func TestSubmitComments_NoCommentsNoop(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello"}
	m.refreshViewport()

	called := false
	prev := sendToPane
	sendToPane = func(pane, text string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { sendToPane = prev })

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if called {
		t.Error("s with no comments should not call inject.Send")
	}
	if len(updated.(model).comments) != 0 {
		t.Error("comments should remain empty after no-op s")
	}
}

// TestVisual_BracketMovesHighlight: in visual mode, } must move the
// cursor to the next block AND the heavy-border highlight must
// follow. Without this the user has no feedback that the keystroke
// did anything — pressing } either scrolls a viewport line or does
// nothing, depending on which switch arm wins.
func TestVisual_BracketMovesHighlight(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	// Three short blocks: two single-line headings with a paragraph
	// between, so } has somewhere obvious to land.
	m.latest = entry{role: roleAgent, text: "# alpha\n\nbody one\n\n## beta\n\nbody two"}
	m.refreshViewport()
	m.reflow()
	if len(m.blocks) < 3 {
		t.Fatalf("setup: expected at least 3 blocks, got %d", len(m.blocks))
	}

	// Enter visual mode.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	m = updated.(model)
	startBlock := m.visual.CurBlock

	// Press } — cursor should jump to the next block.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = updated.(model)
	if m.visual.CurBlock <= startBlock {
		t.Errorf("} did not advance CurBlock: before=%d after=%d", startBlock, m.visual.CurBlock)
	}

	// The borders (heavy ━━━ lines) should bracket the new focused
	// block. injectBorder inserts the leading border at the source
	// StartLine (no lines before it have been rewritten) and the
	// trailing border one line past the source EndLine (one extra
	// border line was already emitted above).
	plain := stripANSI(m.View())
	borders := borderLines(plain)
	if len(borders) != 2 {
		t.Fatalf("expected exactly 2 border lines, got %d:\n%s", len(borders), plain)
	}
	wantStart := m.blocks[m.visual.CurBlock].StartLine
	wantEnd := m.blocks[m.visual.CurBlock].EndLine + 2 // 1 extra for the leading border
	if borders[0] != wantStart {
		t.Errorf("leading border on rendered line %d, want %d\nview:\n%s", borders[0], wantStart, plain)
	}
	if borders[1] != wantEnd {
		t.Errorf("trailing border on rendered line %d, want %d\nview:\n%s", borders[1], wantEnd, plain)
	}
	// Sanity check: block content sits between the borders, the
	// neighboring blocks do NOT.
	lines := strings.Split(plain, "\n")
	for i := borders[0] + 1; i < borders[1]; i++ {
		if strings.Contains(lines[i], "alpha") || strings.Contains(lines[i], "beta") {
			t.Errorf("neighbor block content leaked inside the highlight at line %d: %q", i, lines[i])
		}
	}
}

// TestVisual_JMovesCursorWithinBlock: in visual mode, j must advance
// the visual cursor (not scroll the viewport by one line). Without
// this the user can't move the cursor at all while in visual mode,
// because the idle-view LineDown binding eats the key first.
func TestVisual_JMovesCursorWithinBlock(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	// A block long enough that j has somewhere to go without leaving it.
	m.latest = entry{role: roleAgent, text: "# title\n\nline one\nline two\nline three"}
	m.refreshViewport()
	m.reflow()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	m = updated.(model)
	before := m.visual.Cursor

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(model)
	if m.visual.Cursor <= before {
		t.Errorf("j did not advance visual cursor: before=%d after=%d", before, m.visual.Cursor)
	}
}

// TestHelp_FullHelpHeightReservesSpace: when ShowAll is on, the
// viewport is shortened by the largest group's row count, so the
// help footer doesn't overlap the agent message.
func TestHelp_FullHelpHeightReservesSpace(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.width = 80
	m.height = 24
	m.reflow()
	shortVP := m.viewport.Height

	m.help.ShowAll = true
	m.reflow()
	fullVP := m.viewport.Height

	if fullVP >= shortVP {
		t.Errorf("expected viewport to shrink when help expands; short=%d full=%d", shortVP, fullVP)
	}
	if got := m.helpHeight(); got < 2 {
		t.Errorf("helpHeight() = %d, expected >=2 for full help", got)
	}
}

// TestShortHelp_PerStateCurated: ShortHelp returns at most a handful
// of bindings per state and includes Help itself (except in
// comment-composer, where `?` would steal the keystroke).
func TestShortHelp_PerStateCurated(t *testing.T) {
	cases := []struct {
		state    state
		wantIn   []string
		wantMax  int
	}{
		{statePicking, []string{"toggle help"}, 6},
		{stateIdle, []string{"compose", "toggle help"}, 6},
		{stateCompose, []string{"send", "toggle help"}, 6},
		{stateError, []string{"dismiss"}, 3},
	}
	for _, c := range cases {
		m := newIdleModelForKeymap(t)
		m.state = c.state
		bindings := m.ShortHelp()
		if len(bindings) > c.wantMax {
			t.Errorf("state=%d: ShortHelp returned %d bindings (max %d)",
				c.state, len(bindings), c.wantMax)
		}
		plain := concatHelp(bindings)
		for _, want := range c.wantIn {
			if !strings.Contains(strings.ToLower(plain), strings.ToLower(want)) {
				t.Errorf("state=%d: ShortHelp missing %q; got:\n%s", c.state, want, plain)
			}
		}
	}
}

// TestFullHelp_CoversStateSpecificKeys: FullHelp returns the
// state-specific groups so the expanded help lists the right keys.
func TestFullHelp_CoversStateSpecificKeys(t *testing.T) {
	cases := []struct {
		state  state
		wantIn []string
	}{
		{statePicking, []string{"up", "down", "select", "toggle help"}},
		{stateIdle, []string{"line down", "line up", "next block",
			"prev block", "bottom", "compose", "mark", "visual",
			"edit comment", "delete comment", "next comment",
			"prev comment", "refresh", "quit", "toggle help"}},
		{stateCompose, []string{"send", "newline", "include comments", "cancel", "toggle help"}},
	}
	for _, c := range cases {
		m := newIdleModelForKeymap(t)
		m.state = c.state
		bindings := m.FullHelp()
		plain := strings.ToLower(concatHelp(flatBindings(bindings)))
		for _, want := range c.wantIn {
			if !strings.Contains(plain, strings.ToLower(want)) {
				t.Errorf("state=%d: FullHelp missing %q; got:\n%s", c.state, want, plain)
			}
		}
	}
}

// stripANSI removes ANSI escape sequences from s for plain-text
// assertions.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func concatHelp(bs []key.Binding) string {
	var b strings.Builder
	for _, kb := range bs {
		h := kb.Help()
		b.WriteString(h.Key)
		b.WriteByte(' ')
		b.WriteString(h.Desc)
		b.WriteByte('\n')
	}
	return b.String()
}

func flatBindings(groups [][]key.Binding) []key.Binding {
	var out []key.Binding
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// errTestBoom is a stand-in error for tests; avoids importing errors
// just for one line.
var errTestBoom = errorString("boom")

type errorString string

func (e errorString) Error() string { return string(e) }

// borderLines returns the 0-indexed line numbers of every heavy
// horizontal border (━━━━) in the rendered view. Used by visual-mode
// tests to assert which block the highlight is currently around.
func borderLines(plain string) []int {
	var out []int
	for i, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "━━") {
			out = append(out, i)
		}
	}
	return out
}
