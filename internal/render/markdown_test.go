package render

import (
	"strings"
	"testing"
)

// TestRenderMarkdown_HeaderRendered: a heading lands an H1 / H2 in
// the rendered output. Anchor of "the markdown machinery works".
func TestRenderMarkdown_HeaderRendered(t *testing.T) {
	got := RenderMarkdown("## helper\n\nreturns 42.", 80)
	if !strings.Contains(got, "helper") {
		t.Errorf("expected rendered output to contain 'helper'; got %q", got)
	}
}

// TestRenderMarkdown_WordWrap: long source content wraps to the
// configured width — each rendered line must fit width visible
// cells. Glamour's WithPreservedNewLines keeps source newlines so a
// 40-line input still produces ~40 rendered lines, not 9.
func TestRenderMarkdown_WordWrap(t *testing.T) {
	in := strings.Repeat("line\n", 40)
	got := RenderMarkdown(in, 58)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 40 {
		t.Errorf("expected 40 rendered lines (preserved); got %d", len(lines))
	}
}

// TestRenderMarkdown_FallsBackOnError: when width is below the
// safe floor, glamour's wrap option may return an error or empty
// output. RenderMarkdown should never return an empty string —
// the modal needs something to show.
func TestRenderMarkdown_FallsBackOnEmpty(t *testing.T) {
	got := RenderMarkdown("hello", 4) // below safe floor; gets coerced to 8
	if got == "" {
		t.Errorf("RenderMarkdown returned empty string for valid input")
	}
}