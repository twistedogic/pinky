package render

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
)

// BlockByteRange returns the start and end byte offsets (into src)
// of the block-level content for the given AST node.
//
// ponytail: byte-range vs. line-range; glamour wrap may project a
// source line to N rendered lines. The byte range is exact (1-line
// drift at word-wrap boundaries only affects visual highlighting,
// not the redirect excerpt).
func BlockByteRange(node ast.Node, src []byte) (start, end int) {
	lines := node.Lines()
	if lines.Len() == 0 {
		return 0, 0
	}
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	return first.Start, last.Stop
}

// LineByteOffset returns the byte offset of the renderedLineIdx-th
// source line within block. The block's first rendered line maps to
// its first source line, etc.
//
// ponytail: glamour word-wrap can split a source line into N rendered
// lines without exposing the split points, so this mapping may drift
// by 1 rendered line at wrap boundaries. Acceptable per design — the
// redirect excerpt uses the exact source byte range, not the
// projection.
func LineByteOffset(block Block, renderedLineIdx int) int {
	if block.Source == "" {
		return 0
	}
	lines := bytes.Split([]byte(block.Source), []byte("\n"))
	if renderedLineIdx < 0 || renderedLineIdx >= len(lines) {
		return len(block.Source)
	}
	offset := 0
	for i := 0; i < renderedLineIdx; i++ {
		offset += len(lines[i]) + 1
	}
	return offset
}

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
