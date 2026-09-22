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
	"time"

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
//
// HasComment is set true by RenderMessageWithComments when at least one
// saved comment targets this block. RenderMessage leaves it false. The
// flag drives the yellow left-gutter indicator in the rendered view.
type Block struct {
	Kind       BlockKind
	Source     string
	StartLine  int
	EndLine    int
	HasComment bool
}

// NewRenderer returns a glamour TermRenderer configured for pinky.
//
// The pinky style lives in style.go and uses the same color family
// as the lipgloss TUI (51 cyan selection, 228 yellow comments,
// 250 body, 241 dim, 42 user), so headings/links/etc feel native
// to the rest of the UI.
//
// The pinky style sets Document.Margin to 1; we pass width+1 to
// WordWrap so the rendered content area (1 margin + content) totals
// the requested width. The model's left-gutter ▍ sits in column 2.
//
// `WithChromaFormatter("terminal256")` enables syntax highlighting
// for fenced code blocks via chroma.
func NewRenderer(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStyles(pinkyStyle()),
		glamour.WithChromaFormatter("terminal256"),
		glamour.WithWordWrap(width+1),
	)
}

// cachedRenderer holds the last-built glamour renderer keyed by width.
// tea.Update is single-threaded so no mutex is needed. Width only
// changes on terminal resize, so cache hit rate in steady state is
// ~100%; glamour setup is heavy (style parse + chroma).
var cachedRenderer struct {
	width int
	r     *glamour.TermRenderer
}

func cachedRendererFor(width int) (*glamour.TermRenderer, error) {
	if cachedRenderer.r != nil && cachedRenderer.width == width {
		return cachedRenderer.r, nil
	}
	r, err := NewRenderer(width)
	if err != nil {
		return nil, err
	}
	cachedRenderer.r = r
	cachedRenderer.width = width
	return r, nil
}

// renderBlocks parses md and renders each non-empty top-level block
// (heading, paragraph, code block, list item, blockquote). Empty
// paragraphs, thematic breaks, and HTML blocks are dropped. Each
// block is rendered exactly once and concatenated into the returned
// string; the []Block index carries (Kind, Source, line range) into
// that output.
func renderBlocks(md string, width int) (string, []Block) {
	r, err := cachedRendererFor(width)
	if err != nil {
		return md, nil
	}
	root := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,     // GitHub-flavored markdown (tables, strikethrough, task lists, autolinks)
			extension.Linkify, // auto-detect URLs in text and turn them into links
			extension.DefinitionList,
		),
	).Parser().Parse(text.NewReader([]byte(md)))
	src := []byte(md)

	var buf bytes.Buffer
	var blocks []Block
	line := 0
	for child := root.FirstChild(); child != nil; child = child.NextSibling() {
		for _, ext := range extract(child, src) {
			rendered, _ := r.Render(ext.source + "\n")
			if rendered == "" {
				continue
			}
			count := lineCount(rendered)
			if count == 0 {
				continue
			}
			buf.WriteString(rendered)
			blocks = append(blocks, Block{
				Kind:      ext.kind,
				Source:    ext.source,
				StartLine: line,
				EndLine:   line + count - 1,
			})
			line += count
		}
	}
	return buf.String(), blocks
}

// RenderMessage renders md block-by-block and returns the concatenated
// rendered string plus the matching block index. The []Block return
// value covers callers that only need navigation too — splitting out
// a BuildBlockIndex helper would just duplicate the parse work.
func RenderMessage(md string, width int) (string, []Block) {
	return renderBlocks(md, width)
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

// Comment is a user-authored annotation attached to a block or to a
// byte range within a block's source.
//
// Anchor shape:
//   - BlockIdx: index into the []Block slice for the block this
//     comment annotates.
//   - CharStart, CharEnd: byte offsets into the block's Source. A
//     block-level comment uses CharStart == CharEnd == -1 as a
//     sentinel for "no inline range". An inline comment uses the
//     goldmark AST node's Lines().At(i).Start/.Stop for the
//     anchor line and the cursor line.
//
// Source is the verbatim substring of the block's source for inline
// comments (so the redirect appendix can quote the snippet without
// re-parsing). For block-level comments Source is empty.
type Comment struct {
	Kind      BlockKind // kind of the block this comment annotates
	BlockIdx  int
	CharStart int
	CharEnd   int
	Source    string
	Text      string
	CreatedAt time.Time
}

// Marker returns the gutter marker for a comment: "▸" for block-level
// (CharStart < 0), "•" for inline. It is consumed by the footnote
// line renderer (the ▸/• prefix on the comment's footnote). It no
// longer feeds the in-block gutter marker; the model layer's
// left-gutter replaces that visual.
func (c Comment) Marker() string {
	if c.CharStart < 0 {
		return "▸"
	}
	return "•"
}
