// Package render parses agent message markdown, indexes it into
// navigable blocks, and returns the raw source for each block so the
// caller can display it in a viewport. No styling: the returned
// string is the verbatim markdown text of the message, sliced at
// block boundaries.
//
// Block boundaries are derived from the goldmark AST and tracked by
// line indices in the concatenated output so the caller can navigate
// by block and mark the current block with a gutter indicator.
package render

import (
	"bytes"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extensionAst "github.com/yuin/goldmark/extension/ast"
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
	BlockTable     BlockKind = "table"
	BlockDefList   BlockKind = "definition-list"
)

// Block is one navigable unit in the source markdown.
//
// StartLine and EndLine are 0-indexed line offsets in the concatenated
// raw source produced by renderBlocks. They are contiguous: blocks do
// not overlap, and consecutive blocks are joined back-to-back.
//
// HasComment is set true by RenderMessageWithComments when at least one
// saved comment targets this block. The flag drives the yellow left-
// gutter indicator in the rendered view.
type Block struct {
	Kind       BlockKind
	Source     string
	StartLine  int
	EndLine    int
	HasComment bool
}

// renderBlocks parses md and slices it into top-level blocks. Each
// block's source is the verbatim markdown text from that node's start
// byte to the next sibling's start byte (or end of source for the
// last block). Empty blocks, thematic breaks, and HTML blocks are
// dropped. Lists produce one entry per non-empty ListItem.
func renderBlocks(md string) (string, []Block) {
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
			if strings.TrimSpace(ext.source) == "" {
				continue
			}
			rendered := ext.source + "\n"
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

// CurrentBlockIdx returns the index of the block containing the given
// viewport YOffset, or -1 if yOffset is outside any block (above the
// first block or past the last).
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

type extracted struct {
	kind   BlockKind
	source string
}

// extract returns one entry per navigable block. Top-level nodes
// that are themselves blocks produce one entry; List nodes produce one
// entry per non-empty ListItem. Each entry's `source` is the raw
// markdown text from the node's first line to the next sibling's
// first line (or end of source). Empty (whitespace-only) sources
// are skipped at the top so per-case switches don't repeat the check.
func extract(node ast.Node, src []byte) []extracted {
	start, end := byteRange(node, src)
	if start < 0 || start >= end {
		return nil
	}
	source := strings.TrimRight(string(src[start:end]), "\n")
	if strings.TrimSpace(source) == "" {
		return nil
	}

	switch n := node.(type) {
	case *ast.Heading:
		return []extracted{{kind: BlockHeading, source: source}}

	case *ast.Paragraph:
		return []extracted{{kind: BlockParagraph, source: source}}

	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return []extracted{{kind: BlockCode, source: source}}

	case *ast.Blockquote:
		return []extracted{{kind: BlockQuote, source: source}}

	case *ast.List:
		// One block per ListItem, each with its own raw byte range.
		var out []extracted
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			li, ok := child.(*ast.ListItem)
			if !ok {
				continue
			}
			liStart, liEnd := byteRange(li, src)
			if liStart < 0 || liStart >= liEnd {
				continue
			}
			itemSrc := strings.TrimRight(string(src[liStart:liEnd]), "\n")
			if strings.TrimSpace(itemSrc) == "" {
				continue
			}
			out = append(out, extracted{kind: BlockListItem, source: itemSrc})
		}
		return out

	case *extensionAst.Table:
		return []extracted{{kind: BlockTable, source: source}}

	case *extensionAst.DefinitionList:
		return []extracted{{kind: BlockDefList, source: source}}
	}
	return nil
}

// byteRange returns the start and end byte offsets (into src) of a
// block's content. start is the node's Pos() (the start of its first
// source line). end is the start of the next sibling — walking up
// the parent chain so a last child of a container picks up the
// container's next sibling, not end-of-source.
func byteRange(node ast.Node, src []byte) (start, end int) {
	start = node.Pos()
	if start < 0 {
		return -1, -1
	}
	end = len(src)
	for n := ast.Node(node); n != nil; n = n.Parent() {
		if next := n.NextSibling(); next != nil && next.Pos() >= 0 {
			end = next.Pos()
			break
		}
	}
	if end < start {
		end = start
	}
	return start, end
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n")
}

// CommentKind discriminates which fields of Comment carry the
// anchor. Block-kind comments anchor to a rendered block in the
// latest agent message; file-kind comments anchor to a file path
// and optional byte range in the agent's workspace.
type CommentKind int

const (
	CommentBlock CommentKind = iota // existing — block-anchored
	CommentFile                     // new — file-anchored
)

// Comment is a user-authored annotation attached either to a
// rendered block in the agent's latest message or to a file in
// the agent's workspace.
//
// Block-kind fields (CommentBlock):
//   - BlockIdx: index into the []Block slice for the block this
//     comment annotates.
//   - CharStart, CharEnd: byte offsets into the block's Source. A
//     block-level comment uses CharStart == CharEnd == -1 as a
//     sentinel for "no inline byte offsets". An inline comment
//     uses the goldmark AST node's Lines().At(i).Start/.Stop for
//     the anchor line and the cursor line.
//
// File-kind fields (CommentFile):
//   - Path: relative path to the file under the agent pane's cwd.
//   - LineStart, LineEnd: 1-based inclusive line range.
//   - CharStart, CharEnd: byte offsets into the file's raw content
//     for inline selections made via visual mode; -1 / -1 for
//     line-range comments without an inline selection.
//
// Source is the verbatim substring of the anchored range
// (block.Source or the file's raw content) for inline comments,
// so the redirect appendix can quote the snippet without
// re-reading the file. For line-range / block-level comments
// Source is empty.
type Comment struct {
	Kind      CommentKind
	BlockIdx  int
	CharStart int
	CharEnd   int
	Path      string
	LineStart int
	LineEnd   int
	Source    string
	Text      string
	CreatedAt time.Time
}

// IsInlineSelection reports whether c carries a byte-range inline
// selection (vs. a whole-block or whole-line range). Applies to
// both block-kind and file-kind comments.
func (c Comment) IsInlineSelection() bool { return c.CharStart >= 0 }

// footnoteMarker returns "▸" for block-level (CharStart < 0),
// "•" for inline. Consumed by footnoteLines. Not exported — the
// model layer's left-gutter replaces the in-block visual.
func footnoteMarker(c Comment) string {
	if c.CharStart < 0 {
		return "▸"
	}
	return "•"
}