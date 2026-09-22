package render

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFormatCommentsAppendix_EmptyReturnsEmpty(t *testing.T) {
	got := FormatCommentsAppendix(nil, nil)
	if got != "" {
		t.Errorf("empty comments should give empty string; got %q", got)
	}
}

func TestFormatCommentsAppendix_SingleBlock(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, Source: "first line\nsecond line", StartLine: 0, EndLine: 2}}
	comments := []Comment{{BlockIdx: 0, CharStart: -1, Text: "looks good", CreatedAt: time.Now()}}
	got := FormatCommentsAppendix(comments, blocks)
	for _, want := range []string{"---", "1 comment:", "block", "looks good"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in appendix; got %q", want, got)
		}
	}
	if strings.Contains(got, "1 comments:") {
		t.Errorf("singular label must not be plural; got %q", got)
	}
}

// TestFormatCommentsAppendix_PluralLabel locks in the plural form
// (regression guard for the singular fix above).
func TestFormatCommentsAppendix_PluralLabel(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, Source: "x", StartLine: 0, EndLine: 1}}
	comments := []Comment{
		{BlockIdx: 0, CharStart: -1, Text: "a", CreatedAt: time.Now()},
		{BlockIdx: 0, CharStart: -1, Text: "b", CreatedAt: time.Now().Add(time.Second)},
	}
	got := FormatCommentsAppendix(comments, blocks)
	if !strings.Contains(got, "2 comments:") {
		t.Errorf("plural: want '2 comments:' in output; got:\n%s", got)
	}
}

// TestFormatCommentsAppendix_ExcerptFlattensNewlines locks in the
// one-line-per-comment contract: a multi-line block source must not
// leak raw newlines into the excerpt, which would break the format
// (closing `"` on the wrong line).
func TestFormatCommentsAppendix_ExcerptFlattensNewlines(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, Source: "first line\nsecond line\nthird line", StartLine: 0, EndLine: 4}}
	comments := []Comment{{BlockIdx: 0, CharStart: -1, Text: "x", CreatedAt: time.Now()}}
	got := FormatCommentsAppendix(comments, blocks)
	// Header "\n\n---\n1 comment:\n" (4 newlines) + 1 comment line + trailing "\n"
	// → 5 newlines. A raw-newline excerpt would push this higher.
	if c := strings.Count(got, "\n"); c != 5 {
		t.Errorf("expected 5 newlines (one comment line); got %d:\n%q", c, got)
	}
	// Find the comment line and verify it's single-line with all
	// source lines flattened.
	var commentLine string
	for _, l := range strings.Split(got, "\n") {
		if strings.HasPrefix(l, "- block") {
			commentLine = l
			break
		}
	}
	if commentLine == "" {
		t.Fatalf("no - block line in output:\n%s", got)
	}
	for _, want := range []string{"first line", "second line", "third line"} {
		if !strings.Contains(commentLine, want) {
			t.Errorf("comment line missing flattened %q; got: %q", want, commentLine)
		}
	}
}

func TestFormatCommentsAppendix_SingleInline(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, Source: "first line", StartLine: 0, EndLine: 1}}
	comments := []Comment{{BlockIdx: 0, CharStart: 0, CharEnd: 5, Source: "first", Text: "rename", CreatedAt: time.Now()}}
	got := FormatCommentsAppendix(comments, blocks)
	for _, want := range []string{"inline", "first", "rename"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in appendix; got %q", want, got)
		}
	}
}

func TestFormatCommentsAppendix_ChronologicalOrder(t *testing.T) {
	now := time.Now()
	blocks := []Block{
		{Kind: BlockParagraph, Source: "alpha", StartLine: 0, EndLine: 1},
		{Kind: BlockParagraph, Source: "beta", StartLine: 3, EndLine: 4},
	}
	comments := []Comment{
		{BlockIdx: 1, Source: "beta", Text: "second", CreatedAt: now.Add(time.Second)},
		{BlockIdx: 0, Source: "alpha", Text: "first", CreatedAt: now},
	}
	got := FormatCommentsAppendix(comments, blocks)
	firstIdx := strings.Index(got, "first")
	secondIdx := strings.Index(got, "second")
	if firstIdx == -1 || secondIdx == -1 || firstIdx > secondIdx {
		t.Errorf("expected chronological order; first=%d second=%d\n%s", firstIdx, secondIdx, got)
	}
}

func TestFormatCommentsAppendix_ExcerptTruncation(t *testing.T) {
	longSrc := "this is a very long source line that exceeds forty characters in length"
	blocks := []Block{{Kind: BlockParagraph, Source: longSrc, StartLine: 0, EndLine: 1}}
	// block-level → excerpt comes from blocks[i].Source, not c.Source.
	comments := []Comment{{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x", CreatedAt: time.Now()}}
	got := FormatCommentsAppendix(comments, blocks)
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis for long excerpt; got %q", got)
	}
}

func TestItoa(t *testing.T) {
	tests := map[int]string{0: "0", 1: "1", -1: "-1", 42: "42", 12345: "12345"}
	for n, want := range tests {
		if got := strconv.Itoa(n); got != want {
			t.Errorf("strconv.Itoa(%d) = %q want %q", n, got, want)
		}
	}
}
