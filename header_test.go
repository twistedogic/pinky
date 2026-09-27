package main

import "testing"

// TestShortenCwd covers the abbreviate-to-~/last-N-segments helper
// across the empty / single-segment / multi-segment / leading-slash /
// already-tilde cases.
func TestShortenCwd(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		maxSegs int
		want    string
	}{
		{"empty", "", 2, ""},
		{"single segment kept", "foo", 2, "foo"},
		{"two segments kept", "foo/bar", 2, "foo/bar"},
		{"three segments abbreviates to last two", "foo/bar/baz", 2, "~/bar/baz"},
		{"leading slash stripped", "/Users/a012/Dev/pinky", 2, "~/Dev/pinky"},
		{"already-tilde prefix kept", "~/Dev/pinky", 2, "~/Dev/pinky"},
		{"maxSegs=1 keeps last one", "/a/b/c/d", 1, "~/d"},
		{"maxSegs=2 with four segments keeps last two", "/a/b/c/d", 2, "~/c/d"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shortenCwd(c.path, c.maxSegs)
			if got != c.want {
				t.Errorf("shortenCwd(%q, %d) = %q, want %q", c.path, c.maxSegs, got, c.want)
			}
		})
	}
}

// TestSplitOnSlash covers the greedy-prefix-on-`/` helper used as
// the header's last-resort wrap.
func TestSplitOnSlash(t *testing.T) {
	cases := []struct {
		name  string
		s     string
		max   int
		wantA string
		wantB string
	}{
		{"shorter than max no split", "abc/def", 10, "abc/def", ""},
		{"single segment longer than max returns original", "longpathnomid", 5, "longpathnomid", ""},
		{"multi-segment splits at slash boundary", "abc/def/ghi", 7, "abc/def", "ghi"},
		{"multi-segment where first fits but second would overflow", "abc/def/ghi", 5, "abc", "def/ghi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, b := splitOnSlash(c.s, c.max)
			if a != c.wantA || b != c.wantB {
				t.Errorf("splitOnSlash(%q, %d) = (%q, %q), want (%q, %q)",
					c.s, c.max, a, b, c.wantA, c.wantB)
			}
		})
	}
}
