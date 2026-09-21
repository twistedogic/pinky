package render

import (
	"strings"
	"testing"
)

// TestPinkyStyle_HeadingsPink verifies the pinky style applies the
// 212 accent color to non-h1 headings. hex212 = "#d75fd7" → truecolor
// "rgb(215,95,215)" emitted as "\x1b[38;2;215;95;215m". (h1 uses a
// distinct yellow color and is covered by TestPinkyStyle_H1Yellow.)
func TestPinkyStyle_HeadingsPink(t *testing.T) {
	md := "## Subhead\n\nbody"
	r, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "215;95;215") {
		t.Errorf("expected pink heading color (215;95;215) in output; got:\n%s", out)
	}
}

// TestPinkyStyle_H1Yellow verifies the pinky style gives h1 a
// distinct yellow (hex228 = "#ffdf00") so the top-level heading
// stands out from subheadings.
func TestPinkyStyle_H1Yellow(t *testing.T) {
	md := "# Title\n\nbody"
	r, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "255;223;0") {
		t.Errorf("expected yellow h1 color (255;223;0) in output; got:\n%s", out)
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
