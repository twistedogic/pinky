package render

import "strings"

// wordWrap breaks s into lines no wider than width, splitting on
// whitespace. Newlines in s are preserved. words longer than width
// are placed on their own line unbroken. Used by LineGutter to emit
// per-source-line wrapped sub-lines.
func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		col := 0
		for _, word := range strings.Fields(line) {
			w := len(word)
			switch {
			case col == 0:
				b.WriteString(word)
				col = w
			case col+1+w <= width:
				b.WriteByte(' ')
				b.WriteString(word)
				col += 1 + w
			default:
				b.WriteByte('\n')
				b.WriteString(word)
				col = w
			}
		}
	}
	return b.String()
}

// LineGutter walks the message text line by line, word-wraps each
// line independently to wrapWidth, and prepends a gutter character
// to each wrapped sub-line. The gutter is yellow `▍` (228) on every
// sub-line whose source line overlaps at least one saved comment's
// byte range, or a space otherwise. The returned []int maps each
// wrapped sub-line back to its source-line index so the caller can
// translate viewport YOffset back to (lineIdx, charPos).
//
// Comments are matched by global byte range: a source line is
// "touched" if its [byteStart, byteEnd) range intersects any
// comment's [ByteA, ByteC). wrapWidth <= 0 falls back to a single
// wrapped line per source line (no actual wrap).
func LineGutter(text string, comments []Comment, wrapWidth int) (string, []int) {
	const yellow = "\x1b[38;5;228m"
	const reset = "\x1b[0m"
	lines := strings.Split(text, "\n")
	lineStart := make([]int, len(lines))
	offset := 0
	for i, ln := range lines {
		lineStart[i] = offset
		offset += len(ln) + 1 // +1 for the "\n"
	}
	// Pre-index overlapping comments per source line for O(N+M) instead
	// of O(N*M).
	touched := make([]bool, len(lines))
	for _, c := range comments {
		for i := range lines {
			if c.ByteA < lineStart[i]+len(lines[i]) && c.ByteC > lineStart[i] {
				touched[i] = true
			}
		}
	}

	var b strings.Builder
	wrappedSrc := make([]int, 0, len(lines))
	for i, ln := range lines {
		var wrapped string
		if wrapWidth > 0 {
			wrapped = wordWrap(ln, wrapWidth)
		} else {
			wrapped = ln
		}
		wLines := strings.Split(wrapped, "\n")
		for _, wl := range wLines {
			if touched[i] {
				b.WriteString(yellow)
				b.WriteRune('▍')
				b.WriteString(reset)
			} else {
				b.WriteByte(' ')
			}
			b.WriteString(wl)
			b.WriteByte('\n')
			wrappedSrc = append(wrappedSrc, i)
		}
	}
	return strings.TrimRight(b.String(), "\n"), wrappedSrc
}