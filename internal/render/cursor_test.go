package render

import (
	"strings"
	"testing"
)

func TestWrapLineWithRanges_NoWrap(t *testing.T) {
	// width 0 or large: single sub-line, range covers whole line.
	wrapped, ranges := wrapLineWithRanges("hello world", 100)
	if wrapped != "hello world" {
		t.Errorf("wrapped = %q want %q", wrapped, "hello world")
	}
	if len(ranges) != 1 {
		t.Fatalf("ranges len = %d want 1", len(ranges))
	}
	if ranges[0] != [2]int{0, 11} {
		t.Errorf("range = %v want [0, 11]", ranges[0])
	}
}

func TestWrapLineWithRanges_Wraps(t *testing.T) {
	// "the quick brown fox jumps over" — wrap width 10:
	//   "the quick"  bytes 0..8
	//   "brown fox"   bytes 10..18
	//   "jumps over"  bytes 20..29
	wrapped, ranges := wrapLineWithRanges("the quick brown fox jumps over", 10)
	want := "the quick\nbrown fox\njumps over"
	if wrapped != want {
		t.Errorf("wrapped =\n%q\nwant\n%q", wrapped, want)
	}
	if len(ranges) != 3 {
		t.Fatalf("ranges len = %d want 3", len(ranges))
	}
	if ranges[0] != [2]int{0, 9} {
		t.Errorf("range[0] = %v want [0, 9]", ranges[0])
	}
	if ranges[1] != [2]int{10, 19} {
		t.Errorf("range[1] = %v want [10, 19]", ranges[1])
	}
	if ranges[2] != [2]int{20, 30} {
		t.Errorf("range[2] = %v want [20, 30]", ranges[2])
	}
}

func TestApplyInlineCursor_ShortLine(t *testing.T) {
	rendered, _ := LineGutter("alpha\nbravo\ncharlie", nil, 80)
	// Source line 0 ("alpha"), charPos 2 ("p"). Should paint "p".
	out := ApplyInlineCursor(rendered, []string{"alpha", "bravo", "charlie"}, 0, 2, []int{0, 1, 2}, 80)
	if !strings.Contains(out, "\x1b[7mp\x1b[27m") {
		t.Errorf("expected inverted 'p' in output; got %q", out)
	}
}

func TestApplyInlineCursor_LongLineWraps(t *testing.T) {
	// "the quick brown fox jumps over" — wrap width 10:
	//   sub-line 0 "the quick"   source bytes 0..9   charPos 4 = "q" of "quick"
	//   sub-line 1 "brown fox"   source bytes 10..19 charPos 14 = "o" of "brown"
	//   sub-line 2 "jumps over"  source bytes 20..29
	text := "the quick brown fox jumps over"
	rendered, _ := LineGutter(text, nil, 10)
	sourceToFirst := []int{0, 2, 4} // first wrapped sub-line per source line (single-line message)

	out := ApplyInlineCursor(rendered, []string{text}, 0, 14, sourceToFirst, 10)
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 rendered lines; got %d", len(lines))
	}
	// subIdx for charPos=14 is 1 ("brown fox" — bytes 10..19). The 'n' in 'brown' should be inverted.
	if !strings.Contains(lines[1], "\x1b[7mn\x1b[27m") {
		t.Errorf("expected inverted 'n' on sub-line 1 (brown fox); got %q", lines[1])
	}
}

func TestApplyInlineCursor_EndOfLine(t *testing.T) {
	rendered, _ := LineGutter("alpha", nil, 80)
	// charPos == len(line): trailing space cursor.
	out := ApplyInlineCursor(rendered, []string{"alpha"}, 0, 5, []int{0}, 80)
	if !strings.HasSuffix(out, "\x1b[7m \x1b[27m") {
		t.Errorf("expected trailing inverted space at end of line; got %q", out)
	}
}

func TestApplyInlineCursor_OutOfRange(t *testing.T) {
	rendered := " line1\n line2\n"
	out := ApplyInlineCursor(rendered, []string{"line1", "line2"}, 5, 0, []int{0, 1}, 80)
	if out != rendered {
		t.Errorf("out-of-range lineIdx should return input unchanged")
	}
}

func TestApplyCursorToGuttered_YellowGutter(t *testing.T) {
	// Yellow gutter prefix, body "abc".
	guttered := "\x1b[38;5;228m▍\x1b[0mabc"
	out := applyCursorToGuttered(guttered, "abc", 1)
	if !strings.HasPrefix(out, "\x1b[38;5;228m▍\x1b[0m") {
		t.Errorf("gutter should be preserved; got %q", out)
	}
	if !strings.Contains(out, "\x1b[7mb\x1b[27m") {
		t.Errorf("expected inverted 'b'; got %q", out)
	}
}

func TestApplyCursorToGuttered_SpaceGutter(t *testing.T) {
	guttered := " abc"
	out := applyCursorToGuttered(guttered, "abc", 0)
	if !strings.HasPrefix(out, " ") {
		t.Errorf("space gutter should be preserved; got %q", out)
	}
	if !strings.Contains(out, "\x1b[7ma\x1b[27m") {
		t.Errorf("expected inverted 'a'; got %q", out)
	}
}