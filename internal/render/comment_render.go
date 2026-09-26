package render

import (
	"cmp"
	"slices"
	"strings"

	"github.com/muesli/reflow/wordwrap"
)

// footnote is one comment's footnote line, ready to be injected
// after the block it annotates.
type footnote struct {
	lineIdx int    // rendered line index where the footnote should be inserted
	marker  string // "▸" (block) or "•" (inline)
	body    string // footnote text, including the optional excerpt prefix
	inline  bool
	excerpt string
}



// RenderMessageWithComments renders md and overlays comment annotations
// on top of the result: a footnote line below each commented block
// and the HasComment flag set on every block that has at least one
// comment (used by the model layer to draw the yellow left-gutter
// indicator).
func RenderMessageWithComments(md string, comments []Comment) (string, []Block) {
	rendered, blocks := renderBlocks(md)
	if len(comments) == 0 {
		return rendered, blocks
	}
	for _, c := range comments {
		if c.BlockIdx >= 0 && c.BlockIdx < len(blocks) {
			blocks[c.BlockIdx].HasComment = true
		}
	}
	return injectFootnotes(rendered, footnoteLines(blocks, comments)), blocks
}

// footnoteLines returns one footnote per comment, sorted by (blockIdx,
// order-of-creation). The lineIdx is the EndLine of the block + 1
// (i.e. immediately after the block's last line).
func footnoteLines(blocks []Block, comments []Comment) []footnote {
	if len(comments) == 0 {
		return nil
	}
	out := make([]footnote, 0, len(comments))
	for _, c := range comments {
		if c.BlockIdx < 0 || c.BlockIdx >= len(blocks) {
			continue
		}
		insertAt := blocks[c.BlockIdx].EndLine + 1
		// Multiple comments on the same block → stack after the previous
		// footnote line.
		for _, prev := range out {
			if prev.lineIdx == insertAt {
				insertAt++
			}
		}
		marker := footnoteMarker(c)
		inline := c.CharStart >= 0
		out = append(out, footnote{
			lineIdx: insertAt,
			marker:  marker,
			body:    c.Text,
			inline:  inline,
			excerpt: truncate(c.Source, excerptLimit),
		})
	}
	return out
}

// InjectGutterWrapped is word-wrap-aware: each rendered source
// line is word-wrapped independently to wrapWidth, then a gutter
// is prepended per WRAPPED line. The gutter color is derived from
// the block that contains the source line, so all wrapped lines
// of the same markdown source line carry the same gutter. The
// returned int slice maps wrapped-line index → source-line index
// so callers (the viewport owner) can translate wrapped scroll
// positions back into source-line coordinates when matching
// against blocks. wrapWidth <= 0 falls back to a single wrapped
// line per source line (no actual wrap).
func InjectGutterWrapped(rendered string, blocks []Block, focused int, wrapWidth int) (string, []int) {
	const cyan = "\x1b[38;5;51m"
	const yellow = "\x1b[38;5;228m"
	const reset = "\x1b[0m"
	sourceLines := strings.Split(rendered, "\n")
	var b strings.Builder
	srcIdx := make([]int, 0, len(sourceLines))
	for i, sl := range sourceLines {
		var wrapped string
		if wrapWidth > 0 {
			wrapped = wordwrap.String(sl, wrapWidth)
		} else {
			wrapped = sl
		}
		wLines := strings.Split(wrapped, "\n")
		for _, wl := range wLines {
			idx := CurrentBlockIdx(blocks, i)
			switch {
			case focused >= 0 && idx == focused:
				b.WriteString(cyan)
				b.WriteRune('▍')
				b.WriteString(reset)
			case idx >= 0 && blocks[idx].HasComment:
				b.WriteString(yellow)
				b.WriteRune('▍')
				b.WriteString(reset)
			default:
				b.WriteByte(' ')
			}
			b.WriteString(wl)
			b.WriteByte('\n')
			srcIdx = append(srcIdx, i)
		}
	}
	return strings.TrimRight(b.String(), "\n"), srcIdx
}

// injectFootnotes inserts each footnote line immediately after its
// block's rendered range. Multiple footnotes at the same insertion
// point stack in declaration order.
func injectFootnotes(rendered string, footnotes []footnote) string {
	if len(footnotes) == 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	slices.SortFunc(footnotes, func(a, b footnote) int {
		return cmp.Compare(a.lineIdx, b.lineIdx)
	})
	inserts := make(map[int][]string, len(footnotes))
	maxIdx := len(lines) - 1
	for _, f := range footnotes {
		body := "  " + f.marker + " " + f.body
		if f.inline {
			body = "  " + f.marker + " “" + f.excerpt + "” — " + f.body
		}
		if f.lineIdx > maxIdx {
			f.lineIdx = maxIdx + 1
		}
		for inserts[f.lineIdx] != nil {
			f.lineIdx++
		}
		inserts[f.lineIdx] = append(inserts[f.lineIdx], body)
	}
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
		if extras, ok := inserts[i]; ok {
			for _, e := range extras {
				b.WriteString(e)
				b.WriteByte('\n')
			}
		}
	}
	if extras, ok := inserts[maxIdx+1]; ok {
		for _, e := range extras {
			b.WriteString(e)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}