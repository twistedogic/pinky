package render

import (
	"strings"
	"unicode/utf8"
)

// ApplyCursor renders an inverted-background cursor cell at the byte
// position `charPos` in `line`, applied to the rendered `selected`
// string. The cursor paints the rune at charPos (or a trailing space
// when charPos == len(line)) with foreground/background swapped.
//
// `selected` may contain ANSI escape sequences (the caller adds gutter
// styles before this point); `line` is the raw source without escapes.
// Byte positions line up between the two because all ANSI inserts
// happen at gutter boundaries (column 0) and don't perturb the body.
//
// Used by both the file viewer (single-line cursor) and the message
// view (one-line-at-a-time cursor after the wrap-aware ApplyInlineCursor
// extracts the rendered sub-line).
func ApplyCursor(selected, line string, charPos int) string {
	if charPos < 0 {
		charPos = 0
	}
	if charPos >= len(line) {
		// Trailing space at the line end.
		return selected + "\x1b[7m \x1b[27m"
	}
	_, sz := utf8.DecodeRuneInString(line[charPos:])
	end := charPos + sz
	return SpliceInvert(selected, charPos, end)
}

// SpliceInvert inserts \x1b[7m ... \x1b[27m around the rune spanning
// [start, end) in line. ANSI escapes inside `selected` are preserved
// and not split.
func SpliceInvert(selected string, start, end int) string {
	const cursorInvertOn = "\x1b[7m"
	const cursorInvertOff = "\x1b[27m"
	var b strings.Builder
	pos := 0
	wroteOn := false
	for i := 0; i < len(selected); {
		if selected[i] == 0x1b {
			// Pass through ANSI escape sequence verbatim.
			j := i + 1
			// CSI sequences end at a letter (typically 'm').
			for j < len(selected) && (selected[j] < 0x40 || selected[j] > 0x7e) {
				j++
			}
			if j < len(selected) {
				j++
			}
			b.WriteString(selected[i:j])
			i = j
			continue
		}
		if pos == start {
			b.WriteString(cursorInvertOn)
			wroteOn = true
		}
		// write one rune (or one byte for invalid UTF-8)
		_, sz := utf8.DecodeRuneInString(selected[i:])
		if sz == 0 {
			sz = 1
		}
		b.WriteString(selected[i : i+sz])
		i += sz
		pos++
		if pos == end && wroteOn {
			b.WriteString(cursorInvertOff)
			wroteOn = false
		}
	}
	if wroteOn {
		b.WriteString(cursorInvertOff)
	}
	return b.String()
}

// wrapLineWithRanges wraps `line` to `width` (whitespace-split, words
// longer than width kept on their own line) and returns the wrapped
// string plus the source-byte range of each sub-line as [start, end)
// pairs in `line`'s byte space. Used to map a cursor's (lineIdx,
// charPos) back to the wrapped sub-line containing charPos.
func wrapLineWithRanges(line string, width int) (string, [][2]int) {
	if width <= 0 || len(line) == 0 {
		return line, [][2]int{{0, len(line)}}
	}
	var b strings.Builder
	var ranges [][2]int
	col := 0
	var subStart, subEnd int
	first := true
	i := 0
	for i < len(line) {
		// Skip whitespace between words.
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		wordStart := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		wordEnd := i
		w := wordEnd - wordStart
		if first {
			subStart = wordStart
			b.WriteString(line[wordStart:wordEnd])
			subEnd = wordEnd
			col = w
			first = false
		} else if col+1+w <= width {
			b.WriteByte(' ')
			b.WriteString(line[wordStart:wordEnd])
			col += 1 + w
			subEnd = wordEnd
		} else {
			ranges = append(ranges, [2]int{subStart, subEnd})
			b.WriteByte('\n')
			b.WriteString(line[wordStart:wordEnd])
			subStart = wordStart
			subEnd = wordEnd
			col = w
		}
	}
	if first {
		// Empty line (no words).
		ranges = append(ranges, [2]int{0, 0})
	} else {
		ranges = append(ranges, [2]int{subStart, subEnd})
	}
	return b.String(), ranges
}

// ApplyInlineCursor takes the rendered text produced by LineGutter
// and overlays an inverted-block cursor at (lineIdx, charPos) on the
// rendered sub-line that contains that source-byte position. Used by
// the message view to mark the cursor's byte.
//
// `lines` is the source-line slice (m.lines); `sourceToFirst[lineIdx]`
// is the rendered-line index of the first wrapped sub-line for
// `lineIdx`. `width` is the wrap width passed to LineGutter.
//
// Returns the input `rendered` unchanged when lineIdx is out of range.
func ApplyInlineCursor(rendered string, lines []string, lineIdx, charPos int, sourceToFirst []int, width int) string {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return rendered
	}
	if lineIdx >= len(sourceToFirst) {
		return rendered
	}
	if charPos < 0 {
		charPos = 0
	}
	if charPos > len(lines[lineIdx]) {
		charPos = len(lines[lineIdx])
	}
	_, ranges := wrapLineWithRanges(lines[lineIdx], width)
	subIdx := 0
	for i, r := range ranges {
		if charPos >= r[0] && charPos <= r[1] {
			subIdx = i
			break
		}
		if charPos < r[0] {
			// Cursor is before this sub-line; snap to previous sub-line end.
			if i > 0 {
				subIdx = i - 1
			}
			break
		}
		subIdx = i
	}
	renderedLines := strings.Split(rendered, "\n")
	target := sourceToFirst[lineIdx] + subIdx
	if target >= len(renderedLines) {
		return rendered
	}
	bodyStart, bodyEnd := ranges[subIdx][0], ranges[subIdx][1]
	bodyCharPos := charPos - bodyStart
	if bodyCharPos < 0 {
		bodyCharPos = 0
	}
	if bodyCharPos > bodyEnd-bodyStart {
		bodyCharPos = bodyEnd - bodyStart
	}
	renderedLines[target] = applyCursorToGuttered(
		renderedLines[target],
		lines[lineIdx][bodyStart:bodyEnd],
		bodyCharPos,
	)
	return strings.Join(renderedLines, "\n")
}

// applyCursorToGuttered strips the gutter prefix (one space or a
// yellow ▍ ANSI gutter) from `gutteredLine` and applies ApplyCursor
// to the body, then re-prepends the gutter.
func applyCursorToGuttered(gutteredLine, sourceLine string, charPos int) string {
	bodyStart := 0
	if strings.HasPrefix(gutteredLine, "\x1b[") {
		idx := strings.Index(gutteredLine, "\x1b[0m")
		if idx >= 0 {
			bodyStart = idx + len("\x1b[0m")
		} else {
			bodyStart = 1
		}
	} else if len(gutteredLine) > 0 {
		bodyStart = 1
	}
	if bodyStart > len(gutteredLine) {
		return gutteredLine
	}
	gutter := gutteredLine[:bodyStart]
	body := gutteredLine[bodyStart:]
	return gutter + ApplyCursor(body, sourceLine, charPos)
}