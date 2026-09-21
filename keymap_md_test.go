package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
)

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
