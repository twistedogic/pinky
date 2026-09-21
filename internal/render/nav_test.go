package render

import "testing"

// navBlocks returns a 3-block index for nav tests. The block source
// is the only thing that matters to NavHandle — the rendered-line
// indices are only used by NavLineIndex.
func navBlocks() []Block {
	return []Block{
		{Kind: BlockHeading, Source: "alpha", StartLine: 0, EndLine: 0},
		{Kind: BlockParagraph, Source: "bravo charlie", StartLine: 2, EndLine: 4},
		{Kind: BlockHeading, Source: "delta", StartLine: 6, EndLine: 6},
	}
}

func TestNav_JMovesBlockCursor(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('j', &st, &cur, &sel, blocks); got != ActionBlockDown {
		t.Errorf("action = %v want ActionBlockDown", got)
	}
	if cur.BlockIdx != 2 {
		t.Errorf("BlockIdx = %d want 2", cur.BlockIdx)
	}
}

func TestNav_KMovesBlockCursor(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('k', &st, &cur, &sel, blocks); got != ActionBlockUp {
		t.Errorf("action = %v want ActionBlockUp", got)
	}
	if cur.BlockIdx != 0 {
		t.Errorf("BlockIdx = %d want 0", cur.BlockIdx)
	}
}

func TestNav_JClampsAtLastBlock(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 2, CharPos: 0}
	var sel NavSelection
	NavHandle('j', &st, &cur, &sel, blocks)
	if cur.BlockIdx != 2 {
		t.Errorf("BlockIdx should clamp at last block; got %d", cur.BlockIdx)
	}
}

func TestNav_KClampsAtFirstBlock(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 0, CharPos: 0}
	var sel NavSelection
	NavHandle('k', &st, &cur, &sel, blocks)
	if cur.BlockIdx != 0 {
		t.Errorf("BlockIdx should clamp at first block; got %d", cur.BlockIdx)
	}
}

func TestNav_HLClampAtByteBoundaries(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 1, CharPos: 0}
	var sel NavSelection
	NavHandle('l', &st, &cur, &sel, blocks)
	if cur.CharPos != 1 {
		t.Errorf("CharPos after l from 0 = %d want 1", cur.CharPos)
	}
	// "bravo charlie" is 13 bytes
	for i := 0; i < 50; i++ {
		NavHandle('l', &st, &cur, &sel, blocks)
	}
	if cur.CharPos != 13 {
		t.Errorf("CharPos should clamp at len(Source); got %d want 13", cur.CharPos)
	}
	NavHandle('h', &st, &cur, &sel, blocks)
	if cur.CharPos != 12 {
		t.Errorf("CharPos after h from 13 = %d want 12", cur.CharPos)
	}
	for i := 0; i < 50; i++ {
		NavHandle('h', &st, &cur, &sel, blocks)
	}
	if cur.CharPos != 0 {
		t.Errorf("CharPos should clamp at 0; got %d", cur.CharPos)
	}
}

func TestNav_VTogglesAndSeedsSelection(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 1, CharPos: 3}
	var sel NavSelection
	if got := NavHandle('v', &st, &cur, &sel, blocks); got != ActionEnterVisual {
		t.Errorf("action = %v want ActionEnterVisual", got)
	}
	if st.Visual != NavLine {
		t.Errorf("Visual = %v want NavLine", st.Visual)
	}
	if sel.CharA != 3 || sel.CharC != 3 {
		t.Errorf("selection = (%d,%d) want (3,3)", sel.CharA, sel.CharC)
	}
	if sel.BlockIdx != 1 {
		t.Errorf("selection.BlockIdx = %d want 1", sel.BlockIdx)
	}
}

func TestNav_VWhileVisualExits(t *testing.T) {
	blocks := navBlocks()
	st := NavState{Visual: NavLine}
	cur := NavCursor{BlockIdx: 1, CharPos: 5}
	sel := NavSelection{BlockIdx: 1, CharA: 2, CharC: 5}
	if got := NavHandle('v', &st, &cur, &sel, blocks); got != ActionExitVisual {
		t.Errorf("action = %v want ActionExitVisual", got)
	}
	if st.Visual != NavNone {
		t.Errorf("Visual should be NavNone after second v")
	}
	if sel != (NavSelection{}) {
		t.Errorf("selection should be cleared after second v; got %+v", sel)
	}
}

func TestNav_EscWhileVisualExits(t *testing.T) {
	blocks := navBlocks()
	st := NavState{Visual: NavLine}
	cur := NavCursor{BlockIdx: 1, CharPos: 5}
	sel := NavSelection{BlockIdx: 1, CharA: 2, CharC: 5}
	if got := NavHandle(0x1b, &st, &cur, &sel, blocks); got != ActionExitVisual {
		t.Errorf("action = %v want ActionExitVisual", got)
	}
	if st.Visual != NavNone {
		t.Errorf("Visual should be NavNone after Esc")
	}
}

func TestNav_EscOutsideVisualIsNoop(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 0, CharPos: 0}
	var sel NavSelection
	if got := NavHandle(0x1b, &st, &cur, &sel, blocks); got != ActionNone {
		t.Errorf("Esc outside visual = %v want ActionNone", got)
	}
}

func TestNav_CReturnsComment(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('c', &st, &cur, &sel, blocks); got != ActionComment {
		t.Errorf("action = %v want ActionComment", got)
	}
}

func TestNav_SQRNReturnActions(t *testing.T) {
	blocks := navBlocks()
	cases := []struct {
		key  rune
		want NavAction
	}{
		{'s', ActionSend},
		{'q', ActionQuit},
		{'r', ActionRefresh},
		{'n', ActionCompose},
		{'?', ActionHelp},
	}
	for _, c := range cases {
		var st NavState
		cur := NavCursor{BlockIdx: 0, CharPos: 0}
		var sel NavSelection
		if got := NavHandle(c.key, &st, &cur, &sel, blocks); got != c.want {
			t.Errorf("key=%q action=%v want %v", c.key, got, c.want)
		}
	}
}

func TestNav_UnknownKeyIsNoop(t *testing.T) {
	blocks := navBlocks()
	var st NavState
	cur := NavCursor{BlockIdx: 1, CharPos: 5}
	var sel NavSelection
	if got := NavHandle('x', &st, &cur, &sel, blocks); got != ActionNone {
		t.Errorf("unknown rune action=%v want ActionNone", got)
	}
	if cur.BlockIdx != 1 || cur.CharPos != 5 {
		t.Errorf("cursor moved on unknown rune: (%d,%d)", cur.BlockIdx, cur.CharPos)
	}
}

func TestNav_HLInVisualExtendsSelection(t *testing.T) {
	blocks := navBlocks()
	st := NavState{Visual: NavLine}
	cur := NavCursor{BlockIdx: 1, CharPos: 2}
	sel := NavSelection{BlockIdx: 1, CharA: 2, CharC: 2}
	NavHandle('l', &st, &cur, &sel, blocks)
	if sel.CharC != 3 {
		t.Errorf("after l in visual: CharC=%d want 3", sel.CharC)
	}
	NavHandle('h', &st, &cur, &sel, blocks)
	if sel.CharC != 2 {
		t.Errorf("after h back in visual: CharC=%d want 2", sel.CharC)
	}
}

func TestNavLineIndex_StartOfBlock(t *testing.T) {
	blocks := navBlocks()
	cur := NavCursor{BlockIdx: 0, CharPos: 0}
	if got := NavLineIndex(blocks, cur); got != 0 {
		t.Errorf("NavLineIndex at block 0 start = %d want 0", got)
	}
}

func TestNavLineIndex_MidBlockAdvancesLine(t *testing.T) {
	blocks := []Block{
		{Kind: BlockParagraph, Source: "line one\nline two\nline three", StartLine: 0, EndLine: 2},
	}
	// "line one\n" is 9 bytes; charPos=9 (start of "line two") lands on line 1.
	cur := NavCursor{BlockIdx: 0, CharPos: 9}
	if got := NavLineIndex(blocks, cur); got != 1 {
		t.Errorf("NavLineIndex at line 2 = %d want 1", got)
	}
}

func TestNavLineIndex_PastEndClampsToEndLine(t *testing.T) {
	blocks := []Block{
		{Kind: BlockParagraph, Source: "alpha\nbravo", StartLine: 0, EndLine: 1},
	}
	cur := NavCursor{BlockIdx: 0, CharPos: 9999}
	if got := NavLineIndex(blocks, cur); got != 1 {
		t.Errorf("NavLineIndex past-end = %d want 1", got)
	}
}

func TestNavLineIndex_OutOfRangeIsZero(t *testing.T) {
	blocks := navBlocks()
	cur := NavCursor{BlockIdx: -1, CharPos: 0}
	if got := NavLineIndex(blocks, cur); got != 0 {
		t.Errorf("NavLineIndex out-of-range = %d want 0", got)
	}
	cur = NavCursor{BlockIdx: 99, CharPos: 0}
	if got := NavLineIndex(blocks, cur); got != 0 {
		t.Errorf("NavLineIndex out-of-range = %d want 0", got)
	}
}
