package render

import "testing"

func visualBlocks() []Block {
	return []Block{
		{Kind: BlockHeading, Source: "A", StartLine: 0, EndLine: 0},
		{Kind: BlockParagraph, Source: "B", StartLine: 2, EndLine: 3},
		{Kind: BlockHeading, Source: "C", StartLine: 5, EndLine: 5},
	}
}

func TestVisual_VEntersModeAndParksCursor(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	if v.Mode != SelLine {
		t.Errorf("mode = %v want SelLine", v.Mode)
	}
	if v.Anchor != 2 || v.Cursor != 2 {
		t.Errorf("anchor/cursor = (%d,%d) want (2,2)", v.Anchor, v.Cursor)
	}
	// Drive the V action through Handle to confirm it also enters.
	var v2 VisualState
	v2.CurBlock = 1
	if got := v2.Handle('V', 10, blocks); got != VisualEnter {
		t.Errorf("V should VisualEnter; got %v", got)
	}
	if v2.Mode != SelLine {
		t.Errorf("mode after V = %v want SelLine", v2.Mode)
	}
}

func TestVisual_JKExtendsCursor(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	v.Handle('j', 10, blocks)
	if v.Cursor != 3 {
		t.Errorf("after j: cursor=%d want 3", v.Cursor)
	}
	v.Handle('k', 10, blocks)
	if v.Cursor != 2 {
		t.Errorf("after k: cursor=%d want 2", v.Cursor)
	}
}

func TestVisual_BraceJumpByBlock(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	if got := v.Handle('}', 10, blocks); got != VisualNextBlock {
		t.Errorf("} action=%v want VisualNextBlock", got)
	}
	if v.Cursor != 5 {
		t.Errorf("after }: cursor=%d want 5 (block 2 start)", v.Cursor)
	}
	if got := v.Handle('{', 10, blocks); got != VisualPrevBlock {
		t.Errorf("{ action=%v want VisualPrevBlock", got)
	}
	if v.Cursor != 2 {
		t.Errorf("after {: cursor=%d want 2 (back to block 1)", v.Cursor)
	}
}

func TestVisual_EscExits(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	if got := v.Handle(0x1b, 10, blocks); got != VisualExit {
		t.Errorf("Esc action=%v want VisualExit", got)
	}
	if v.Mode != SelNone {
		t.Errorf("after Esc: mode=%v want SelNone", v.Mode)
	}
}

func TestVisual_CReturnsComposer(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	if got := v.Handle('c', 10, blocks); got != VisualComposer {
		t.Errorf("c action=%v want VisualComposer", got)
	}
}

func TestVisual_JClampsAtTotalLines(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	for i := 0; i < 10; i++ {
		v.Handle('j', 7, blocks)
	}
	if v.Cursor != 6 {
		t.Errorf("cursor should clamp at totalLines-1; got %d", v.Cursor)
	}
}

func TestVisual_KClampsAtZero(t *testing.T) {
	blocks := visualBlocks()
	var v VisualState
	v.Enter(blocks[1].StartLine, 1, blocks)
	for i := 0; i < 5; i++ {
		v.Handle('k', 10, blocks)
	}
	if v.Cursor != 0 {
		t.Errorf("cursor should clamp at 0; got %d", v.Cursor)
	}
}
