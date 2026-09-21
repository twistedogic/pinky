package render

import (
	"strings"
	"testing"
)

// TestLinkify_AutoLinkBareURL verifies that bare URLs in text get
// turned into link AST nodes by the linkify extension.
func TestLinkify_AutoLinkBareURL(t *testing.T) {
	md := "Visit https://example.com for more."
	rendered, err := NewRenderer(80)
	if err != nil {
		t.Fatal(err)
	}
	out, err := rendered.Render(md)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://example.com") {
		t.Errorf("expected URL to appear in output; got:\n%s", out)
	}
	// The pinky style colors links in pink with underline. Look for
	// the underline attribute (4m) anywhere in the output, which is
	// emitted by glamour for link-styled text after linkify.
	if !strings.Contains(out, "4m") {
		t.Errorf("expected underline ANSI code (link styling); got:\n%s", out)
	}
}
