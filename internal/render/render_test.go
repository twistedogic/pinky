package render

import (
	"strings"
	"testing"
)

func TestBuildBlockIndex_MultiBlockMessage(t *testing.T) {
	md := `# Heading One

A short paragraph.

` + "```go\nfunc main() {}\n```"
	_, blocks := renderBlocks(md, 80)
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
	_, blocks := renderBlocks(md, 80)
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
	_, blocks := renderBlocks(md, 80)
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
	_, blocks := renderBlocks(md, 80)
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
	_, blocks := renderBlocks(md, 80)
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
	_, b := renderBlocks(md, width)
	return b
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

// TestBlock_HasComment_NoComments verifies that when no comments are
// passed, every block has HasComment=false (the zero value).
func TestBlock_HasComment_NoComments(t *testing.T) {
	md := `# A

para one

## B

para two`
	_, blocks := RenderMessageWithComments(md, 80, nil)
	if len(blocks) == 0 {
		t.Fatal("expected blocks")
	}
	for i, b := range blocks {
		if b.HasComment {
			t.Errorf("block %d HasComment=true with no comments", i)
		}
	}
}

// TestBlock_HasComment_OneCommentOnBlockN verifies that a single
// block-level comment on block N flips blocks[N].HasComment=true and
// leaves the others false.
func TestBlock_HasComment_OneCommentOnBlockN(t *testing.T) {
	md := `# A

para one

## B

para two`
	_, blocks := RenderMessageWithComments(md, 80, []Comment{
		{BlockIdx: 1, CharStart: -1, CharEnd: -1, Text: "x"},
	})
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(blocks))
	}
	for i, b := range blocks {
		want := i == 1
		if b.HasComment != want {
			t.Errorf("block %d HasComment=%v want %v", i, b.HasComment, want)
		}
	}
}

// _ = strings keeps the import until/unless more tests need it.
var _ = strings.TrimSpace

// TestRender_TableCellsAppearInOutput: a GFM table in the input
// must reach the rendered output. Regression test for the silent
// drop where extract() had no case for *ast.Table; without the
// fix, no cell text and no pipe characters appear anywhere.
func TestRender_TableCellsAppearInOutput(t *testing.T) {
	md := `# heading

| col1 | col2 |
|------|------|
| a    | b    |
| c    | d    |

trailing paragraph
`
	rendered, blocks := renderBlocks(md, 80)
	plain := stripANSI(rendered)
	for _, want := range []string{"col1", "col2", "a", "b", "c", "d"} {
		if !strings.Contains(plain, want) {
			t.Errorf("table cell %q missing from rendered output:\n%s", want, plain)
		}
	}
	// Sanity: the surrounding blocks still produced.
	if len(blocks) < 3 {
		t.Errorf("expected >=3 blocks (heading, table, paragraph), got %d", len(blocks))
	}
}

// TestExtract_TableRecognized: a GFM table parses to exactly one
// block with Kind == BlockTable. Guards the AST-handler addition.
func TestExtract_TableRecognized(t *testing.T) {
	md := `| col1 | col2 |
|------|------|
| a    | b    |
`
	_, blocks := renderBlocks(md, 80)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d (%+v)", len(blocks), blocks)
	}
	if blocks[0].Kind != BlockTable {
		t.Errorf("blocks[0].Kind = %s want %s", blocks[0].Kind, BlockTable)
	}
}

// TestRender_TableBetweenBlocks: a table sandwiched between a
// heading and a paragraph produces three blocks with contiguous
// line ranges. Guards the spec scenario "Table sits cleanly
// between adjacent blocks".
func TestRender_TableBetweenBlocks(t *testing.T) {
	md := `# heading

| col1 | col2 |
|------|------|
| a    | b    |

trailing paragraph
`
	_, blocks := renderBlocks(md, 80)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d (%+v)", len(blocks), blocks)
	}
	kinds := []BlockKind{blocks[0].Kind, blocks[1].Kind, blocks[2].Kind}
	wantKinds := []BlockKind{BlockHeading, BlockTable, BlockParagraph}
	for i, k := range kinds {
		if k != wantKinds[i] {
			t.Errorf("block %d Kind = %s want %s", i, k, wantKinds[i])
		}
	}
	for i := 0; i < len(blocks)-1; i++ {
		if blocks[i].EndLine+1 != blocks[i+1].StartLine {
			t.Errorf("blocks[%d] ends at %d, blocks[%d] starts at %d (not contiguous)",
				i, blocks[i].EndLine, i+1, blocks[i+1].StartLine)
		}
	}
}

// TestExtract_DefinitionListRecognized: a GFM definition list
// parses to exactly one block with Kind == BlockDefList. Same
// root cause as tables (extension.DefinitionList is enabled but
// *ast.DefinitionList was unhandled by extract()).
func TestExtract_DefinitionListRecognized(t *testing.T) {
	md := `Term 1
:   Definition 1

Term 2
:   Definition 2
`
	_, blocks := renderBlocks(md, 80)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d (%+v)", len(blocks), blocks)
	}
	if blocks[0].Kind != BlockDefList {
		t.Errorf("blocks[0].Kind = %s want %s", blocks[0].Kind, BlockDefList)
	}
}