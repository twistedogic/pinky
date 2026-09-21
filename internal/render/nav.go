package render

// NavAction is the result of handling one nav keypress.
type NavAction int

const (
	ActionNone      NavAction = iota
	ActionBlockDown           // j
	ActionBlockUp             // k
	ActionRuneLeft            // h
	ActionRuneRight           // l
	ActionEnterVisual
	ActionExitVisual
	ActionComment
	ActionSend
	ActionRefresh
	ActionQuit
	ActionCompose
	ActionHelp
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

// NavCursor is the model's pointer into the rendered content.
// BlockIdx is into the []Block slice; CharPos is a byte offset
// into blocks[BlockIdx].Source (range [0, len(Source)]).
type NavCursor struct {
	BlockIdx int
	CharPos  int
}

// NavSelection is the byte range under visual selection.
// CharA/CharC are byte offsets within Selection.BlockIdx's
// source; the anchor is the CharA side, the cursor the CharC
// side. BlockIdx is whichever block currently holds the cursor's
// tail — when the cursor crosses a block boundary the selection
// re-anchors to the new block (the anchor side is preserved as
// CharA relative to that block).
type NavSelection struct {
	BlockIdx int
	CharA    int
	CharC    int
}

// NavHandle processes one nav keypress. Mutates *cur on motion
// keys, mutates *sel on motion while visual is active, and
// flips st.Visual on v/Esc. Returns the model-level action; the
// caller (model.handleNavKey) handles comments / send / quit /
// refresh / compose / help.
func NavHandle(r rune, st *NavState, cur *NavCursor, sel *NavSelection, blocks []Block) NavAction {
	if r == 0x1b { // Esc
		if st.Visual == NavLine {
			st.Visual = NavNone
			*sel = NavSelection{}
			return ActionExitVisual
		}
		return ActionNone
	}
	switch r {
	case 'j':
		if cur.BlockIdx < len(blocks)-1 {
			cur.BlockIdx++
		}
		cur.CharPos = 0
		if st.Visual == NavLine {
			sel.BlockIdx = cur.BlockIdx
			sel.CharC = 0
		}
		return ActionBlockDown
	case 'k':
		if cur.BlockIdx > 0 {
			cur.BlockIdx--
		}
		cur.CharPos = 0
		if st.Visual == NavLine {
			sel.BlockIdx = cur.BlockIdx
			sel.CharC = 0
		}
		return ActionBlockUp
	case 'h':
		if cur.CharPos > 0 {
			cur.CharPos--
		}
		if st.Visual == NavLine {
			sel.CharC = cur.CharPos
		}
		return ActionRuneLeft
	case 'l':
		max := 0
		if cur.BlockIdx >= 0 && cur.BlockIdx < len(blocks) {
			max = len(blocks[cur.BlockIdx].Source)
		}
		if cur.CharPos < max {
			cur.CharPos++
		}
		if st.Visual == NavLine {
			sel.CharC = cur.CharPos
		}
		return ActionRuneRight
	case 'v':
		if st.Visual == NavLine {
			st.Visual = NavNone
			*sel = NavSelection{}
			return ActionExitVisual
		}
		st.Visual = NavLine
		*sel = NavSelection{BlockIdx: cur.BlockIdx, CharA: cur.CharPos, CharC: cur.CharPos}
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
	case '?':
		return ActionHelp
	}
	return ActionNone
}

// NavLineIndex maps a (blockIdx, charPos) cursor to the rendered
// line index within the concatenated viewport. Same 1-line drift
// tolerance on glamour word-wrap as the comment-projection
// helpers in anchor.go.
func NavLineIndex(blocks []Block, c NavCursor) int {
	if c.BlockIdx < 0 || c.BlockIdx >= len(blocks) {
		return 0
	}
	return blocks[c.BlockIdx].StartLine + byteToLineInBlock(blocks[c.BlockIdx], c.CharPos)
}
