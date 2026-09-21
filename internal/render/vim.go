package render

import "time"

// VimAction is the result of handling one vim key event.
type VimAction int

const (
	VimNoAction VimAction = iota
	VimLineDown
	VimLineUp
	VimNextBlock
	VimPrevBlock
	VimNextHeading
	VimPrevHeading
	VimGotoTop
	VimGotoBottom
)

// VimTimeout is the window for two-key sequences (gg, ]], [[). Matches
// vim's timeoutlen=500 default.
const VimTimeout = 500 * time.Millisecond

// VimState tracks the pending first key of a two-key sequence.
// All other fields are zero; treat the state as opaque from callers.
type VimState struct {
	LastKey     rune
	LastKeyTime time.Time
}

// Handle processes one key rune and returns the action to take. The
// time argument is injected for testability.
//
// Two-key completion: when LastKey is set and the new rune matches
// (gg, ]], or [[), the combined action fires and state clears. On any
// other rune, the second rune is processed as a fresh single-key
// action (mismatched keys fall through).
func (s *VimState) Handle(r rune, now time.Time) VimAction {
	// Clear stale state first.
	if s.LastKey != 0 && now.Sub(s.LastKeyTime) > VimTimeout {
		s.LastKey = 0
	}

	prev := s.LastKey
	s.LastKey = 0

	switch {
	case prev == 'g' && r == 'g':
		return VimGotoTop
	case prev == ']' && r == ']':
		return VimNextHeading
	case prev == '[' && r == '[':
		return VimPrevHeading
	}

	switch r {
	case 'j':
		return VimLineDown
	case 'k':
		return VimLineUp
	case '}':
		return VimNextBlock
	case '{':
		return VimPrevBlock
	case 'G':
		return VimGotoBottom
	case ']', '[', 'g':
		s.LastKey = r
		s.LastKeyTime = now
	}
	return VimNoAction
}
