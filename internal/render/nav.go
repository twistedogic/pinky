package render

import "unicode/utf8"

// NavAction is the result of handling one nav keypress.
type NavAction int

const (
	ActionNone      NavAction = iota
	ActionBlockDown           // j
	ActionBlockUp             // k
	ActionRuneLeft            // h
	ActionRuneRight           // l
	ActionWordRight           // w
	ActionWordLeft            // b
	ActionEnterVisual
	ActionExitVisual
	ActionComment
	ActionSend
	ActionRefresh
	ActionQuit
	ActionCompose
)

// NavMode tracks the visual selection mode.
type NavMode int

const (
	NavNone NavMode = iota
	NavLine
)

// NavState is the nav state machine state. Visual is a mode flag,
// not a dispatch owner.
type NavState struct {
	Visual NavMode
}

// NavCursor is the model's pointer into the message source. LineIdx
// is 1-based into the source line slice (one entry per "\n"-split
// line of m.latest.Text). CharPos is a byte offset into that source
// line, clamped to [0, len(currentLine)] and snapped to rune
// boundaries on every motion. Preferred is the rightmost column
// reached by `l` and survives `j`/`k`, matching the file viewer's
// preferred-column tracker.
type NavCursor struct {
	LineIdx   int
	CharPos   int
	Preferred int
}

// NavSelection is a global byte range over m.latest.Text. ByteA is
// the anchor byte (set by `v`), ByteC is the cursor byte (updated
// by motion in visual mode). The visible byte range is
// [min(ByteA, ByteC), max(ByteA, ByteC)]; ordering by byte position
// is the source of truth, not which side is anchor vs. cursor.
type NavSelection struct {
	ByteA int
	ByteC int
}

// NavHandle processes one nav keypress. Mutates *cur on motion
// keys, mutates *sel on motion while visual is active, and flips
// st.Visual on v/Esc. Returns the model-level action; the caller
// (model.handleNavKey) handles comments / send / quit / refresh /
// compose / help.
//
// lines is the message's "\n"-split source. lineStartOffsets[i] is
// the byte offset of lines[i]'s first byte in m.latest.Text, so
// lineStartOffsets[0] is always 0 and lineStartOffsets[i+1] ==
// lineStartOffsets[i] + len(lines[i]) + 1 for i < len(lines)-1.
func NavHandle(r rune, st *NavState, cur *NavCursor, sel *NavSelection, lines []string, lineStartOffsets []int) NavAction {
	if r == 0x1b { // Esc
		if st.Visual == NavLine {
			st.Visual = NavNone
			*sel = NavSelection{}
			return ActionExitVisual
		}
		return ActionNone
	}
	if cur.LineIdx < 0 {
		cur.LineIdx = 0
	}
	if cur.LineIdx >= len(lines) {
		if len(lines) == 0 {
			cur.LineIdx = 0
			cur.CharPos = 0
		} else {
			cur.LineIdx = len(lines) - 1
		}
	}
	switch r {
	case 'j':
		if cur.LineIdx < len(lines)-1 {
			cur.LineIdx++
		}
		cur.CharPos = preferredCharPos(lines[cur.LineIdx], cur.Preferred)
		if st.Visual == NavLine {
			sel.ByteC = byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		}
		return ActionBlockDown
	case 'k':
		if cur.LineIdx > 0 {
			cur.LineIdx--
		}
		cur.CharPos = preferredCharPos(lines[cur.LineIdx], cur.Preferred)
		if st.Visual == NavLine {
			sel.ByteC = byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		}
		return ActionBlockUp
	case 'h':
		if cur.CharPos > 0 {
			cur.CharPos = runeStart(lines[cur.LineIdx], cur.CharPos)
		}
		if st.Visual == NavLine {
			sel.ByteC = byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		}
		return ActionRuneLeft
	case 'w':
		li, cp := NextWordStart(lines, cur.LineIdx, cur.CharPos)
		cur.LineIdx, cur.CharPos = li, cp
		if cur.CharPos > cur.Preferred {
			cur.Preferred = cur.CharPos
		}
		if st.Visual == NavLine {
			sel.ByteC = byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		}
		return ActionWordRight
	case 'b':
		li, cp := PrevWordStart(lines, cur.LineIdx, cur.CharPos)
		cur.LineIdx, cur.CharPos = li, cp
		if st.Visual == NavLine {
			sel.ByteC = byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		}
		return ActionWordLeft
	case 'l':
		max := len(lines[cur.LineIdx])
		if cur.CharPos < max {
			cur.CharPos = runeAdvance(lines[cur.LineIdx], cur.CharPos)
		}
		if cur.CharPos > cur.Preferred {
			cur.Preferred = cur.CharPos
		}
		if st.Visual == NavLine {
			sel.ByteC = byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		}
		return ActionRuneRight
	case 'v':
		if st.Visual == NavLine {
			st.Visual = NavNone
			*sel = NavSelection{}
			return ActionExitVisual
		}
		st.Visual = NavLine
		bo := byteOffset(lineStartOffsets, cur.LineIdx, cur.CharPos)
		sel.ByteA = bo
		sel.ByteC = bo
		return ActionEnterVisual
	case 'c':
		return ActionComment
	case 's':
		return ActionSend
	case 'r':
		return ActionRefresh
	case 'q':
		return ActionQuit
	case 'n':
		return ActionCompose
	}
	return ActionNone
}

// preferredCharPos returns min(preferred, len(line)), snapped to the
// nearest rune start at or before that byte. Matches the file
// viewer's preferred-column rule for `j`/`k`.
func preferredCharPos(line string, preferred int) int {
	clamped := preferred
	if clamped > len(line) {
		clamped = len(line)
	}
	return snapLeft(line, clamped)
}

// byteOffset returns the global byte offset of (lineIdx, charPos) in
// the concatenated message text. Caller passes a precomputed
// lineStartOffsets table; this function is a single add.
func byteOffset(lineStartOffsets []int, lineIdx, charPos int) int {
	if lineIdx < 0 {
		lineIdx = 0
	}
	if lineIdx >= len(lineStartOffsets) {
		if len(lineStartOffsets) == 0 {
			return charPos
		}
		lineIdx = len(lineStartOffsets) - 1
	}
	return lineStartOffsets[lineIdx] + charPos
}

// snapLeft returns bytePos snapped to the start of the rune that
// contains bytePos. If bytePos falls mid-rune, returns the start of
// the rune preceding bytePos. Used by `j`/`k` to keep CharPos on a
// valid rune boundary after clamping.
func snapLeft(s string, bytePos int) int {
	if bytePos <= 0 {
		return 0
	}
	if bytePos >= len(s) {
		return len(s)
	}
	for bytePos > 0 {
		_, sz := utf8.DecodeRuneInString(s[bytePos:])
		if bytePos+sz <= len(s) {
			return bytePos
		}
		bytePos--
	}
	return 0
}

// runeStart returns the start of the rune that ENDS at bytePos-1
// (the rune "just behind" the cursor at bytePos). Used by `h` to
// retreat one rune. Returns 0 if bytePos is 0.
func runeStart(s string, bytePos int) int {
	if bytePos <= 0 {
		return 0
	}
	if bytePos >= len(s) {
		bytePos = len(s)
	}
	prev, sz := utf8.DecodeLastRuneInString(s[:bytePos])
	if prev == utf8.RuneError && sz <= 1 {
		return 0
	}
	return bytePos - sz
}

// runeAdvance returns the byte position immediately after the rune
// that starts at bytePos. Used by `l` to advance one rune.
func runeAdvance(s string, bytePos int) int {
	if bytePos < 0 {
		return 0
	}
	if bytePos >= len(s) {
		return len(s)
	}
	_, sz := utf8.DecodeRuneInString(s[bytePos:])
	return bytePos + sz
}

// NavLineIndex maps a (lineIdx, charPos) cursor to the rendered line
// index inside the concatenated viewport. wrappedToSrc[yOffset]
// gives the source-line index for each viewport line; the byte
// within the source line maps to one of the wrapped sub-lines.
//
// This helper is the post-wrap projection; the byteToLineInBlock
// helper for the old block model is gone — wrapping is now a
// per-source-line concern (see LineGutter).
func NavLineIndex(sourceToFirst []int, c NavCursor) int {
	if c.LineIdx < 0 || c.LineIdx >= len(sourceToFirst) {
		return 0
	}
	return sourceToFirst[c.LineIdx]
}