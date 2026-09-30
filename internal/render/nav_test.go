package render

import "testing"

// navLines returns a small source-line slice and its byte-offset
// table for nav tests. The byte offsets follow the "+\n" rule: each
// line occupies len(line)+1 bytes in the concatenated text (the +1
// being the trailing newline).
func navLines() ([]string, []int) {
	lines := []string{"alpha", "bravo charlie", "delta"}
	offsets := []int{0, 6, 20}
	return lines, offsets
}

func TestNav_JMovesLineCursor(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('j', &st, &cur, &sel, lines, offsets); got != ActionBlockDown {
		t.Errorf("action = %v want ActionBlockDown", got)
	}
	if cur.LineIdx != 2 {
		t.Errorf("LineIdx = %d want 2", cur.LineIdx)
	}
}

func TestNav_KMovesLineCursor(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('k', &st, &cur, &sel, lines, offsets); got != ActionBlockUp {
		t.Errorf("action = %v want ActionBlockUp", got)
	}
	if cur.LineIdx != 0 {
		t.Errorf("LineIdx = %d want 0", cur.LineIdx)
	}
}

func TestNav_JClampsAtLastLine(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 2, CharPos: 0}
	var sel NavSelection
	NavHandle('j', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 2 {
		t.Errorf("LineIdx should clamp at last line; got %d", cur.LineIdx)
	}
}

func TestNav_KClampsAtFirstLine(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 0, CharPos: 0}
	var sel NavSelection
	NavHandle('k', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 0 {
		t.Errorf("LineIdx should clamp at first line; got %d", cur.LineIdx)
	}
}

func TestNav_HLClampAtByteBoundaries(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	NavHandle('l', &st, &cur, &sel, lines, offsets)
	if cur.CharPos != 1 {
		t.Errorf("CharPos after l from 0 = %d want 1", cur.CharPos)
	}
	// "bravo charlie" is 13 bytes
	for range 50 {
		NavHandle('l', &st, &cur, &sel, lines, offsets)
	}
	if cur.CharPos != 13 {
		t.Errorf("CharPos should clamp at len(line); got %d want 13", cur.CharPos)
	}
	NavHandle('h', &st, &cur, &sel, lines, offsets)
	if cur.CharPos != 12 {
		t.Errorf("CharPos after h from 13 = %d want 12", cur.CharPos)
	}
	for range 50 {
		NavHandle('h', &st, &cur, &sel, lines, offsets)
	}
	if cur.CharPos != 0 {
		t.Errorf("CharPos should clamp at 0; got %d", cur.CharPos)
	}
}

func TestNav_VTogglesAndSeedsSelection(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 3}
	var sel NavSelection
	if got := NavHandle('v', &st, &cur, &sel, lines, offsets); got != ActionEnterVisual {
		t.Errorf("action = %v want ActionEnterVisual", got)
	}
	if st.Visual != NavLine {
		t.Errorf("Visual = %v want NavLine", st.Visual)
	}
	wantA := byteOffset(offsets, 1, 3)
	if sel.ByteA != wantA || sel.ByteC != wantA {
		t.Errorf("selection byte = (%d, %d) want (%d, %d)", sel.ByteA, sel.ByteC, wantA, wantA)
	}
	if got := NavHandle('v', &st, &cur, &sel, lines, offsets); got != ActionExitVisual {
		t.Errorf("second v action = %v want ActionExitVisual", got)
	}
	if sel != (NavSelection{}) {
		t.Errorf("selection should clear on visual exit; got %+v", sel)
	}
}

func TestNav_JInVisualUpdatesByteC(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 0, CharPos: 0}
	var sel NavSelection
	NavHandle('v', &st, &cur, &sel, lines, offsets)
	cur.CharPos = 2
	NavHandle('l', &st, &cur, &sel, lines, offsets)
	selBefore := sel.ByteC
	NavHandle('j', &st, &cur, &sel, lines, offsets)
	if sel.ByteC == selBefore {
		t.Errorf("j in visual should change ByteC; got %d (was %d)", sel.ByteC, selBefore)
	}
}

func TestNav_EscExitsVisual(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	NavHandle('v', &st, &cur, &sel, lines, offsets)
	if got := NavHandle(0x1b, &st, &cur, &sel, lines, offsets); got != ActionExitVisual {
		t.Errorf("Esc action = %v want ActionExitVisual", got)
	}
	if st.Visual != NavNone {
		t.Errorf("Visual = %v want NavNone", st.Visual)
	}
}

func TestNav_EscOutsideVisualIsNoOp(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle(0x1b, &st, &cur, &sel, lines, offsets); got != ActionNone {
		t.Errorf("Esc outside visual action = %v want ActionNone", got)
	}
}

func TestNav_PreferredColumnOnShortLine(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	// Move to long line "bravo charlie" (13 bytes) and walk to column 10
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	for range 10 {
		NavHandle('l', &st, &cur, &sel, lines, offsets)
	}
	if cur.CharPos != 10 {
		t.Fatalf("setup: CharPos = %d want 10", cur.CharPos)
	}
	if cur.Preferred != 10 {
		t.Fatalf("setup: Preferred = %d want 10", cur.Preferred)
	}
	// Move to "delta" (5 bytes) — CharPos should clamp to 5
	NavHandle('j', &st, &cur, &sel, lines, offsets)
	if cur.CharPos != 5 {
		t.Errorf("CharPos on short line = %d want 5", cur.CharPos)
	}
	if cur.Preferred != 10 {
		t.Errorf("Preferred unchanged by j; got %d want 10", cur.Preferred)
	}
}

func TestNav_PreferredColumnOnReturnToLongLine(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	for range 10 {
		NavHandle('l', &st, &cur, &sel, lines, offsets)
	}
	// j to "delta" then back to "bravo charlie"
	NavHandle('j', &st, &cur, &sel, lines, offsets)
	NavHandle('k', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 1 {
		t.Fatalf("LineIdx = %d want 1", cur.LineIdx)
	}
	if cur.CharPos != 10 {
		t.Errorf("CharPos on return = %d want 10", cur.CharPos)
	}
}

func TestNav_HLeavesPreferredUnchanged(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 10}
	cur.Preferred = 10
	var sel NavSelection
	NavHandle('h', &st, &cur, &sel, lines, offsets)
	if cur.Preferred != 10 {
		t.Errorf("Preferred after h = %d want 10", cur.Preferred)
	}
}

func TestNav_ByteOffsetRoundTrip(t *testing.T) {
	lines, offsets := navLines()
	for i := range lines {
		got := byteOffset(offsets, i, 0)
		if got != offsets[i] {
			t.Errorf("byteOffset(%d, 0) = %d want %d", i, got, offsets[i])
		}
	}
}

func TestNav_RuneSnapLeft(t *testing.T) {
	// "éclair" is multi-byte: e(1) + 'é'(2 bytes) + c(1) + l(1) + a(1) + i(1) + r(1) = 8 bytes
	// runeStart at byte 4 should land at byte 3 (start of 'c')
	got := runeStart("éclair", 4)
	if got != 3 {
		t.Errorf("runeStart = %d want 3", got)
	}
}

func TestNav_RuneAdvance(t *testing.T) {
	// runeAdvance at byte 0 should jump over 'é' (2 bytes) to byte 2
	got := runeAdvance("éclair", 0)
	if got != 2 {
		t.Errorf("runeAdvance(0) = %d want 2", got)
	}
}

func TestNav_WAdvancesToNextWord(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	// "alpha" — cursor at pos 0 ('a' of alpha) → next word "bravo" starts at line 1, pos 0
	cur := NavCursor{LineIdx: 0, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('w', &st, &cur, &sel, lines, offsets); got != ActionWordRight {
		t.Errorf("action = %v want ActionWordRight", got)
	}
	if cur.LineIdx != 1 || cur.CharPos != 0 {
		t.Errorf("cursor after w = (%d,%d) want (1,0)", cur.LineIdx, cur.CharPos)
	}
}

func TestNav_BRetreatsToPreviousWord(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	// cursor on 'b' of "bravo" (line 1, pos 0) → previous word "alpha" starts at line 0, pos 0
	cur := NavCursor{LineIdx: 1, CharPos: 0}
	var sel NavSelection
	if got := NavHandle('b', &st, &cur, &sel, lines, offsets); got != ActionWordLeft {
		t.Errorf("action = %v want ActionWordLeft", got)
	}
	if cur.LineIdx != 0 || cur.CharPos != 0 {
		t.Errorf("cursor after b = (%d,%d) want (0,0)", cur.LineIdx, cur.CharPos)
	}
}

func TestNav_WCrossesNewline(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	// cursor on last rune of "bravo charlie" (line 1, pos 12 = 'e') → next word "delta" at line 2, pos 0
	cur := NavCursor{LineIdx: 1, CharPos: 12}
	var sel NavSelection
	NavHandle('w', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 2 || cur.CharPos != 0 {
		t.Errorf("cursor after w from (1,12) = (%d,%d) want (2,0)", cur.LineIdx, cur.CharPos)
	}
}

func TestNav_BCrossesNewline(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	// cursor on 'd' of "delta" (line 2, pos 0) → previous word "charlie" ends at line 1, pos 12
	// the START of that previous word is line 1, pos 6 ('c' of charlie)
	cur := NavCursor{LineIdx: 2, CharPos: 0}
	var sel NavSelection
	NavHandle('b', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 1 || cur.CharPos != 6 {
		t.Errorf("cursor after b from (2,0) = (%d,%d) want (1,6)", cur.LineIdx, cur.CharPos)
	}
}

func TestNav_WSkipsBlankLine(t *testing.T) {
	lines := []string{"hello", "", "world"}
	offsets := []int{0, 6, 7}
	var st NavState
	// cursor on last rune of "hello" (line 0, pos 4) → next word "world" at line 2, pos 0
	cur := NavCursor{LineIdx: 0, CharPos: 4}
	var sel NavSelection
	NavHandle('w', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 2 || cur.CharPos != 0 {
		t.Errorf("cursor after w blank-skip = (%d,%d) want (2,0)", cur.LineIdx, cur.CharPos)
	}
}

func TestNav_WUpdatesPreferred(t *testing.T) {
	var st NavState
	lines2 := []string{"alpha beta", "gamma"}
	offsets2 := []int{0, 11}
	cur := NavCursor{LineIdx: 0, CharPos: 0, Preferred: 0}
	var sel NavSelection
	// w from 'a' of "alpha" lands on 'b' of "beta" at (0, 6); Preferred becomes 6.
	NavHandle('w', &st, &cur, &sel, lines2, offsets2)
	if cur.LineIdx != 0 || cur.CharPos != 6 {
		t.Fatalf("setup: cursor = (%d,%d) want (0,6)", cur.LineIdx, cur.CharPos)
	}
	if cur.Preferred != 6 {
		t.Errorf("Preferred after w = %d want 6", cur.Preferred)
	}
}

func TestNav_BLeavesPreferredUnchanged(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 1, CharPos: 0, Preferred: 12}
	var sel NavSelection
	NavHandle('b', &st, &cur, &sel, lines, offsets)
	if cur.Preferred != 12 {
		t.Errorf("Preferred after b = %d want 12 (unchanged)", cur.Preferred)
	}
}

func TestNav_VWExtendsSelection(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 0, CharPos: 0}
	var sel NavSelection
	// Enter visual; ByteA seeded at (0, 0)
	NavHandle('v', &st, &cur, &sel, lines, offsets)
	wantA := byteOffset(offsets, 0, 0)
	if sel.ByteA != wantA || sel.ByteC != wantA {
		t.Fatalf("setup: selection = (%d,%d) want (%d,%d)", sel.ByteA, sel.ByteC, wantA, wantA)
	}
	// w moves cursor to (1, 0); ByteC should track
	NavHandle('w', &st, &cur, &sel, lines, offsets)
	wantC := byteOffset(offsets, 1, 0)
	if sel.ByteA != wantA {
		t.Errorf("ByteA after w in visual = %d want %d (unchanged)", sel.ByteA, wantA)
	}
	if sel.ByteC != wantC {
		t.Errorf("ByteC after w in visual = %d want %d", sel.ByteC, wantC)
	}
}

func TestNav_WNoOpAtEndOfSource(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	// cursor on last rune of "delta" (line 2, pos 4) — no next word exists
	cur := NavCursor{LineIdx: 2, CharPos: 4, Preferred: 4}
	var sel NavSelection
	NavHandle('w', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 2 || cur.CharPos != 4 {
		t.Errorf("w at end-of-source should no-op; got (%d,%d)", cur.LineIdx, cur.CharPos)
	}
}

func TestNav_BNoOpAtStartOfSource(t *testing.T) {
	lines, offsets := navLines()
	var st NavState
	cur := NavCursor{LineIdx: 0, CharPos: 0}
	var sel NavSelection
	NavHandle('b', &st, &cur, &sel, lines, offsets)
	if cur.LineIdx != 0 || cur.CharPos != 0 {
		t.Errorf("b at start-of-source should no-op; got (%d,%d)", cur.LineIdx, cur.CharPos)
	}
}