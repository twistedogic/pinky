package render

import (
	"strings"
	"testing"
)

func TestRenderedLineRange_BlockLevel(t *testing.T) {
	md := "# A\n\npara one\n\n## B\n\npara two"
	_, blocks := renderBlocks(md, 80)
	start, end := RenderedLineRange(blocks, 1, -1, -1)
	if start != blocks[1].StartLine || end != blocks[1].EndLine {
		t.Errorf("block-level projection should equal block range; got [%d,%d] want [%d,%d]",
			start, end, blocks[1].StartLine, blocks[1].EndLine)
	}
}

func TestRenderedLineRange_InlineWithinBlock(t *testing.T) {
	md := "first line\nsecond line\nthird line\n"
	_, blocks := renderBlocks(md, 80)
	// Source "second line" starts around byte offset 11 (after "first line\n").
	src := []byte(md)
	idx := strings.Index(string(src), "second line")
	if idx < 0 {
		t.Fatal("setup: could not find 'second line' in source")
	}
	start, end := RenderedLineRange(blocks, 0, idx, idx+len("second line"))
	if start < blocks[0].StartLine || start > blocks[0].EndLine {
		t.Errorf("start=%d outside block range [%d,%d]", start, blocks[0].StartLine, blocks[0].EndLine)
	}
	if end < start {
		t.Errorf("end=%d should be >= start=%d", end, start)
	}
}

func TestRenderedLineRange_OneLineDriftOnWrap(t *testing.T) {
	// A long paragraph that wraps to multiple rendered lines.
	md := "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua"
	_, blocks := renderBlocks(md, 40) // narrow width → forces wrapping
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	b := blocks[0]
	// Map the byte offset of the middle of the source onto the block's
	// rendered line range — exact projection is N/A due to wrap drift,
	// so we only assert the result is within the block.
	mid := len(b.Source) / 2
	start, end := RenderedLineRange(blocks, 0, mid, mid+5)
	if start < b.StartLine || end > b.EndLine {
		t.Errorf("projection out of range [%d,%d] for block [%d,%d]",
			start, end, b.StartLine, b.EndLine)
	}
}

func TestRenderedLineRange_OffTheEndClamps(t *testing.T) {
	md := "alpha\nbravo\ncharlie\n"
	_, blocks := renderBlocks(md, 80)
	// offset >> len(Source) → should clamp to last line of block.
	start, end := RenderedLineRange(blocks, 0, 9999, 99999)
	if end != blocks[0].EndLine {
		t.Errorf("off-the-end should clamp to block's EndLine; got %d want %d",
			end, blocks[0].EndLine)
	}
	_ = start
}