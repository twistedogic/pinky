package render

import "testing"

// wordLines returns a small source-line slice for word-motion tests.
// Byte offsets follow the "+1" newline rule: line i starts at
// offsets[i] and occupies len(lines[i])+1 bytes (the +1 being the
// trailing newline). "foo bar" is 7 bytes; "baz" is 3 bytes.
//
//	0..6   "foo bar"
//	8..10  "baz"
//	12..16 "quux quux"   (offsets[2]=12, len=9+1=10 → next at 22; trimmed to fixture)
func wordLines() []string {
	return []string{"foo bar", "baz", "quux quux"}
}

func TestIsWordRune(t *testing.T) {
	for _, r := range []rune{'a', 'Z', '0', '_', 'é', '日'} {
		if !isWordRune(r) {
			t.Errorf("isWordRune(%q) = false want true", r)
		}
	}
	for _, r := range []rune{' ', '\t', '.', ',', '-', '#', '*', '`'} {
		if isWordRune(r) {
			t.Errorf("isWordRune(%q) = true want false", r)
		}
	}
}

func TestNextWordStart_BasicForward(t *testing.T) {
	lines := wordLines()
	// "foo bar" — cursor on 'f' of foo (line 0, pos 0) → next word "bar" at pos 4
	gotLine, gotPos := NextWordStart(lines, 0, 0)
	if gotLine != 0 || gotPos != 4 {
		t.Errorf("NextWordStart(0,0) = (%d,%d) want (0,4)", gotLine, gotPos)
	}
}

func TestNextWordStart_MidWord(t *testing.T) {
	lines := wordLines()
	// cursor on 'o' of foo (pos 1) → still next word "bar" at pos 4
	gotLine, gotPos := NextWordStart(lines, 0, 1)
	if gotLine != 0 || gotPos != 4 {
		t.Errorf("NextWordStart(0,1) = (%d,%d) want (0,4)", gotLine, gotPos)
	}
}

func TestNextWordStart_OnSeparator(t *testing.T) {
	lines := wordLines()
	// cursor on space at pos 3 → next word "bar" at pos 4
	gotLine, gotPos := NextWordStart(lines, 0, 3)
	if gotLine != 0 || gotPos != 4 {
		t.Errorf("NextWordStart(0,3) = (%d,%d) want (0,4)", gotLine, gotPos)
	}
}

func TestNextWordStart_PunctuationIsWord(t *testing.T) {
	// vim's iskeyword model: punctuation runs are words on their own.
	// "foo.bar.baz" — cursor on 'f' of foo (pos 0) → next word "." at pos 3
	lines := []string{"foo.bar.baz"}
	gotLine, gotPos := NextWordStart(lines, 0, 0)
	if gotLine != 0 || gotPos != 3 {
		t.Errorf("NextWordStart(0,0) on foo.bar.baz = (%d,%d) want (0,3)", gotLine, gotPos)
	}
}

func TestNextWordStart_CrossesNewline(t *testing.T) {
	lines := wordLines()
	// cursor on 'r' of bar (line 0, pos 6) → next word "baz" at line 1, pos 0
	gotLine, gotPos := NextWordStart(lines, 0, 6)
	if gotLine != 1 || gotPos != 0 {
		t.Errorf("NextWordStart(0,6) = (%d,%d) want (1,0)", gotLine, gotPos)
	}
}

func TestNextWordStart_SkipsBlankLine(t *testing.T) {
	// "hello", "", "world" — cursor at end of "hello" (line 0, pos 5)
	// → next word "world" at line 2, pos 0 (skipping the blank line)
	lines := []string{"hello", "", "world"}
	gotLine, gotPos := NextWordStart(lines, 0, 5)
	if gotLine != 2 || gotPos != 0 {
		t.Errorf("nextWordStart blank-skip = (%d,%d) want (2,0)", gotLine, gotPos)
	}
}

func TestNextWordStart_FromBlankLine(t *testing.T) {
	// cursor on a blank line → next word in the next non-blank line
	lines := []string{"hello", "", "world"}
	gotLine, gotPos := NextWordStart(lines, 1, 0)
	if gotLine != 2 || gotPos != 0 {
		t.Errorf("nextWordStart from blank = (%d,%d) want (2,0)", gotLine, gotPos)
	}
}

func TestNextWordStart_EndOfSourceNoOp(t *testing.T) {
	// cursor on last rune of last line — no next word exists
	lines := wordLines()
	gotLine, gotPos := NextWordStart(lines, 2, 9) // last 'x' of "quux quux"
	if gotLine != 2 || gotPos != 9 {
		t.Errorf("nextWordStart at end = (%d,%d) want (2,9) unchanged", gotLine, gotPos)
	}
}

func TestNextWordStart_MultibyteWordChars(t *testing.T) {
	// "héllo wörld" — 'é' (2 bytes) at pos 1, 'ö' (2 bytes) at pos 8
	// cursor on 'h' (pos 0) → next word "wörld" at pos 7
	lines := []string{"héllo wörld"}
	gotLine, gotPos := NextWordStart(lines, 0, 0)
	if gotLine != 0 || gotPos != 7 {
		t.Errorf("nextWordStart multibyte = (%d,%d) want (0,7)", gotLine, gotPos)
	}
}

func TestPrevWordStart_BasicBackward(t *testing.T) {
	lines := wordLines()
	// cursor on 'b' of bar (pos 4) → previous word "foo" at pos 0
	gotLine, gotPos := PrevWordStart(lines, 0, 4)
	if gotLine != 0 || gotPos != 0 {
		t.Errorf("PrevWordStart(0,4) = (%d,%d) want (0,0)", gotLine, gotPos)
	}
}

func TestPrevWordStart_MidWord(t *testing.T) {
	// cursor on 'a' of bar (pos 5) → previous word "foo" at pos 0
	lines := wordLines()
	gotLine, gotPos := PrevWordStart(lines, 0, 5)
	if gotLine != 0 || gotPos != 0 {
		t.Errorf("PrevWordStart(0,5) = (%d,%d) want (0,0)", gotLine, gotPos)
	}
}

func TestPrevWordStart_OnSeparator(t *testing.T) {
	// cursor on space at pos 3 → previous word "foo" at pos 0
	lines := wordLines()
	gotLine, gotPos := PrevWordStart(lines, 0, 3)
	if gotLine != 0 || gotPos != 0 {
		t.Errorf("PrevWordStart(0,3) = (%d,%d) want (0,0)", gotLine, gotPos)
	}
}

func TestPrevWordStart_CrossesNewline(t *testing.T) {
	// cursor on 'b' of baz (line 1, pos 0) → previous word "bar" starts at line 0, pos 4
	lines := wordLines()
	gotLine, gotPos := PrevWordStart(lines, 1, 0)
	if gotLine != 0 || gotPos != 4 {
		t.Errorf("PrevWordStart(1,0) = (%d,%d) want (0,4)", gotLine, gotPos)
	}
}

func TestPrevWordStart_SkipsBlankLine(t *testing.T) {
	// "hello", "", "world" — cursor on 'w' (line 2, pos 0) → previous word "hello" starts at line 0, pos 0
	lines := []string{"hello", "", "world"}
	gotLine, gotPos := PrevWordStart(lines, 2, 0)
	if gotLine != 0 || gotPos != 0 {
		t.Errorf("prevWordStart blank-skip = (%d,%d) want (0,0)", gotLine, gotPos)
	}
}

func TestPrevWordStart_FromBlankLine(t *testing.T) {
	// cursor on a blank line → previous word in the previous non-blank line
	lines := []string{"hello", "", "world"}
	gotLine, gotPos := PrevWordStart(lines, 1, 0)
	if gotLine != 0 || gotPos != 0 {
		t.Errorf("prevWordStart from blank = (%d,%d) want (0,0)", gotLine, gotPos)
	}
}

func TestPrevWordStart_StartOfSourceNoOp(t *testing.T) {
	// cursor on first rune of first line — no previous word exists
	lines := wordLines()
	gotLine, gotPos := PrevWordStart(lines, 0, 0)
	if gotLine != 0 || gotPos != 0 {
		t.Errorf("prevWordStart at start = (%d,%d) want (0,0) unchanged", gotLine, gotPos)
	}
}