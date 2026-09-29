package main

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestWordAtCursor_FindsIdentifier: cursor in the middle of an
// identifier returns the byte offset of the identifier's start.
// Spec scenario: cursor in middle of an identifier.
func TestWordAtCursor_FindsIdentifier(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("foo bar baz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := openFileInFixture(t, root, "main.go")
	// File: "foo bar baz\n" — bytes: f=0, o=1, o=2, ' '=3,
	// b=4, a=5, r=6, ' '=7, b=8, a=9, z=10.
	// Move cursor to byte 6 (the 'r' in "bar") by pressing l 6 times.
	for range 6 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.charPos != 6 {
		t.Fatalf("setup: charPos = %d; want 6", m.fileViewer.charPos)
	}
	line, char := m.wordAtCursor()
	if line != 1 || char != 4 {
		t.Errorf("wordAtCursor at (1, 6) = (%d, %d); want (1, 4) (start of 'bar')", line, char)
	}
}

// TestWordAtCursor_OnWhitespaceReturnsCharPos: cursor on a space
// or punctuation byte returns charPos unchanged so LSP can
// return 0 results silently. Spec scenario: cursor on whitespace.
func TestWordAtCursor_OnWhitespaceReturnsCharPos(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("foo bar baz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := openFileInFixture(t, root, "main.go")
	// Move cursor to byte 3 (the space).
	for range 3 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	if m.fileViewer.charPos != 3 {
		t.Fatalf("setup: charPos = %d; want 3", m.fileViewer.charPos)
	}
	line, char := m.wordAtCursor()
	if line != 1 || char != 3 {
		t.Errorf("wordAtCursor at (1, 3) [space] = (%d, %d); want (1, 3) (charPos unchanged)", line, char)
	}
}

// TestWordAtCursor_AtStartOfLine: cursor at byte 0 of an
// identifier returns byte 0. Spec scenario: cursor at start of
// line / start of word.
func TestWordAtCursor_AtStartOfLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("foo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := openFileInFixture(t, root, "main.go")
	if m.fileViewer.charPos != 0 {
		t.Fatalf("setup: charPos = %d; want 0", m.fileViewer.charPos)
	}
	line, char := m.wordAtCursor()
	if line != 1 || char != 0 {
		t.Errorf("wordAtCursor at (1, 0) = (%d, %d); want (1, 0)", line, char)
	}
}

// TestWordAtCursor_AtEndOfLine: cursor at the line end returns
// charPos unchanged (nothing to find).
func TestWordAtCursor_AtEndOfLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("foo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := openFileInFixture(t, root, "main.go")
	for range 7 {
		upd, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = updatedModelPtr(upd)
	}
	line, char := m.wordAtCursor()
	if line != 1 || char != 7 {
		t.Errorf("wordAtCursor at (1, 7) = (%d, %d); want (1, 7) (line end, unchanged)", line, char)
	}
}

// TestIsWordByte: sanity check on the alphabet boundary.
func TestIsWordByte(t *testing.T) {
	wordChars := []byte{'a', 'z', 'A', 'Z', '0', '9', '_'}
	for _, b := range wordChars {
		if !isWordByte(b) {
			t.Errorf("isWordByte(%q) = false; want true", b)
		}
	}
	nonWordChars := []byte{' ', '\t', '\n', '.', ',', '(', ')', '[', ']', '{', '}', '!', '@', '#', '$', '%', '^', '&', '*', '+', '-', '/', '<', '>', '?', ':', ';', '"', '\'', '`', '|', '\\'}
	for _, b := range nonWordChars {
		if isWordByte(b) {
			t.Errorf("isWordByte(%q) = true; want false", b)
		}
	}
}
