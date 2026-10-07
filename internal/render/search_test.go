package render

import (
	"strings"
	"testing"
)

func TestFindHits_LiteralSubstring(t *testing.T) {
	// "alpha" in "alpha beta alpha" yields two contiguous
	// substring matches: bytes 0..5 and 11..16. A subsequence
	// matcher (the previous implementation) would have given a
	// second hit at {0,9,16} — covering the 'a' of "beta" plus
	// "lpha" from the second "alpha". Literal substring
	// matches only the actual word.
	hits := FindHits("alpha", []string{"alpha beta alpha"})
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if hits[0] != (Hit{LineIdx: 0, ByteA: 0, ByteC: 5}) {
		t.Errorf("hits[0] = %+v, want {0,0,5}", hits[0])
	}
	if hits[1] != (Hit{LineIdx: 0, ByteA: 11, ByteC: 16}) {
		t.Errorf("hits[1] = %+v, want {0,11,16} (literal second alpha)", hits[1])
	}
}

func TestFindHits_LiteralNotSubsequence(t *testing.T) {
	// "abc" must NOT match "axxbc" — the runes are in order
	// but not contiguous. This is the key difference from the
	// previous subsequence matcher.
	hits := FindHits("abc", []string{"axxbc"})
	if len(hits) != 0 {
		t.Errorf("literal 'abc' should not match subsequence 'axxbc'; got %d hits", len(hits))
	}
}

func TestFindHits_LiteralContiguousOnly(t *testing.T) {
	// "the" in "the cat the dog" yields two contiguous matches
	// at the two literal "the" substrings: bytes 0..3 and
	// 8..11. A subsequence matcher would have produced
	// {0,6,11} (spanning the space).
	hits := FindHits("the", []string{"the cat the dog"})
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if hits[0] != (Hit{LineIdx: 0, ByteA: 0, ByteC: 3}) {
		t.Errorf("hits[0] = %+v, want {0,0,3}", hits[0])
	}
	if hits[1] != (Hit{LineIdx: 0, ByteA: 8, ByteC: 11}) {
		t.Errorf("hits[1] = %+v, want {0,8,11} (literal second 'the')", hits[1])
	}
}

func TestFindHits_NoMatch(t *testing.T) {
	hits := FindHits("zzz", []string{"hello", "world"})
	if len(hits) != 0 {
		t.Errorf("hits = %d, want 0", len(hits))
	}
}

func TestFindHits_EmptyQuery(t *testing.T) {
	hits := FindHits("", []string{"hello"})
	if len(hits) != 0 {
		t.Errorf("empty query should yield no hits; got %d", len(hits))
	}
}

func TestFindHits_EmptyLines(t *testing.T) {
	hits := FindHits("the", nil)
	if len(hits) != 0 {
		t.Errorf("empty lines should yield no hits; got %d", len(hits))
	}
}

func TestFindHits_CaseInsensitive(t *testing.T) {
	hits := FindHits("THE", []string{"the cat", "The Fox", "thumbs"})
	// "the" matches "the cat" (0..3) and "The Fox" (0..3);
	// "thumbs" starts with "th" but no full "the" → no hit.
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if hits[0] != (Hit{LineIdx: 0, ByteA: 0, ByteC: 3}) {
		t.Errorf("hits[0] = %+v, want {0,0,3}", hits[0])
	}
	if hits[1] != (Hit{LineIdx: 1, ByteA: 0, ByteC: 3}) {
		t.Errorf("hits[1] = %+v, want {1,0,3}", hits[1])
	}
}

func TestFindHits_MultipleLines(t *testing.T) {
	hits := FindHits("foo", []string{"foo bar", "baz qux", "foofoo"})
	// "foo" in line 0 (0..3), in line 2 (0..3) and (3..6).
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want 3", len(hits))
	}
	if hits[0] != (Hit{LineIdx: 0, ByteA: 0, ByteC: 3}) {
		t.Errorf("hits[0] = %+v, want {0,0,3}", hits[0])
	}
	if hits[1] != (Hit{LineIdx: 2, ByteA: 0, ByteC: 3}) {
		t.Errorf("hits[1] = %+v, want {2,0,3}", hits[1])
	}
	if hits[2] != (Hit{LineIdx: 2, ByteA: 3, ByteC: 6}) {
		t.Errorf("hits[2] = %+v, want {2,3,6}", hits[2])
	}
}

func TestFindHits_OrderedByLineThenByte(t *testing.T) {
	hits := FindHits("a", []string{"x a", "a x", "a a"})
	if len(hits) != 4 {
		t.Fatalf("hits = %d, want 4", len(hits))
	}
	// "a" in "x a" (pos 2), "a" in "a x" (pos 0), "a" in "a a" (pos 0 and 2)
	want := []Hit{
		{LineIdx: 0, ByteA: 2, ByteC: 3},
		{LineIdx: 1, ByteA: 0, ByteC: 1},
		{LineIdx: 2, ByteA: 0, ByteC: 1},
		{LineIdx: 2, ByteA: 2, ByteC: 3},
	}
	for i, w := range want {
		if hits[i] != w {
			t.Errorf("hits[%d] = %+v, want %+v", i, hits[i], w)
		}
	}
}

func TestSpliceStyle_NoParentBG(t *testing.T) {
	// Plain line, no parent escape; splice should close with \x1b[49m.
	line := "hello world"
	got := SpliceStyle(line, "\x1b[48;5;58m", 6, 11) // "world"
	want := "hello \x1b[48;5;58mworld\x1b[49m"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSpliceStyle_ParentBGPreserved(t *testing.T) {
	// Code block style: parent sets bg 236. The splice on lands
	// AFTER the leading open (so position 6 is "world"). On close,
	// the parent bg must be restored so the cells after the hit
	// keep the code-block background.
	openEsc := "\x1b[38;5;228;48;5;236m"
	body := "hello world"
	closeEsc := "\x1b[0m"
	line := openEsc + body + closeEsc
	got := SpliceStyle(line, "\x1b[48;5;58m", 6, 11)
	want := openEsc + "hello " + "\x1b[48;5;58m" + "world" + "\x1b[38;5;228;48;5;236m" + closeEsc
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSpliceStyle_ParentResetClears(t *testing.T) {
	// After a reset, the next escape sets a new bg; the splice close
	// should restore that newer bg. The body is "hihello world" — the
	// first 2 bytes are before the reset, the next 11 are after, and
	// the second escape has no bg.
	line := "\x1b[38;5;228;48;5;236mhi\x1b[0m\x1b[38;5;51mhello world\x1b[0m"
	// Body byte 8 is 'o' (after "hihello " = 8 bytes). Body byte 13 is
	// 'd' (end of "world"). Splice on body [8, 13) = "o worl" — but
	// the test only needs to assert the close is \x1b[49m, not 236.
	got := SpliceStyle(line, "\x1b[48;5;58m", 8, 13)
	if !strings.Contains(got, "\x1b[48;5;58m") {
		t.Errorf("splice on should be present; got %q", got)
	}
	if !strings.Contains(got, "\x1b[49m") {
		t.Errorf("splice close should be \\x1b[49m after a reset (no parent bg); got %q", got)
	}
	if strings.Contains(got, "\x1b[48;5;236m") {
		t.Errorf("splice close should NOT restore the pre-reset bg 236; got %q", got)
	}
}

func TestSpliceStyle_CombinedSGR(t *testing.T) {
	// Single SGR escape combining fg + bg: 38;5;228;48;5;236m.
	// Parser must remember 236.
	esc := "\x1b[38;5;228;48;5;236m"
	line := esc + "hello world\x1b[0m"
	got := SpliceStyle(line, "\x1b[48;5;58m", 6, 11)
	if !strings.Contains(got, "\x1b[48;5;58mworld") {
		t.Errorf("splice on missing; got %q", got)
	}
	// Close should restore the combined escape (both fg and bg).
	if !strings.Contains(got, "\x1b[38;5;228;48;5;236m") {
		t.Errorf("splice close should restore the combined fg+bg; got %q", got)
	}
}

func TestSpliceStyle_TrueColorBG(t *testing.T) {
	// 48;2;R;G;B form: parser should remember and restore.
	esc := "\x1b[48;2;10;20;30m"
	line := esc + "hello world\x1b[0m"
	got := SpliceStyle(line, "\x1b[48;5;58m", 6, 11)
	if !strings.Contains(got, "\x1b[48;2;10;20;30m") {
		t.Errorf("truecolor parent bg should be restored on close; got %q", got)
	}
}

func TestSpliceStyle_HitAtStart(t *testing.T) {
	// Hit starts at the very first byte of the body.
	line := "\x1b[48;5;236mhello\x1b[0m"
	got := SpliceStyle(line, "\x1b[48;5;58m", 0, 5)
	if !strings.HasPrefix(got, "\x1b[48;5;236m\x1b[48;5;58mhello") {
		t.Errorf("splice on should follow the leading parent escape; got %q", got)
	}
}

func TestSpliceStyleAcrossWrap_HitOnShortLine(t *testing.T) {
	// Short line (fits in one wrapped sub-line) — the splice is
	// applied to the single corresponding rendered row. The row
	// carries a 1-byte gutter (" "), so a hit on source bytes
	// [0, 5) of "bravo" maps to row body bytes [1, 6) of
	// "  bravo" = " brav".
	rendered := "  alpha\n  bravo\n  charlie\n"
	hits := []Hit{{LineIdx: 1, ByteA: 0, ByteC: 5}}
	lines := []string{"alpha", "bravo", "charlie"}
	sourceToFirst := []int{0, 1, 2}
	got := SpliceStyleAcrossWrap(rendered, lines, hits[0], "\x1b[48;5;58m", 80, sourceToFirst)
	row1 := strings.Split(got, "\n")[1]
	if !strings.Contains(row1, "\x1b[48;5;58m brav") {
		t.Errorf("expected dim on ' brav' (gutter + 'brav') on row 1; got %q", row1)
	}
}

func TestSpliceStyleAcrossWrap_HitCrossesWrapBoundary(t *testing.T) {
	// A line wraps at width 10; the hit spans the wrap boundary.
	// "the quick brown fox" wraps to:
	//   "the quick"   source bytes 0..8
	//   "brown fox"   source bytes 10..18 (wait, spaces take a column)
	// wrapLineWithRanges output for "the quick brown fox" @ 10:
	//   sub 0: "the quick"   range [0, 9)   (includes the trailing space — actually [0, 9) means chars 0..8)
	//   sub 1: "brown fox"   range [10, 19)
	// We use a hit on source bytes [5, 15) which crosses both subs.
	// Per sub-line, the splice covers [max(5, subStart), min(15, subEnd)).
	//   sub 0: max(5,0)=5, min(15,9)=9  → bytes 5..8 (" quic")
	//   sub 1: max(5,10)=10, min(15,19)=15  → bytes 0..5 of sub 1 = "brown" (full)
	source := "the quick brown fox"
	rendered := "the quick\nbrown fox\n"
	sourceToFirst := []int{0} // single source line
	hits := []Hit{{LineIdx: 0, ByteA: 5, ByteC: 15}}
	got := SpliceStyleAcrossWrap(rendered, []string{source}, hits[0], "\x1b[48;5;58m", 10, sourceToFirst)
	rows := strings.Split(got, "\n")
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows; got %d (%q)", len(rows), got)
	}
	// Row 0 should have a splice around " quic" (bytes 5..9 of source).
	if !strings.Contains(rows[0], "\x1b[48;5;58m") {
		t.Errorf("row 0 should contain the splice on; got %q", rows[0])
	}
	// Row 1 should have a splice around "brown" (bytes 10..15 of source).
	if !strings.Contains(rows[1], "\x1b[48;5;58m") {
		t.Errorf("row 1 should contain the splice on; got %q", rows[1])
	}
}

func TestSpliceStyleAcrossWrap_HitInsideCodeBlock(t *testing.T) {
	// Glamour-styled code block: the body is wrapped in
	// "\x1b[38;5;228;48;5;236m" + body + "\x1b[0m" with a 1-byte
	// gutter (" ") in front, matching the LineGutter output. A
	// hit on source bytes [6, 11) of "hello world" maps to row
	// body bytes [7, 12) of " " + body = " world" (the space is
	// part of the body, not the gutter). The parent escape must
	// be restored on close so the trailing cells keep bg 236.
	esc := "\x1b[38;5;228;48;5;236m"
	reset := "\x1b[0m"
	body := "hello world"
	// Build a 1-row rendered string with a 1-byte gutter and
	// the parent style wrapping the rest of the body.
	rendered := " " + esc + body + reset
	sourceToFirst := []int{0}
	hits := []Hit{{LineIdx: 0, ByteA: 6, ByteC: 11}} // "world"
	got := SpliceStyleAcrossWrap(rendered, []string{body}, hits[0], "\x1b[48;5;58m", 80, sourceToFirst)
	// Expected: gutter " ", open style, "hello ", on-58, "world",
	// restore-228-236, reset.
	want := " " + esc + "hello " + "\x1b[48;5;58m" + "world" + esc + reset
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSpliceStyleAcrossWrap_NoHitInLine(t *testing.T) {
	// Hit on a different line — should not change the rendered string.
	rendered := "  alpha\n  bravo\n  charlie\n"
	hits := []Hit{{LineIdx: 5, ByteA: 0, ByteC: 3}}
	got := SpliceStyleAcrossWrap(rendered, []string{"alpha", "bravo", "charlie"}, hits[0], "\x1b[48;5;58m", 80, []int{0, 1, 2})
	if got != rendered {
		t.Errorf("hit on a non-existent line should not change output; got %q", got)
	}
}
