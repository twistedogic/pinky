package render

import "bytes"

// RenderedLineRange projects a (CharStart, CharEnd) byte range in
// the original markdown onto a (startLine, endLine) range within the
// block's rendered output. Returns the block's full StartLine/EndLine
// for block-level comments (CharStart == CharEnd == -1).
func RenderedLineRange(blocks []Block, blockIdx int, charA, charC int) (startLine, endLine int) {
	if blockIdx < 0 || blockIdx >= len(blocks) {
		return 0, 0
	}
	b := blocks[blockIdx]
	if charA < 0 {
		return b.StartLine, b.EndLine
	}
	a, c := charA, charC
	if a > c {
		a, c = c, a
	}
	startLine = b.StartLine + byteToLineInBlock(b, a)
	endLine = b.StartLine + byteToLineInBlock(b, c)
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}
	return startLine, endLine
}

func byteToLineInBlock(block Block, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(block.Source) {
		return bytes.Count([]byte(block.Source), []byte("\n"))
	}
	return bytes.Count([]byte(block.Source)[:offset], []byte("\n"))
}
