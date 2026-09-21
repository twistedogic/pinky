package render

import (
	"testing"
	"time"
)

// vimTime is a fixed reference time for deterministic tests.
var vimTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestVim_LineKeys(t *testing.T) {
	var s VimState
	cases := []struct {
		key  rune
		want VimAction
	}{
		{'j', VimLineDown},
		{'k', VimLineUp},
		{'}', VimNextBlock},
		{'{', VimPrevBlock},
		{'G', VimGotoBottom},
	}
	for _, c := range cases {
		s = VimState{} // reset
		if got := s.Handle(c.key, vimTime); got != c.want {
			t.Errorf("key=%q got=%v want=%v", c.key, got, c.want)
		}
		if s.LastKey != 0 {
			t.Errorf("key=%q left state=%q (want cleared)", c.key, s.LastKey)
		}
	}
}

func TestVim_GG(t *testing.T) {
	var s VimState
	if got := s.Handle('g', vimTime); got != VimNoAction {
		t.Errorf("first g: got %v want VimNoAction", got)
	}
	if s.LastKey != 'g' {
		t.Errorf("first g: LastKey=%q want %q", s.LastKey, 'g')
	}
	// Second g within timeout → GotoTop.
	if got := s.Handle('g', vimTime.Add(100*time.Millisecond)); got != VimGotoTop {
		t.Errorf("second g: got %v want VimGotoTop", got)
	}
	if s.LastKey != 0 {
		t.Errorf("second g: LastKey=%q want cleared", s.LastKey)
	}
}

func TestVim_HeadingKeys(t *testing.T) {
	var s VimState
	// ]] sequence.
	if got := s.Handle(']', vimTime); got != VimNoAction {
		t.Errorf("first ]: got %v want VimNoAction", got)
	}
	if got := s.Handle(']', vimTime.Add(50*time.Millisecond)); got != VimNextHeading {
		t.Errorf("second ]: got %v want VimNextHeading", got)
	}
	// [[ sequence.
	s = VimState{}
	if got := s.Handle('[', vimTime); got != VimNoAction {
		t.Errorf("first [: got %v want VimNoAction", got)
	}
	if got := s.Handle('[', vimTime.Add(50*time.Millisecond)); got != VimPrevHeading {
		t.Errorf("second [: got %v want VimPrevHeading", got)
	}
}

func TestVim_TwoKeyTimeout(t *testing.T) {
	var s VimState
	// First g at vimTime.
	if got := s.Handle('g', vimTime); got != VimNoAction {
		t.Fatalf("first g: got %v want VimNoAction", got)
	}
	// Second g AFTER timeout → state was cleared, so this is treated as a fresh first g.
	if got := s.Handle('g', vimTime.Add(600*time.Millisecond)); got != VimNoAction {
		t.Errorf("timeout g: got %v want VimNoAction", got)
	}
	if s.LastKey != 'g' {
		t.Errorf("timeout g: LastKey=%q want %q (fresh first g)", s.LastKey, 'g')
	}
}

func TestVim_TimeoutBoundary(t *testing.T) {
	var s VimState
	if got := s.Handle('g', vimTime); got != VimNoAction {
		t.Fatalf("first g: got %v", got)
	}
	// Exactly 500ms later — within timeout per "now.Sub > timeout" semantics.
	halfSec := vimTime.Add(500 * time.Millisecond)
	if got := s.Handle('g', halfSec); got != VimGotoTop {
		t.Errorf("at 500ms: got %v want VimGotoTop", got)
	}
}

func TestVim_NonMatchingSecondKey(t *testing.T) {
	var s VimState
	if got := s.Handle('g', vimTime); got != VimNoAction {
		t.Fatalf("first g: got %v", got)
	}
	// j within timeout — mismatch. Should be processed as fresh j.
	if got := s.Handle('j', vimTime.Add(100*time.Millisecond)); got != VimLineDown {
		t.Errorf("g then j: got %v want VimLineDown", got)
	}
	if s.LastKey != 0 {
		t.Errorf("g then j: LastKey=%q want cleared", s.LastKey)
	}
}

func TestVim_MismatchedBracketFallsThrough(t *testing.T) {
	var s VimState
	if got := s.Handle('g', vimTime); got != VimNoAction {
		t.Fatalf("first g: got %v", got)
	}
	// ] within timeout — mismatch. Should start a new pending state for ].
	if got := s.Handle(']', vimTime.Add(50*time.Millisecond)); got != VimNoAction {
		t.Errorf("g then ]: got %v want VimNoAction", got)
	}
	if s.LastKey != ']' {
		t.Errorf("g then ]: LastKey=%q want %q", s.LastKey, ']')
	}
	// Now ] completes → NextHeading.
	if got := s.Handle(']', vimTime.Add(100*time.Millisecond)); got != VimNextHeading {
		t.Errorf("] then ]: got %v want VimNextHeading", got)
	}
}
