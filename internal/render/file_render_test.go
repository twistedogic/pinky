package render

import (
	"testing"
	"time"
)

// TestMarkLines_FileKindCommentsCovered: file-kind line-range
// comments flip HasComment on every line in LineStart..End.
func TestMarkLines_FileKindCommentsCovered(t *testing.T) {
	content := "alpha\nbeta\ngamma\ndelta\n"
	comments := []Comment{
		{Kind: CommentFile, Path: "x", LineStart: 2, LineEnd: 3,
			CharStart: -1, Text: "rename", CreatedAt: time.Now()},
	}
	lines := MarkLines(content, comments)
	if len(lines) != 4 {
		t.Fatalf("expected 4 FileLine entries; got %d", len(lines))
	}
	for _, l := range lines {
		want := l.LineNo == 2 || l.LineNo == 3
		if l.HasComment != want {
			t.Errorf("line %d: HasComment = %v, want %v", l.LineNo, l.HasComment, want)
		}
	}
}

// TestMarkLines_IgnoresBlockKindComments locks in the rule that
// block-kind comments don't bleed into file line coverage.
func TestMarkLines_IgnoresBlockKindComments(t *testing.T) {
	content := "one\ntwo\n"
	comments := []Comment{
		{Kind: CommentBlock, BlockIdx: 0, CharStart: -1, Text: "x",
			CreatedAt: time.Now()},
	}
	lines := MarkLines(content, comments)
	for _, l := range lines {
		if l.HasComment {
			t.Errorf("block-kind comment marked file line %d", l.LineNo)
		}
	}
}

// TestMarkLines_NoComments covers the no-comment case.
func TestMarkLines_NoComments(t *testing.T) {
	content := "a\nb\nc\n"
	lines := MarkLines(content, nil)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines; got %d", len(lines))
	}
	for _, l := range lines {
		if l.HasComment {
			t.Errorf("line %d unexpectedly marked", l.LineNo)
		}
	}
}

// TestMarkLines_TrimsTrailingEmpty: a trailing newline drops the
// final empty line.
func TestMarkLines_TrimsTrailingEmpty(t *testing.T) {
	content := "x\ny\nz\n"
	lines := MarkLines(content, nil)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines; got %d", len(lines))
	}
}
