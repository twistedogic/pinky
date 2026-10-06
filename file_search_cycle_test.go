package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestFileSearch_NCyclesAndResetsPreferred: typing "alpha" and
// Enter, then n, cycles to the next "alpha" hit and resets
// preferred to the hit's charPos.
func TestFileSearch_NCyclesAndResetsPreferred(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta alpha\nalpha gamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// First hit should be line 1 (1-based), byte 0.
	if m.fileViewer.cursor != 1 || m.fileViewer.charPos != 0 {
		t.Fatalf("after Enter: cursor=(%d, %d), want (1, 0)", m.fileViewer.cursor, m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 0 {
		t.Errorf("preferred = %d, want 0", m.fileViewer.preferred)
	}
	// n: advance to second "alpha" hit. With the greedy
	// subsequence matcher, the second hit on line 2 starts at
	// the 'a' of "beta" (byte 3) and runs through "lpha" from
	// the second "alpha" — the start byte is 3, not 5.
	m = keyModelVal(t, m, "n")
	if m.fileViewer.cursor != 2 {
		t.Errorf("after n: cursor = %d, want 2", m.fileViewer.cursor)
	}
	if m.fileViewer.charPos != 3 {
		t.Errorf("after n: charPos = %d, want 3 (greedy: 'a' of 'beta' + 'lpha')", m.fileViewer.charPos)
	}
	if m.fileViewer.preferred != 3 {
		t.Errorf("after n: preferred = %d, want 3 (reset to hit)", m.fileViewer.preferred)
	}
	// n: advance to third "alpha" at line 3, byte 0.
	m = keyModelVal(t, m, "n")
	if m.fileViewer.cursor != 3 {
		t.Errorf("after second n: cursor = %d, want 3", m.fileViewer.cursor)
	}
	if m.fileViewer.preferred != 0 {
		t.Errorf("after second n: preferred = %d, want 0", m.fileViewer.preferred)
	}
}

// TestFileSearch_NCyclesWrapsForward: from the last hit, n
// wraps to the first.
func TestFileSearch_NCyclesWrapsForward(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta alpha\nalpha gamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// Two more n's to reach the last (3rd) hit.
	m = keyModelVal(t, m, "n")
	m = keyModelVal(t, m, "n")
	if m.fileSearchState.cur != 2 {
		t.Fatalf("setup: cur=%d, want 2", m.fileSearchState.cur)
	}
	// n: wrap to 0.
	m = keyModelVal(t, m, "n")
	if m.fileSearchState.cur != 0 {
		t.Errorf("after wrap: cur=%d, want 0", m.fileSearchState.cur)
	}
}

// TestFileSearch_NCyclesWrapsBack: N from the first hit wraps
// to the last.
func TestFileSearch_NCyclesWrapsBack(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta alpha\nalpha gamma\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	upd, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)
	// cur=0; N wraps to 2.
	m = keyModelVal(t, m, "N")
	if m.fileSearchState.cur != 2 {
		t.Errorf("after N: cur=%d, want 2", m.fileSearchState.cur)
	}
}
