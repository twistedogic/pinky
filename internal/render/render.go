// Package render parses agent message markdown, indexes it into navigable
// blocks, and renders it to a width-aware terminal string.
//
// Block boundaries are derived from the goldmark AST and tracked by line
// indices in the rendered output so the caller can navigate by block and
// mark the current block with an indicator.
package render

import (
	"bytes"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

type BlockKind string

const (
	BlockHeading   BlockKind = "heading"
	BlockParagraph BlockKind = "paragraph"
	BlockCode      BlockKind = "code"
	BlockListItem  BlockKind = "list-item"
	BlockQuote     BlockKind = "quote"
)

// Block is one navigable unit in a rendered agent message.
//
// StartLine and EndLine are 0-indexed line offsets in the concatenated
// rendered output produced by RenderMessage. They are contiguous: blocks
// do not overlap, and consecutive blocks are joined back-to-back.
type Block struct {
	Kind      BlockKind
	Source    string
	StartLine int
	EndLine   int
}

// NewRenderer returns a glamour TermRenderer configured for pinky.
// ponytail: cached by width at the call site; revisit if memory grows.
func NewRenderer(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
}

// BuildBlockIndex parses md and returns one Block per non-empty
// top-level block (heading, paragraph, code block, list item,
// blockquote). Empty paragraphs, thematic breaks, and HTML blocks are
// excluded. List items each become their own block.
func BuildBlockIndex(md string, width int) []Block {
	src := []byte(md)
	mdParser := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := mdParser.Parser().Parse(text.NewReader(src))

	r, err := NewRenderer(width)
	if err != nil {
		return nil
	}

	var out []Block
	line := 0
	for child := root.FirstChild(); child != nil; child = child.NextSibling() {
		for _, ext := range extract(child, src) {
			rendered, _ := r.Render(ext.source + "\n")
			count := lineCount(rendered)
			out = append(out, Block{
				Kind:      ext.kind,
				Source:    ext.source,
				StartLine: line,
				EndLine:   line + count - 1,
			})
			line += count
		}
	}
	return out
}

// RenderMessage renders md block-by-block and returns the concatenated
// rendered string plus the block index. The block index is the same one
// BuildBlockIndex would return for the same md and width.
func RenderMessage(md string, width int) (string, []Block) {
	r, err := NewRenderer(width)
	if err != nil {
		return md, nil
	}
	blocks := BuildBlockIndex(md, width)
	var buf bytes.Buffer
	for _, b := range blocks {
		out, err := r.Render(b.Source + "\n")
		if err != nil {
			continue
		}
		buf.WriteString(out)
	}
	return buf.String(), blocks
}

// CurrentBlockIdx returns the index of the block containing the given
// viewport YOffset, or -1 if yOffset is outside any block (above the
// first block or past the last).
//
// The "containing" block is the one whose StartLine is closest to (and
// not greater than) yOffset, bounded by EndLine.
func CurrentBlockIdx(blocks []Block, yOffset int) int {
	if len(blocks) == 0 {
		return -1
	}
	for i, b := range blocks {
		if yOffset >= b.StartLine && yOffset <= b.EndLine {
			return i
		}
	}
	return -1
}

// JumpBlock returns the StartLine of the block delta steps from
// blocks[idx], or -1 if the jump goes out of range.
//
// delta == +1: next block; delta == -1: previous block.
func JumpBlock(blocks []Block, idx, delta int) int {
	next := idx + delta
	if next < 0 || next >= len(blocks) {
		return -1
	}
	return blocks[next].StartLine
}

// JumpHeading returns the StartLine of the heading block delta steps
// from blocks[idx], or -1 if no such heading exists.
func JumpHeading(blocks []Block, idx, delta int) int {
	if delta > 0 {
		for i := idx + 1; i < len(blocks); i++ {
			if blocks[i].Kind == BlockHeading {
				return blocks[i].StartLine
			}
		}
		return -1
	}
	for i := idx - 1; i >= 0; i-- {
		if blocks[i].Kind == BlockHeading {
			return blocks[i].StartLine
		}
	}
	return -1
}

type extracted struct {
	kind   BlockKind
	source string
}

// extract walks a top-level AST node and returns the navigable blocks
// it contains. Top-level nodes that are themselves blocks produce one
// entry; List nodes produce one entry per non-empty ListItem.
func extract(node ast.Node, src []byte) []extracted {
	switch n := node.(type) {
	case *ast.Heading:
		s := blockText(n, src)
		if strings.TrimSpace(s) == "" {
			return nil
		}
		// Reconstruct heading source: "#" * Level + " " + text + "\n".
		return []extracted{{kind: BlockHeading, source: strings.Repeat("#", n.Level) + " " + s}}

	case *ast.Paragraph:
		s := blockText(n, src)
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return []extracted{{kind: BlockParagraph, source: s}}

	case *ast.FencedCodeBlock, *ast.CodeBlock:
		s := blockText(n, src)
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return []extracted{{kind: BlockCode, source: "```\n" + s + "\n```"}}

	case *ast.Blockquote:
		s := blockText(n, src)
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return []extracted{{kind: BlockQuote, source: s}}

	case *ast.List:
		var out []extracted
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			li, ok := child.(*ast.ListItem)
			if !ok {
				continue
			}
			s := blockText(li, src)
			if strings.TrimSpace(s) == "" {
				continue
			}
			out = append(out, extracted{kind: BlockListItem, source: "- " + s})
		}
		return out
	}
	return nil
}

// blockText concatenates text content from a block node. For nodes
// with their own Lines() (heading, paragraph, code block, blockquote)
// it uses those. For container nodes (ListItem) it walks children and
// concatenates their text. Empty nodes return "".
func blockText(node ast.Node, src []byte) string {
	if _, ok := node.(*ast.ListItem); ok {
		var b strings.Builder
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			b.WriteString(blockText(child, src))
		}
		return b.String()
	}
	lines := node.Lines()
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return b.String()
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n")
}
