package render

import (
	"strings"
	"testing"
	"time"
)

// TestRenderFile_LineNumbersAndGutter covers the right-aligned line
// number column, the per-line gutter, and the yellow highlight on
// commented lines.
func TestRenderFile_LineNumbersAndGutter(t *testing.T) {
	content := "alpha\nbeta\ngamma\ndelta\n"
	comments := []Comment{
		{Kind: CommentFile, Path: "x", LineStart: 2, LineEnd: 2, CharStart: -1,
			Text: "rename", CreatedAt: time.Now()},
	}
	rendered, lines := RenderFile(content, comments)

	// 4 source lines → width 1 (max single digit).
	if len(lines) != 4 {
		t.Fatalf("expected 4 FileLine entries; got %d", len(lines))
	}
	// Line 2 carries the comment; the others do not.
	for _, l := range lines {
		want := l.LineNo == 2
		if l.HasComment != want {
			t.Errorf("line %d: HasComment = %v, want %v", l.LineNo, l.HasComment, want)
		}
	}

	// Right-aligned line numbers + gutter + content.
	wantRows := []string{
		"  1  alpha",
		"\x1b[38;5;228m▍\x1b[0m 2  beta",
		"  3  gamma",
		"  4  delta",
	}
	for _, want := range wantRows {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing rendered row %q in:\n%s", want, rendered)
		}
	}
}

// TestRenderFile_IgnoresBlockKindComments locks in the rule that
// block-kind comments don't bleed into the file viewer gutter.
func TestRenderFile_IgnoresBlockKindComments(t *testing.T) {
	content := "one\ntwo\n"
	comments := []Comment{
		{Kind: CommentBlock, BlockIdx: 0, CharStart: -1, Text: "x",
			CreatedAt: time.Now()},
	}
	_, lines := RenderFile(content, comments)
	for _, l := range lines {
		if l.HasComment {
			t.Errorf("block-kind comment marked file line %d", l.LineNo)
		}
	}
}

// TestRenderFile_NoComments covers the no-comment case: every
// gutter is a space, every line number is present.
func TestRenderFile_NoComments(t *testing.T) {
	content := "a\nb\nc\n"
	rendered, lines := RenderFile(content, nil)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines; got %d", len(lines))
	}
	for _, l := range lines {
		if l.HasComment {
			t.Errorf("line %d unexpectedly marked", l.LineNo)
		}
	}
	for _, want := range []string{"1  a", "2  b", "3  c"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q in:\n%s", want, rendered)
		}
	}
}

// TestRenderFile_LineNumberWidth covers the right-alignment width
// when line count crosses a digit boundary.
func TestRenderFile_LineNumberWidth(t *testing.T) {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "x"
	}
	content := strings.Join(lines, "\n") + "\n"
	rendered, _ := RenderFile(content, nil)
	// Line 12 should be right-aligned to width 2 ("12  x").
	if !strings.Contains(rendered, "12  x") {
		t.Errorf("line 12 not right-aligned in:\n%s", rendered)
	}
	// Line 1 should also be width 2 (" 1  x").
	if !strings.Contains(rendered, " 1  x") {
		t.Errorf("line 1 not padded in:\n%s", rendered)
	}
}
