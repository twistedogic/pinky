package render

// VisualAction is the result of handling one visual-mode key event.
type VisualAction int

const (
	VisualNoAction VisualAction = iota
	VisualEnter
	VisualLineDown
	VisualLineUp
	VisualNextBlock
	VisualPrevBlock
	VisualComposer
	VisualExit
)

// VisualState tracks the inline visual selection state. The model
// owns a copy of this on its visual field. cursor is a rendered-line
// index; curBlock is the block containing the cursor (recomputed on
// every move so }/{ can use the live cursor position).
type VisualState struct {
	Mode     SelectionMode
	Anchor   int
	Cursor   int
	CharA    int
	CharC    int
	CurBlock int
}

// Handle processes one key rune and returns the action to take.
// blocks is the current message's block index; totalLines is the
// total rendered line count.
func (v *VisualState) Handle(r rune, totalLines int, blocks []Block) VisualAction {
	switch v.Mode {
	case SelNone:
		if r == 'V' {
			v.Mode = SelLine
			v.CurBlock = -1
			return VisualEnter
		}
		return VisualNoAction
	case SelLine:
		switch r {
		case 'j':
			v.Cursor++
			if v.Cursor >= totalLines {
				v.Cursor = totalLines - 1
			}
			v.recomputeBlock(blocks)
			return VisualLineDown
		case 'k':
			v.Cursor--
			if v.Cursor < 0 {
				v.Cursor = 0
			}
			v.recomputeBlock(blocks)
			return VisualLineUp
		case '}':
			if y := JumpBlock(blocks, v.CurBlock, +1); y >= 0 {
				v.Cursor = y
				v.recomputeBlock(blocks)
				return VisualNextBlock
			}
		case '{':
			if y := JumpBlock(blocks, v.CurBlock, -1); y >= 0 {
				v.Cursor = y
				v.recomputeBlock(blocks)
				return VisualPrevBlock
			}
		case 'c':
			return VisualComposer
		case 0x1b: // Esc
			v.Mode = SelNone
			return VisualExit
		}
	}
	return VisualNoAction
}

// Enter parks the cursor at the given rendered line (typically the
// currently-focused block's StartLine). Sets CharA/CharC from the
// byte offset of that line within the anchor block.
func (v *VisualState) Enter(startLine int, blockIdx int, blocks []Block) {
	v.Mode = SelLine
	v.Anchor = startLine
	v.Cursor = startLine
	v.CurBlock = blockIdx
	off := blockByteOffset(blocks, blockIdx, startLine)
	v.CharA, v.CharC = off, off
}

// recomputeBlock updates CurBlock based on the cursor position and
// refreshes CharC to match.
func (v *VisualState) recomputeBlock(blocks []Block) {
	idx := CurrentBlockIdx(blocks, v.Cursor)
	if idx >= 0 {
		v.CurBlock = idx
	}
	v.CharC = blockByteOffset(blocks, v.CurBlock, v.Cursor)
}

// blockByteOffset maps a rendered line index to the byte offset of
// the source line it represents within the block.
func blockByteOffset(blocks []Block, blockIdx, renderedLine int) int {
	if blockIdx < 0 || blockIdx >= len(blocks) {
		return -1
	}
	b := blocks[blockIdx]
	if renderedLine < b.StartLine {
		renderedLine = b.StartLine
	}
	if renderedLine > b.EndLine {
		renderedLine = b.EndLine
	}
	return LineByteOffset(b, renderedLine-b.StartLine)
}
