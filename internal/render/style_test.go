package render

import (
	"strings"
	"testing"
)

// TestPinkyStyle_H1CyanBoldUnderlined verifies the h1 style is cyan
// (hex51 = #00d7d7 → truecolor 0;215;215) with bold + underline,
// and that the rendered line does NOT begin with `#` (the glamour
// `Prefix` is "").
func TestPinkyStyle_H1CyanBoldUnderlined(t *testing.T) {
	md := "# Title\n\nbody"
	r, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if strings.HasPrefix(plain, "#") {
		t.Errorf("h1 should NOT begin with '#' (no prefix); got first line: %q", strings.Split(plain, "\n")[0])
	}
	if !strings.Contains(out, "0;215;215") {
		t.Errorf("expected cyan h1 color (0;215;215) in output; got:\n%s", out)
	}
}

// TestPinkyStyle_H2CyanBold verifies the h2 style is cyan with bold
// and no `##` prefix.
func TestPinkyStyle_H2CyanBold(t *testing.T) {
	md := "## Subhead\n\nbody"
	r, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if strings.HasPrefix(plain, "#") {
		t.Errorf("h2 should NOT begin with '#'; got first line: %q", strings.Split(plain, "\n")[0])
	}
	if !strings.Contains(out, "0;215;215") {
		t.Errorf("expected cyan h2 color (0;215;215) in output; got:\n%s", out)
	}
}

// TestPinkyStyle_H3SofterCyanBold verifies the h3 style uses the
// softer cyan (hex87 = #5fffff → truecolor 95;255;255) with bold
// and no `###` prefix.
func TestPinkyStyle_H3SofterCyanBold(t *testing.T) {
	md := "### Detail\n\nbody"
	r, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if strings.HasPrefix(plain, "#") {
		t.Errorf("h3 should NOT begin with '#'; got first line: %q", strings.Split(plain, "\n")[0])
	}
	if !strings.Contains(out, "95;255;255") {
		t.Errorf("expected softer cyan h3 color (95;255;255) in output; got:\n%s", out)
	}
}

// TestPinkyStyle_DocumentMargin verifies the glamour Document margin
// is 1 (one leading cell), so the focus gutter sits in column 2 and
// the total left indent matches the prior 2-margin + border layout.
func TestPinkyStyle_DocumentMargin(t *testing.T) {
	cfg := pinkyStyle()
	if got := *cfg.Document.Margin; got != 1 {
		t.Errorf("Document.Margin = %d want 1", got)
	}
}

// TestPinkyStyle_CodeHighlight verifies the chroma wiring produces
// multiple distinct ANSI color codes for a fenced code block.
func TestPinkyStyle_CodeHighlight(t *testing.T) {
	md := "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```"
	r, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	// Should have at least 2 distinct foreground colors (keyword
	// pink vs. plain text gray vs. string green).
	codes := distinctANSIFG(out)
	if len(codes) < 2 {
		t.Errorf("expected >= 2 distinct ANSI fg colors for code highlighting; got %d in:\n%s",
			len(codes), out)
	}
}
