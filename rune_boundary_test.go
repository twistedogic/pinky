package main

import (
	"testing"

	"github.com/twistedogic/pinky/internal/render"
)

// TestClampToRuneBoundary_ASCIIPassthrough: ASCII-only lines have
// every byte on a rune boundary, so any pos in [0, len] returns
// itself. Spec scenario: rune boundary preservation on l/j for
// the ASCII case (the degenerate case).
func TestClampToRuneBoundary_ASCIIPassthrough(t *testing.T) {
	line := "alpha beta"
	tests := []struct {
		pos, want int
	}{
		{0, 0},
		{3, 3},
		{5, 5},
		{9, 9},
		{len(line), len(line)},
		{len(line) + 5, len(line)},
		{-1, 0},
	}
	for _, tc := range tests {
		got := render.SnapToRuneStart(line, tc.pos)
		if got != tc.want {
			t.Errorf("render.SnapToRuneStart(%q, %d) = %d; want %d", line, tc.pos, got, tc.want)
		}
	}
}

// TestClampToRuneBoundary_EmptyLine: empty string returns pos
// clamped to 0 (the only valid offset).
func TestClampToRuneBoundary_EmptyLine(t *testing.T) {
	if got := render.SnapToRuneStart("", 0); got != 0 {
		t.Errorf("render.SnapToRuneStart(%q, 0) = %d; want 0", "", got)
	}
	if got := render.SnapToRuneStart("", 5); got != 0 {
		t.Errorf("render.SnapToRuneStart(%q, 5) = %d; want 0", "", got)
	}
}

// TestClampToRuneBoundary_MiddleOfRuneSnapsBack: "é" is two bytes
// (0xC3 0xA9); pos=1 falls inside the rune and must snap back to
// 0 (the rune's start).
func TestClampToRuneBoundary_MiddleOfRuneSnapsBack(t *testing.T) {
	line := "é"
	if got := render.SnapToRuneStart(line, 0); got != 0 {
		t.Errorf("render.SnapToRuneStart(%q, 0) = %d; want 0", line, got)
	}
	if got := render.SnapToRuneStart(line, 1); got != 0 {
		t.Errorf("render.SnapToRuneStart(%q, 1) = %d; want 0 (rune start)", line, got)
	}
	if got := render.SnapToRuneStart(line, 2); got != 2 {
		t.Errorf("render.SnapToRuneStart(%q, 2) = %d; want 2 (line end)", line, got)
	}
}

// TestClampToRuneBoundary_MixedLine: a line with ASCII and
// multi-byte runes (Japanese + emoji). pos values that land on
// rune starts return unchanged; values inside runes snap back.
func TestClampToRuneBoundary_MixedLine(t *testing.T) {
	// "a日b🎉" — bytes: 61 E6 97 A5 62 F0 9F 8E 89
	// offsets:    0  1  2  3  4  5  6  7  8
	line := "a日b🎉"
	if len(line) != 9 {
		t.Fatalf("test setup: line byte length = %d; want 9", len(line))
	}
	tests := []struct {
		pos, want int
	}{
		{0, 0},   // start of 'a'
		{1, 1},   // start of '日' (3-byte rune)
		{2, 1},   // mid '日'
		{3, 1},   // mid '日'
		{4, 4},   // start of 'b'
		{5, 5},   // start of '🎉' (4-byte rune)
		{6, 5},   // mid '🎉'
		{7, 5},   // mid '🎉'
		{8, 5},   // mid '🎉'
		{9, 9},   // line end
	}
	for _, tc := range tests {
		got := render.SnapToRuneStart(line, tc.pos)
		if got != tc.want {
			t.Errorf("render.SnapToRuneStart(%q, %d) = %d; want %d", line, tc.pos, got, tc.want)
		}
	}
}

// TestClampToRuneBoundary_AtLineEnd: pos == len(line) is always a
// valid rune boundary (the position one past the last byte).
func TestClampToRuneBoundary_AtLineEnd(t *testing.T) {
	for _, line := range []string{"a", "é", "abc", "日本語"} {
		got := render.SnapToRuneStart(line, len(line))
		if got != len(line) {
			t.Errorf("render.SnapToRuneStart(%q, len) = %d; want %d", line, got, len(line))
		}
	}
}
