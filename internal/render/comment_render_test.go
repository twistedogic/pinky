package render

import (
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func TestRenderMessageWithComments_NoCommentsMatchesPlain(t *testing.T) {
	md := "# Title\n\nbody paragraph.\n"
	withComments, _ := RenderMessageWithComments(md, nil)
	plain, _ := renderBlocks(md)
	if withComments != plain {
		t.Errorf("zero-comment render should equal plain render\n  with: %q\n  plain: %q", withComments, plain)
	}
}

func TestRenderMessageWithComments_FootnoteBelowBlock(t *testing.T) {
	md := "# Title\n\nbody paragraph.\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "looks good", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, comments)
	plain := stripANSIForRender(out)
	if !strings.Contains(plain, "▸") {
		t.Errorf("expected block-level marker ▸ in output:\n%s", plain)
	}
	if !strings.Contains(plain, "looks good") {
		t.Errorf("expected footnote text in output:\n%s", plain)
	}
	// Footnote should appear AFTER the heading line.
	headingIdx := strings.Index(plain, "Title")
	footnoteIdx := strings.Index(plain, "looks good")
	if footnoteIdx <= headingIdx {
		t.Errorf("footnote should appear after block; got headingIdx=%d footnoteIdx=%d", headingIdx, footnoteIdx)
	}
}

func TestRenderMessageWithComments_InlineFootnoteIncludesExcerpt(t *testing.T) {
	md := "alpha\nbravo charlie\ndelta\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: 6, CharEnd: 19, Source: "bravo charlie", Text: "rename", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, comments)
	plain := stripANSIForRender(out)
	if !strings.Contains(plain, "•") {
		t.Errorf("expected inline marker • in output:\n%s", plain)
	}
	if !strings.Contains(plain, "rename") {
		t.Errorf("expected footnote text in output:\n%s", plain)
	}
	if !strings.Contains(plain, "bravo charlie") {
		t.Errorf("expected excerpt in output:\n%s", plain)
	}
}

func TestRenderMessageWithComments_NoInBlockGutterMarker(t *testing.T) {
	md := "# Title\n\nbody.\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, comments)
	plain := stripANSIForRender(out)
	// The block-level marker ▸ must appear ONLY in the footnote line
	// below the block, not prepended to the first line of the block
	// itself. The yellow left gutter (added by the model layer)
	// replaces the in-block marker.
	lines := strings.Split(plain, "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if strings.HasPrefix(lines[0], "▸") {
		t.Errorf("first line of block should NOT start with ▸ (in-block marker removed); got %q", lines[0])
	}
	// And the footnote line below the block still carries the marker.
	if !strings.Contains(plain, "▸") {
		t.Errorf("expected ▸ in footnote line; got:\n%s", plain)
	}
}

func TestRenderMessageWithComments_BackgroundTintApplied(t *testing.T) {
	md := "first line\nsecond line\nthird line\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, comments)
	// Background tinting was removed when glamour was dropped. The
	// raw output must not contain any background-color ANSI code,
	// since adding color to plain text would be visual noise.
	for _, seq := range []string{"48;5;", "48;2;"} {
		if strings.Contains(out, seq) {
			t.Errorf("raw rendering must not emit background-color ANSI %q; got:\n%s", seq, out)
		}
	}
}

func TestRenderMessageWithComments_MultipleCommentsStack(t *testing.T) {
	md := "alpha\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "first", CreatedAt: mustTime(t)},
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "second", CreatedAt: mustTime(t).Add(time.Second)},
	}
	out, _ := RenderMessageWithComments(md, comments)
	plain := stripANSIForRender(out)
	if !strings.Contains(plain, "first") || !strings.Contains(plain, "second") {
		t.Errorf("expected both footnotes in output:\n%s", plain)
	}
	// Both should appear after the block content.
	firstIdx := strings.Index(plain, "first")
	secondIdx := strings.Index(plain, "second")
	if firstIdx == -1 || secondIdx == -1 || firstIdx > secondIdx {
		t.Errorf("footnote ordering broken; first=%d second=%d", firstIdx, secondIdx)
	}
}

func TestMarker(t *testing.T) {
	block := Comment{CharStart: -1}
	if block.Marker() != "▸" {
		t.Errorf("block marker = %q want ▸", block.Marker())
	}
	inline := Comment{CharStart: 5}
	if inline.Marker() != "•" {
		t.Errorf("inline marker = %q want •", inline.Marker())
	}
}

// stripANSIForRender keeps the historical name; the implementation
// lives in pipeline_test.go (package-shared). ponytail: alias kept so
// the call sites read cleanly; remove if/when all callers migrate.
func stripANSIForRender(s string) string { return stripANSI(s) }
