package render

import (
	"unicode"
	"unicode/utf8"
)

// isWordRune reports whether r is part of an iskeyword run (vim's
// default iskeyword set: ASCII letters and digits, underscore, plus
// any other Unicode letter or digit).
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordClass buckets a rune for word-motion purposes.
type wordClass int

const (
	classEOF wordClass = iota
	classSep          // whitespace, or implicit '\n' between source lines
	classPunct        // non-keyword, non-whitespace (".", "*", etc.)
	classWord         // iskeyword rune
)

func classifyRune(r rune) wordClass {
	switch {
	case r == utf8.RuneError:
		return classEOF
	case unicode.IsSpace(r):
		return classSep
	case isWordRune(r):
		return classWord
	default:
		return classPunct
	}
}

// runeAt returns the rune and its class at (lineIdx, charPos). The
// implicit '\n' at the end of every non-last source line is classSep;
// positions past the last rune of the source return (0, classEOF).
func runeAt(lines []string, lineIdx, charPos int) (rune, wordClass) {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return 0, classEOF
	}
	line := lines[lineIdx]
	if charPos < len(line) {
		r, _ := utf8.DecodeRuneInString(line[charPos:])
		return r, classifyRune(r)
	}
	if charPos == len(line) {
		if lineIdx < len(lines)-1 {
			return '\n', classSep
		}
		return 0, classEOF
	}
	return 0, classEOF
}

// advancePos moves (lineIdx, charPos) forward by one rune. At the
// implicit \n of a non-last line, jumps to start of next line. At
// end of source, returns the position unchanged.
func advancePos(lines []string, lineIdx, charPos int) (int, int) {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return lineIdx, charPos
	}
	line := lines[lineIdx]
	if charPos < len(line) {
		_, sz := utf8.DecodeRuneInString(line[charPos:])
		return lineIdx, charPos + sz
	}
	if charPos == len(line) && lineIdx+1 < len(lines) {
		return lineIdx + 1, 0
	}
	return lineIdx, charPos
}

// retreatPos moves (lineIdx, charPos) backward by one rune. At start
// of a non-first line, jumps to end of previous line. At start of
// source, returns the position unchanged.
func retreatPos(lines []string, lineIdx, charPos int) (int, int) {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return lineIdx, charPos
	}
	if charPos > 0 {
		_, sz := utf8.DecodeLastRuneInString(lines[lineIdx][:charPos])
		return lineIdx, charPos - sz
	}
	if lineIdx > 0 {
		return lineIdx - 1, len(lines[lineIdx-1])
	}
	return lineIdx, charPos
}

// nextWordStart returns the (lineIdx, charPos) of the start of the
// next word after the cursor at (lineIdx, charPos). Blank lines are
// separators; punctuation runs are words on their own (vim's iskeyword
// model). Returns the input unchanged if no next word exists.
func NextWordStart(lines []string, lineIdx, charPos int) (int, int) {
	li, cp := lineIdx, charPos
	_, cls := runeAt(lines, li, cp)

	// Step 1: skip past the rest of the current word/punct run.
	if cls == classWord || cls == classPunct {
		want := cls
		for {
			li, cp = advancePos(lines, li, cp)
			if _, c := runeAt(lines, li, cp); c != want {
				_, cls = runeAt(lines, li, cp)
				break
			}
		}
	}

	// Step 2: skip separators (whitespace + implicit '\n' at line ends,
	// which lets us cross blank lines freely).
	for cls == classSep {
		li, cp = advancePos(lines, li, cp)
		_, cls = runeAt(lines, li, cp)
	}

	if cls == classEOF {
		return lineIdx, charPos // no next word
	}
	return li, cp
}

// prevWordStart returns the (lineIdx, charPos) of the start of the
// previous word before the cursor at (lineIdx, charPos). Blank lines
// are separators; punctuation runs are words on their own. Returns
// the input unchanged if no previous word exists.
func PrevWordStart(lines []string, lineIdx, charPos int) (int, int) {
	li, cp := lineIdx, charPos
	_, cls := runeAt(lines, li, cp)

	// Step 1: walk back through any current word/punct run, stopping
	// one byte before the run's start (i.e., on the separator preceding
	// the run, or at start of source).
	if cls == classWord || cls == classPunct {
		want := cls
		for {
			nli, ncp := retreatPos(lines, li, cp)
			if nli == li && ncp == cp {
				break // at start of source
			}
			li, cp = nli, ncp
			if _, c := runeAt(lines, li, cp); c != want {
				break // left the run
			}
		}
		_, cls = runeAt(lines, li, cp)
	}

	// Step 2: skip separators backwards (handles blank lines).
	for cls == classSep {
		prevLI, prevCP := li, cp
		li, cp = retreatPos(lines, li, cp)
		if li == prevLI && cp == prevCP {
			_, cls = runeAt(lines, li, cp)
			break
		}
		_, cls = runeAt(lines, li, cp)
	}

	if cls == classEOF {
		return lineIdx, charPos // no previous word
	}

	// Step 3: we're at the END of a previous word/punct run. Walk back
// to its START (the first rune of the run).
	want := cls
	for {
		nli, ncp := retreatPos(lines, li, cp)
		if nli == li && ncp == cp {
			break // at start of source
		}
		if _, c := runeAt(lines, nli, ncp); c != want {
			break // (nli, ncp) is just before the run; current (li, cp) is the run's start
		}
		li, cp = nli, ncp
	}
	return li, cp
}