package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
)

// TestKeymapMarkdown_HasSectionForState verifies the generated
// markdown for each state contains the expected section headings
// and key bindings.
func TestKeymapMarkdown_HasSectionForState(t *testing.T) {
	cases := []struct {
		state       state
		wantSection string
		wantKey     string
	}{
		{statePicking, "**navigation**", "`↑/k`"},
		{stateIdle, "**navigation**", "`↓/j`"},
		{stateIdle, "**compose**", "`c`"},
		{stateCompose, "**send**", "`^S`"},
		{stateCompose, "**cancel**", "`esc`"},
		{stateError, "**dismiss**", "`⏎`"},
	}
	for _, c := range cases {
		md := keymapMarkdown(c.state)
		if !strings.Contains(md, c.wantSection) {
			t.Errorf("state=%d: expected section %q in:\n%s", c.state, c.wantSection, md)
		}
		if !strings.Contains(md, c.wantKey) {
			t.Errorf("state=%d: expected key %q in:\n%s", c.state, c.wantKey, md)
		}
	}
}

// TestKeymapMarkdown_Structure verifies the markdown document has the
// expected top-level structure: a heading, grouped sections, and a
// dismiss hint.
func TestKeymapMarkdown_Structure(t *testing.T) {
	md := keymapMarkdown(stateIdle)
	want := []string{
		"# pinky keymap",
		"press any key to dismiss",
	}
	for _, w := range want {
		if !strings.Contains(md, w) {
			t.Errorf("expected %q in markdown:\n%s", w, md)
		}
	}
}

// TestShowHelpMarkdown_RendersIntoViewport verifies pressing `?`
// puts the rendered keymap markdown into the viewport. The keymap
// markdown goes through the same glamour renderer as the agent
// message, so this test confirms the wiring AND verifies the
// rendered content has ANSI styling (not raw markdown).
func TestShowHelpMarkdown_RendersIntoViewport(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.width = 80
	m.height = 24

	// Seed viewport with the agent message first.
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()

	// Toggle help.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(model)
	if !got.help {
		t.Error("expected help=true after pressing ?")
	}
	content := got.viewport.View()
	plain := stripANSIForTest(content)
	if !strings.Contains(plain, "pinky keymap") {
		t.Errorf("expected 'pinky keymap' in viewport after ?; got first 200 chars:\n%q",
			plain[:min(len(plain), 200)])
	}

	// The rendered output should contain glamour's ANSI escape
	// sequences (38;2;R;G;B for truecolor, 38;5;N for 256-color).
	// Raw markdown text would NOT contain these. This is the
	// regression guard for "keymap shows as raw markdown instead of
	// rendered".
	ansiCount := strings.Count(content, "\x1b[")
	if ansiCount < 5 {
		t.Errorf("expected glamour-rendered content with ANSI codes; got %d ANSI escapes in:\n%s",
			ansiCount, content[:min(len(content), 400)])
	}

	// Also check the viewport is scrolled to top so the user sees
	// the title, not the middle of the document.
	if got.viewport.YOffset != 0 {
		t.Errorf("expected viewport.YOffset=0 after help toggle; got %d", got.viewport.YOffset)
	}

	// Pressing any other key dismisses help and restores the agent
	// message; the next key event is re-processed.
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got2 := updated2.(model)
	if got2.help {
		t.Error("expected help=false after pressing another key")
	}
}

// TestShowHelp_VisibleInFullView confirms the rendered keymap
// appears in the full View() output (not just the viewport), so the
// user actually sees it. Regression guard for "keymap is set on the
// viewport but the View() output doesn't reflect it".
func TestShowHelp_VisibleInFullView(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.width = 80
	m.height = 24
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()

	// Toggle help on.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(model)

	view := got.View()
	plain := stripANSIForTest(view)
	if !strings.Contains(plain, "pinky keymap") {
		t.Errorf("expected 'pinky keymap' in full View() output; got first 400 chars:\n%q",
			plain[:min(len(plain), 400)])
	}
	// The agent message should be GONE from the visible content
	// (the keymap has replaced it).
	if strings.Contains(plain, "Hello") || strings.Contains(plain, "body") {
		t.Errorf("agent message should be hidden while help is showing; got first 400 chars:\n%q",
			plain[:min(len(plain), 400)])
	}
}

// TestKeymapMarkdown_RenderableViaGlamour verifies the generated
// markdown round-trips through glamour without error. Guards
// against syntax errors in the generator.
func TestKeymapMarkdown_RenderableViaGlamour(t *testing.T) {
	for _, s := range []state{statePicking, stateIdle, stateCompose, stateError} {
		md := keymapMarkdown(s)
		r, err := render.NewRenderer(80)
		if err != nil {
			t.Fatalf("NewRenderer: %v", err)
		}
		out, err := r.Render(md)
		if err != nil {
			t.Errorf("state=%d: glamour.Render failed: %v\n%s", s, err, md)
			continue
		}
		if out == "" {
			t.Errorf("state=%d: rendered output is empty", s)
		}
	}
}

// stripANSIForTest removes ANSI escape codes from s for plain-text
// assertions in tests (the production visibleWidth counts runes
// but here we want a substring search against the actual text).
func stripANSIForTest(s string) string {
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
