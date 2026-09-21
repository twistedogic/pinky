package render

import (
	"strings"
	"testing"
)

func TestBuildBlockIndex_MultiBlockMessage(t *testing.T) {
	md := `# Heading One

A short paragraph.

` + "```go\nfunc main() {}\n```"
	blocks := BuildBlockIndex(md, 80)
	if len(blocks) != 3 {
		t.Fatalf("len(blocks)=%d want 3 (got %+v)", len(blocks), blocks)
	}
	wantKinds := []BlockKind{BlockHeading, BlockParagraph, BlockCode}
	for i, b := range blocks {
		if b.Kind != wantKinds[i] {
			t.Errorf("[%d] kind=%s want=%s", i, b.Kind, wantKinds[i])
		}
	}
}

func TestBuildBlockIndex_ListItemsIndividual(t *testing.T) {
	md := `- alpha
- beta
- gamma
- delta
- epsilon`
	blocks := BuildBlockIndex(md, 80)
	if len(blocks) != 5 {
		t.Fatalf("len(blocks)=%d want 5 (got %+v)", len(blocks), blocks)
	}
	for i, b := range blocks {
		if b.Kind != BlockListItem {
			t.Errorf("[%d] kind=%s want=%s", i, b.Kind, BlockListItem)
		}
	}
}

func TestBuildBlockIndex_EmptyParagraphSkipped(t *testing.T) {
	md := `# Title

First paragraph.

Second paragraph.`
	blocks := BuildBlockIndex(md, 80)
	// Expected: heading, paragraph (first), paragraph (second).
	// Empty paragraph between title and first paragraph is dropped.
	if len(blocks) != 3 {
		t.Fatalf("len(blocks)=%d want 3 (got %+v)", len(blocks), blocks)
	}
	if blocks[0].Kind != BlockHeading {
		t.Errorf("blocks[0].Kind=%s want heading", blocks[0].Kind)
	}
	if blocks[1].Kind != BlockParagraph || blocks[2].Kind != BlockParagraph {
		t.Errorf("expected two paragraphs, got %s and %s", blocks[1].Kind, blocks[2].Kind)
	}
}

// Lines are contiguous: block[i].EndLine + 1 == block[i+1].StartLine.
func TestBuildBlockIndex_LinesContiguous(t *testing.T) {
	md := `# A

para one

## B

para two`
	blocks := BuildBlockIndex(md, 80)
	for i := 0; i < len(blocks)-1; i++ {
		if blocks[i].EndLine+1 != blocks[i+1].StartLine {
			t.Errorf("blocks[%d] ends at %d, blocks[%d] starts at %d (not contiguous)",
				i, blocks[i].EndLine, i+1, blocks[i+1].StartLine)
		}
	}
}

func TestBuildBlockIndex_StartAtZero(t *testing.T) {
	md := `first block
second block`
	blocks := BuildBlockIndex(md, 80)
	if len(blocks) == 0 {
		t.Fatal("expected blocks")
	}
	if blocks[0].StartLine != 0 {
		t.Errorf("blocks[0].StartLine=%d want 0", blocks[0].StartLine)
	}
}

// Helpers for navigation tests below.
func mustBuild(t *testing.T, md string, width int) []Block {
	t.Helper()
	return BuildBlockIndex(md, width)
}

func lines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = ""
	}
	return out
}

func TestJumpBlock_Next(t *testing.T) {
	md := `# A

para one

## B

para two`
	blocks := mustBuild(t, md, 80)
	// From line 0 (in block 0, heading A), jumpBlock(+1) → first line of block 1.
	if got := JumpBlock(blocks, 0, 1); got != blocks[1].StartLine {
		t.Errorf("JumpBlock(+1) from 0 = %d want %d", got, blocks[1].StartLine)
	}
}

func TestJumpBlock_Previous(t *testing.T) {
	md := `# A

para one

## B

para two`
	blocks := mustBuild(t, md, 80)
	// From block 2 (heading B), jumpBlock(-1) → first line of block 1.
	if got := JumpBlock(blocks, 2, -1); got != blocks[1].StartLine {
		t.Errorf("JumpBlock(-1) from 2 = %d want %d", got, blocks[1].StartLine)
	}
}

func TestJumpBlock_NoOpAtBoundary(t *testing.T) {
	md := `# A

para one`
	blocks := mustBuild(t, md, 80)
	// JumpBlock(+1) from the last block returns -1.
	if got := JumpBlock(blocks, len(blocks)-1, 1); got != -1 {
		t.Errorf("JumpBlock(+1) from last = %d want -1", got)
	}
	// JumpBlock(-1) from the first block returns -1.
	if got := JumpBlock(blocks, 0, -1); got != -1 {
		t.Errorf("JumpBlock(-1) from 0 = %d want -1", got)
	}
}

func TestJumpHeading_SkipsNonHeadings(t *testing.T) {
	md := `# A

para one

## B

para two`
	blocks := mustBuild(t, md, 80)
	// From block 1 (paragraph), JumpHeading(+1) → block 2 (heading B).
	if got := JumpHeading(blocks, 1, 1); got != blocks[2].StartLine {
		t.Errorf("JumpHeading(+1) from para = %d want %d", got, blocks[2].StartLine)
	}
	// From block 3 (paragraph two), JumpHeading(-1) → block 2 (heading B).
	if got := JumpHeading(blocks, 3, -1); got != blocks[2].StartLine {
		t.Errorf("JumpHeading(-1) from para = %d want %d", got, blocks[2].StartLine)
	}
}

func TestJumpHeading_NoHeadings(t *testing.T) {
	md := `just a paragraph
another line`
	blocks := mustBuild(t, md, 80)
	if got := JumpHeading(blocks, 0, 1); got != -1 {
		t.Errorf("JumpHeading with no headings = %d want -1", got)
	}
}

func TestCurrentBlockIdx(t *testing.T) {
	md := `# A

para one

## B`
	blocks := mustBuild(t, md, 80)
	// YOffset at the first line of a block returns that block.
	for i, b := range blocks {
		if got := CurrentBlockIdx(blocks, b.StartLine); got != i {
			t.Errorf("CurrentBlockIdx at block %d start = %d want %d", i, got, i)
		}
	}
	// YOffset in the middle of a block returns that block.
	mid := blocks[1].StartLine + (blocks[1].EndLine-blocks[1].StartLine)/2
	if got := CurrentBlockIdx(blocks, mid); got != 1 {
		t.Errorf("CurrentBlockIdx mid-block 1 = %d want 1", got)
	}
	// YOffset before the first block's start returns -1.
	if got := CurrentBlockIdx(blocks, -1); got != -1 {
		t.Errorf("CurrentBlockIdx -1 = %d want -1", got)
	}
	// YOffset past the last block returns -1.
	if got := CurrentBlockIdx(blocks, 9999); got != -1 {
		t.Errorf("CurrentBlockIdx past end = %d want -1", got)
	}
}

// Suppress unused warnings for test helpers.
var _ = lines
var _ = strings.TrimSpace
