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
	for _, want := range []string{"---", "1 comments:", "block", "looks good"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in appendix; got %q", want, got)
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
