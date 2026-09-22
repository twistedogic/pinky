package render

import "bytes"

// byteToLineInBlock counts newlines in block.Source[:offset], so
// the result is the 0-indexed line number within the block. Used
// by NavLineIndex to project a (blockIdx, charPos) cursor to its
// rendered line.
func byteToLineInBlock(block Block, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(block.Source) {
		return bytes.Count([]byte(block.Source), []byte("\n"))
	}
	return bytes.Count([]byte(block.Source)[:offset], []byte("\n"))
}