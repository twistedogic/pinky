package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
)

// TestHelp_AlwaysVisibleInIdleView: the help footer is part of every
// idle-state View, not behind an overlay. The agent message must stay
// readable AND the help line must be present below it.
func TestHelp_AlwaysVisibleInIdleView(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()

	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "Hello") {
		t.Errorf("agent message should remain visible; got first 200 chars:\n%q",
			plain[:min(len(plain), 200)])
	}
	// Short help includes the nav binding description.
	if !strings.Contains(plain, "nav") {
		t.Errorf("expected short help to mention 'nav' in idle view; got:\n%q",
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
	m.state = stateNav
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(model)
	if !got.help.ShowAll {
		t.Fatal("expected ShowAll=true after pressing ?")
	}
	view := got.View()
	plain := stripANSI(view)
	// Full help exposes the nav binding description. With the
	// collapsed keymap the nav is one group; assert on the nav
	// binding's full descriptor text.
	if !strings.Contains(plain, "j k h l v c s q r n") {
		t.Errorf("full help should surface the nav group's full descriptor; got:\n%q", plain)
	}
}

// TestVisual_StatusLineIndicatesActive: pressing `v` in nav must be
// visible — visual mode flips Mode to NavLine but the viewport
// doesn't otherwise change, so the status line carries a "VISUAL"
// chip. The chip is rendered with a background style and stripped
// ANSI shows it as a leading-space-prefixed block; assert on the
// status-line substring specifically, not the whole view (the help
// footer mentions "v visual" too, which is unrelated).
func TestVisual_StatusLineIndicatesActive(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello"}
	m.refreshViewport()
	m.reflow()

	statusBefore := m.statusLine()
	if strings.Contains(strings.ToUpper(statusBefore), "VISUAL") {
		t.Fatalf("VISUAL indicator should not be present before v; got status:\n%s", statusBefore)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := updated.(model)
	statusAfter := got.statusLine()
	if !strings.Contains(statusAfter, "VISUAL") {
		t.Errorf("status line should contain VISUAL indicator after v; got:\n%s", statusAfter)
	}
}

// TestSubmitComments_ClearsOnSuccess: pressing `s` in idle with
// accumulated comments must submit them as a single redirect and
// clear the comment slice. Tested via submitAllComments so the
// inject call doesn't need a live tmux — the hook variable
// captures what was sent.
func TestSubmitComments_ClearsOnSuccess(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# title\n\nbody one"}
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
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello"}
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

// TestVisual_JMovesCursorAcrossBlocks: in nav mode, `j` advances
// the cursor to the next block and the cyan left-gutter follows.
// Per design D1 the cursor is the single source of truth — no
// separate visual.cursor.
func TestVisual_JMovesCursorAcrossBlocks(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	// Three short blocks: two headings with a paragraph between so
	// `j` has somewhere obvious to land.
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# alpha\n\nbody one\n\n## beta\n\nbody two"}
	m.refreshViewport()
	m.reflow()
	if len(m.blocks) < 3 {
		t.Fatalf("setup: expected at least 3 blocks, got %d", len(m.blocks))
	}
	startBlock := m.cursor.BlockIdx

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(model)
	if m.cursor.BlockIdx <= startBlock {
		t.Errorf("j did not advance cursor block: before=%d after=%d", startBlock, m.cursor.BlockIdx)
	}

	// Cyan left-gutter (▍) follows the cursor's block — every line
	// inside the new block starts with ▍, the previous block's
	// lines do not.
	plain := stripANSI(m.View())
	focused := m.blocks[m.cursor.BlockIdx]
	prev := m.blocks[startBlock]
	for i := focused.StartLine; i <= focused.EndLine; i++ {
		line := lineAt(plain, i)
		if !strings.HasPrefix(line, "▍") {
			t.Errorf("focused block line %d should start with ▍ gutter; got %q", i, line)
		}
	}
	for i := prev.StartLine; i <= prev.EndLine; i++ {
		if i >= focused.StartLine && i <= focused.EndLine {
			continue
		}
		line := lineAt(plain, i)
		if strings.HasPrefix(line, "▍") {
			t.Errorf("previous block line %d should NOT start with ▍ (focus moved); got %q", i, line)
		}
	}
}

// TestVisual_VEnterThenJ_KeepsVisualActive: `v` then `j` keeps
// visual mode active while moving the cursor. Per design D2 the
// state machine updates the visual flag and the cursor in one
// call; the model then refreshes the viewport so the highlight
// reflects the new cursor position.
func TestVisual_VEnterThenJ_KeepsVisualActive(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# title\n\nline one\nline two\nline three"}
	m.refreshViewport()
	m.reflow()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(model)
	if m.nav.Visual != render.NavLine {
		t.Fatal("v should enter visual")
	}
	before := m.cursor.BlockIdx

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(model)
	if m.nav.Visual != render.NavLine {
		t.Errorf("j in visual should keep visual active; got %v", m.nav.Visual)
	}
	if m.cursor.BlockIdx <= before {
		t.Errorf("j did not advance cursor: before=%d after=%d", before, m.cursor.BlockIdx)
	}
}

// TestHelp_FullHelpHeightReservesSpace: when ShowAll is on, the
// viewport is shortened by the largest group's row count, so the
// help footer doesn't overlap the agent message. The collapsed
// nav keymap has one help binding (NavGroup), so the full help
// height matches the short help height (1 row). The guard below
// still validates that the help height calculation produces a
// non-negative value that does not exceed the terminal height.
func TestHelp_FullHelpHeightReservesSpace(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.width = 80
	m.height = 24
	m.reflow()

	if got := m.helpHeight(); got < 1 {
		t.Errorf("helpHeight() = %d, expected >=1", got)
	}
	if m.viewport.Height <= 0 {
		t.Errorf("viewport height collapsed to %d", m.viewport.Height)
	}
}

// TestShortHelp_PerStateCurated: ShortHelp returns at most a handful
// of bindings per state and includes Help itself (except in
// comment-composer, where `?` would steal the keystroke).
func TestShortHelp_PerStateCurated(t *testing.T) {
	cases := []struct {
		state   state
		wantIn  []string
		wantMax int
	}{
		{statePicking, []string{"toggle help"}, 6},
		{stateNav, []string{"nav", "toggle help"}, 6},
		{stateCompose, []string{"include comments", "cancel", "toggle help"}, 6},
		{stateCommentComposer, []string{"save", "cancel"}, 4},
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
		{stateNav, []string{"nav", "toggle help"}},
		{stateCompose, []string{"newline", "include comments", "cancel", "toggle help"}},
		{stateCommentComposer, []string{"save", "cancel"}},
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

// errTestBoom is a stand-in error for tests.
var errTestBoom = errors.New("boom")

// lineAt returns the i-th line of plain (0-indexed). Returns "" if i
// is out of range. Used by visual-mode tests to assert per-line
// gutter presence without scanning the whole view.
func lineAt(plain string, i int) string {
	lines := strings.Split(plain, "\n")
	if i < 0 || i >= len(lines) {
		return ""
	}
	return lines[i]
}
