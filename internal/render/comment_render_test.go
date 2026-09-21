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
	withComments, _ := RenderMessageWithComments(md, 80, nil)
	plain, _ := RenderMessage(md, 80)
	if withComments != plain {
		t.Errorf("zero-comment render should equal plain render\n  with: %q\n  plain: %q", withComments, plain)
	}
}

func TestRenderMessageWithComments_FootnoteBelowBlock(t *testing.T) {
	md := "# Title\n\nbody paragraph.\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "looks good", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, 80, comments)
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
	out, _ := RenderMessageWithComments(md, 80, comments)
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

func TestRenderMessageWithComments_GutterMarkerOnFirstLine(t *testing.T) {
	md := "# Title\n\nbody.\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, 80, comments)
	// The gutter marker ▸ should appear on the FIRST line of the
	// heading block (block 0, StartLine=0).
	lines := strings.Split(stripANSIForRender(out), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "▸") {
		t.Errorf("expected gutter marker on first line of block; first line was: %q", lines[0])
	}
}

func TestRenderMessageWithComments_BackgroundTintApplied(t *testing.T) {
	md := "first line\nsecond line\nthird line\n"
	// Comment on block 0 → tint should apply to all block lines.
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x", CreatedAt: mustTime(t)},
	}
	out, _ := RenderMessageWithComments(md, 80, comments)
	// The tint is a lipgloss background — look for the bg ANSI code
	// (48;5;Nm) which lipgloss emits when Background is set.
	if !strings.Contains(out, "48;5;") && !strings.Contains(out, "48;2;") {
		t.Errorf("expected background-color ANSI in output; got:\n%s", out)
	}
}

func TestRenderMessageWithComments_MultipleCommentsStack(t *testing.T) {
	md := "alpha\n"
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "first", CreatedAt: mustTime(t)},
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "second", CreatedAt: mustTime(t).Add(time.Second)},
	}
	out, _ := RenderMessageWithComments(md, 80, comments)
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

// stripANSIForRender removes ANSI escape codes from s.
func stripANSIForRender(s string) string {
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
