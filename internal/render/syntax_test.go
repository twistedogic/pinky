package render

import (
	"regexp"
	"testing"
)

// TestSyntaxHighlighting_Applied verifies that fenced code blocks
// with a language hint get distinct ANSI color codes for keywords
// vs. strings vs. plain text. Regression guard for the chroma
// wiring in NewRenderer.
func TestSyntaxHighlighting_Applied(t *testing.T) {
	md := "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```"
	rendered, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := rendered.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("rendered output is empty")
	}

	// Collect distinct ANSI color codes used in the code line area.
	// With chroma highlighting, we expect at least 2 distinct
	// foreground codes (one for the keyword, one for the string).
	codes := distinctANSIFG(out)
	if len(codes) < 2 {
		t.Errorf("expected >= 2 distinct ANSI foreground colors for syntax highlighting; got %d in:\n%s",
			len(codes), out)
	}
}

// distinctANSIFG returns the set of unique 38;5;Nm foreground color
// codes used in s. Helps verify chroma emitted more than one color.
func distinctANSIFG(s string) []string {
	re := regexp.MustCompile(`38;5;\d+`)
	matches := re.FindAllString(s, -1)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}
