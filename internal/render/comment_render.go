package render

import (
	"strings"
	"cmp"
	"slices"
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

// commentTint is the lipgloss style applied to lines inside a
// commented block's range. Background-only so the foreground colors
// of glamour's output are preserved.

// tintLine wraps a rendered line in a background-fill ANSI sequence
// (the lipgloss styles drop the background when the input already
// carries foreground color codes, which glamour's output always does).
// We append enough trailing-bg spaces to fill the terminal width so
// the background extends across the full line, then a final reset.
func tintLine(line string, width int) string {
	const bg = "[48;5;237m"
	const reset = "[0m"
	visW := VisibleWidth(line)
	pad := width - visW
	if pad < 0 {
		pad = 0
	}
	return bg + line + strings.Repeat(" ", pad) + reset
}

// VisibleWidth counts non-ANSI runes in s. Used for status-bar
// padding so the rendered bar fills the terminal even when its bg
// style would otherwise break lipgloss.Width. Exported because the
// model package needs to apply the same padding.
func VisibleWidth(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		if r == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}


// gutterMarkerANSI is the foreground ANSI for the gutter marker prepended
// to the first rendered line of a commented block.
const gutterMarkerANSI = "[38;5;228m"

// RenderMessageWithComments renders md and overlays comment annotations
// on top of the result: a footnote line below each commented block,
// a gutter marker on the first line of each commented block, and a
// background tint over the commented range.
func RenderMessageWithComments(md string, width int, comments []Comment) (string, []Block) {
	rendered, blocks := renderBlocks(md, width)
	if len(comments) == 0 {
		return rendered, blocks
	}
	tinted := applyHighlights(rendered, blocks, comments, width)
	withFootnotes := injectFootnotes(tinted, footnoteLines(blocks, comments))
	return withFootnotes, blocks
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
		marker := c.Marker()
		inline := c.CharStart >= 0
		excerpt := c.Source
		if len(excerpt) > 40 {
			excerpt = excerpt[:40] + "…"
		}
		out = append(out, footnote{
			lineIdx: insertAt,
			marker:  marker,
			body:    c.Text,
			inline:  inline,
			excerpt: excerpt,
		})
	}
	return out
}

// applyHighlights rewrites rendered line-by-line, applying the gutter
// marker on the first rendered line of each commented block and a
// background tint over each commented block's rendered range.
func applyHighlights(rendered string, blocks []Block, comments []Comment, width int) string {
	if len(comments) == 0 {
		return rendered
	}
	// Build the set of (start, end) line ranges (relative to the
	// pre-footnote rendered output) per block.
	type range_ struct{ start, end int }
	rangeByBlock := make(map[int]range_, len(blocks))
	for _, c := range comments {
		s, e := RenderedLineRange(blocks, c.BlockIdx, c.CharStart, c.CharEnd)
		if cur, ok := rangeByBlock[c.BlockIdx]; !ok || s < cur.start {
			rangeByBlock[c.BlockIdx] = range_{start: s, end: e}
		} else {
			rangeByBlock[c.BlockIdx] = range_{start: cur.start, end: e}
		}
	}
	// Pick the first comment per block as the marker source.
	markerByBlock := make(map[int]string, len(blocks))
	for _, c := range comments {
		if _, ok := markerByBlock[c.BlockIdx]; !ok {
			markerByBlock[c.BlockIdx] = c.Marker()
		}
	}
	lines := strings.Split(rendered, "\n")
	var b strings.Builder
	for i, line := range lines {
		idx := CurrentBlockIdx(blocks, i)
		if idx >= 0 {
			if r, ok := rangeByBlock[idx]; ok && i >= r.start && i <= r.end {
				line = tintLine(line, width)
				if _, ok := markerByBlock[idx]; ok && i == r.start {
					line = gutterMarkerANSI + markerByBlock[idx] + "[0m " + line
				}
			}
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// injectFootnotes inserts each footnote line immediately after its
// block's rendered range. Multiple footnotes at the same insertion
// point stack in declaration order.
func injectFootnotes(rendered string, footnotes []footnote) string {
	if len(footnotes) == 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	// Sort footnotes by lineIdx ascending so earlier insertions don't
	// shift later indices. slices.SortFunc is stable, so declaration
	// order is preserved for ties.
	slices.SortFunc(footnotes, func(a, b footnote) int {
		return cmp.Compare(a.lineIdx, b.lineIdx)
	})
	// Build (insertion index → list of footnote lines to insert).
	// Slice-valued map entries accumulate so multiple footnotes at
	// the same insertion point stack in declaration order.
	inserts := make(map[int][]string, len(footnotes))
	maxIdx := len(lines) - 1 // highest index in the split result
	for _, f := range footnotes {
		body := "  " + f.marker + " " + f.body
		if f.inline {
			body = "  " + f.marker + " “" + f.excerpt + "” — " + f.body
		}
		// Clamp footnote at-or-past the last line to "after the
		// final line" sentinel handled below.
		if f.lineIdx > maxIdx {
			f.lineIdx = maxIdx + 1
		}
		// Dedup: if another footnote already claimed this exact
		// lineIdx, shift ours down by one. Allow stacking past
		// maxIdx for the multi-comment-on-last-block case.
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
	// Footnotes that landed past the end (clamped to maxIdx+1) get
	// appended here in their original order.
	if extras, ok := inserts[maxIdx+1]; ok {
		for _, e := range extras {
			b.WriteString(e)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
// TrimLeadingVisible strips n visible (non-ANSI) characters from
// the start of s. Exposed so the model's border injection can reuse
// it instead of keeping its own copy.
func TrimLeadingVisible(s string, n int) string {
	var b strings.Builder
	skipped := 0
	inEscape := false
	for _, r := range s {
		if r == 0x1b {
			inEscape = true
			b.WriteRune(r)
			continue
		}
		if inEscape {
			b.WriteRune(r)
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if skipped < n {
			skipped++
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// HasCommentGutter reports whether s starts with the gutter marker
// (▸ or •) that the comment renderer prepends to the first line of
// a commented block. Used so the left bar doesn't clobber the marker.
func HasCommentGutter(s string) bool {
	inEscape := false
	for _, r := range s {
		if r == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		return r == '\u25B8' || r == '\u2022'
	}
	return false
}
