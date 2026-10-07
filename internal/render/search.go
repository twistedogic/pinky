package render

import (
	"strconv"
	"strings"
)

// Hit is one literal-string match of a search query against a
// source line. (LineIdx, ByteA, ByteC) is in source-line byte
// space: ByteA inclusive, ByteC exclusive. Byte offsets are
// valid in lines[LineIdx].
type Hit struct {
	LineIdx int
	ByteA   int
	ByteC   int
}

// FindHits returns every non-overlapping left-to-right literal
// substring match of query in lines, in document order (line
// asc, byte asc). Matching is case-insensitive. Empty query or
// nil lines returns nil.
//
// Literal substring means the query bytes (after case-folding)
// must appear as a contiguous run in each line — not as a
// subsequence. So "abc" matches "xxabcxx" but not "axxbc". This
// matches what users typing a query into a TUI search expect:
// each typed rune is a character they want to find, not a fuzzy
// signal. Multi-byte runes are matched byte-for-byte against the
// case-folded line.
func FindHits(query string, lines []string) []Hit {
	if query == "" || len(lines) == 0 {
		return nil
	}
	qlen := len(query)
	lowq := strings.ToLower(query)
	var hits []Hit
	for i, line := range lines {
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		// Walk the line; on every match, emit a hit and advance
		// past the match so overlapping occurrences are not
		// emitted (the next search starts at low[ByteC]).
		off := 0
		for {
			idx := strings.Index(low[off:], lowq)
			if idx < 0 {
				break
			}
			start := off + idx
			hits = append(hits, Hit{LineIdx: i, ByteA: start, ByteC: start + qlen})
			off = start + qlen
			if off >= len(line) {
				break
			}
		}
	}
	return hits
}

// SpliceStyle layers an ANSI style escape (typically a background
// highlight like "\x1b[48;5;58m") around the body bytes at
// [start, end) inside `line`. ANSI escapes inside `line` are passed
// through verbatim and not split; `start`/`end` are body byte
// offsets (ANSI bytes do not count).
//
// On close, SpliceStyle restores the parent background so cells
// immediately before and after the splice keep any glamour-set
// background (e.g. the 228-on-236 code block style). The
// restoration is done by remembering the most recent SGR escape
// that set a background, and writing it back on close. A reset
// ("\x1b[0m" or "\x1b[m") clears the remembered background, so
// the close becomes "\x1b[49m" (default).
//
// If start >= end the function returns line unchanged.
func SpliceStyle(line, on string, start, end int) string {
	if start >= end {
		return line
	}
	const closeDefault = "\x1b[49m"
	var b strings.Builder
	pos := 0
	currentBG := ""
	wroteOn := false

	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			// Read SGR escape: "\x1b[...m" (terminator is the
			// first byte in 0x40-0x7e; for SGR that's 'm').
			j := i + 2
			for j < len(line) && (line[j] < 0x40 || line[j] > 0x7e) {
				j++
			}
			if j >= len(line) {
				// Malformed; copy the rest unchanged.
				b.WriteString(line[i:])
				break
			}
			j++ // include the terminator
			esc := line[i:j]
			b.WriteString(esc)
			currentBG = updateSGRBackground(esc, currentBG)
			i = j
			continue
		}
		if pos == start && !wroteOn {
			b.WriteString(on)
			wroteOn = true
		}
		b.WriteByte(line[i])
		pos++
		i++
		if pos == end && wroteOn {
			if currentBG != "" {
				b.WriteString(currentBG)
			} else {
				b.WriteString(closeDefault)
			}
			wroteOn = false
		}
	}
	if wroteOn {
		if currentBG != "" {
			b.WriteString(currentBG)
		} else {
			b.WriteString(closeDefault)
		}
	}
	return b.String()
}

// updateSGRBackground returns the SGR escape to remember for
// background restoration, given the current remembered bg and a
// new SGR escape. The remembered bg is cleared by:
//   - "\x1b[0m" or "\x1b[m" (full reset, SGR 0)
//   - "\x1b[49m" (default bg, SGR 49)
// The remembered bg is updated by an escape containing
// "48;5;N" or "48;2;R;G;B" (any fg set in the same escape is
// preserved by remembering the whole escape).
func updateSGRBackground(esc, currentBG string) string {
	if len(esc) < 3 || esc[0] != 0x1b || esc[1] != '[' || esc[len(esc)-1] != 'm' {
		return currentBG
	}
	body := esc[2 : len(esc)-1]
	if body == "" {
		// "\x1b[m" — same as reset.
		return ""
	}
	params := strings.Split(body, ";")
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case "0", "49":
			// Full reset or explicit default bg — clear the
			// remembered bg.
			return ""
		case "48":
			if i+1 >= len(params) {
				return currentBG
			}
			switch params[i+1] {
			case "5":
				if i+2 < len(params) {
					if _, err := strconv.Atoi(params[i+2]); err == nil {
						return esc
					}
				}
			case "2":
				if i+4 < len(params) {
					return esc
				}
			}
			return currentBG
		}
	}
	return currentBG
}

// SpliceStyleAcrossWrap applies SpliceStyle to every wrapped
// sub-line of `h` inside the multi-row `rendered` string. The
// caller passes the source lines (used for the wrap walk) and
// sourceToFirst (the wrapped-row index of the first sub-line for
// each source line, parallel to lines).
//
// `h.LineIdx` must be a valid index into `lines`; otherwise
// `rendered` is returned unchanged.
func SpliceStyleAcrossWrap(rendered string, lines []string, h Hit, on string, wrapWidth int, sourceToFirst []int) string {
	if h.LineIdx < 0 || h.LineIdx >= len(lines) {
		return rendered
	}
	_, subRanges := wrapLineWithRanges(lines[h.LineIdx], wrapWidth)
	rows := strings.Split(rendered, "\n")
	if h.LineIdx >= len(sourceToFirst) {
		return rendered
	}
	base := sourceToFirst[h.LineIdx]
	if base >= len(rows) {
		return rendered
	}
	const gutter = 1
	for subIdx, sr := range subRanges {
		lo, hi := h.ByteA, h.ByteC
		if lo < sr[0] {
			lo = sr[0]
		}
		if hi > sr[1] {
			hi = sr[1]
		}
		if lo >= hi {
			continue
		}
		row := base + subIdx
		if row >= len(rows) {
			break
		}
		// Hit offsets are in source-line byte space; SpliceStyle's
		// body counter starts at the gutter cell, so shift by
		// gutter + (lo - subStart).
		bodyA := gutter + (lo - sr[0])
		bodyC := gutter + (hi - sr[0])
		rows[row] = SpliceStyle(rows[row], on, bodyA, bodyC)
	}
	return strings.Join(rows, "\n")
}
