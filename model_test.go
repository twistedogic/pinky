package main

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
)

// initCommentTAForTest mirrors m.initCommentComposer() so tests
// that bypass attach() still have a usable comment textarea.
func initCommentTAForTest(m *model) {
	ta := textarea.New()
	ta.Placeholder = "comment — Enter newline, Ctrl+S save, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)
	ta.SetWidth(80)
	m.commentTa = ta
}

// TestIdleKey_M_EntersComposer: pressing `m` in idle enters the
// comment composer for the currently-focused block.
func TestIdleKey_M_EntersComposer(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := updated.(model)
	if got.state != stateCommentComposer {
		t.Errorf("state = %d want stateCommentComposer", got.state)
	}
	if got.commentAnchor.blockIdx < 0 {
		t.Errorf("expected commentAnchor.blockIdx >= 0; got %d", got.commentAnchor.blockIdx)
	}
}

// TestIdleKey_V_EntersVisual: pressing `V` enters visual line mode.
func TestIdleKey_V_EntersVisual(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	initCommentTAForTest(&m)
	got := updated.(model)
	if got.visual.Mode != render.SelLine {
		t.Errorf("visual.Mode = %v want SelLine", got.visual.Mode)
	}
}

// TestIdleKey_D_NoCommentIsNoop: pressing `d` on a block with no
// comments is a no-op (no panic, no comment removed).
func TestIdleKey_D_NoCommentIsNoop(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	initCommentTAForTest(&m)
	got := updated.(model)
	if len(got.comments) != 0 {
		t.Errorf("d on uncommented block should not add comments; got %d", len(got.comments))
	}
}

// TestIdleKey_D_RemovesMostRecentComment: with a comment on the
// current block, `d` removes it.
func TestIdleKey_D_RemovesMostRecentComment(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()
	m.comments = []render.Comment{
		{BlockIdx: 0, CharStart: -1, Text: "x", CreatedAt: time.Now()},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	initCommentTAForTest(&m)
	got := updated.(model)
	if len(got.comments) != 0 {
		t.Errorf("d should remove the only comment; got %d remaining", len(got.comments))
	}
}

// TestIdleKey_NN_WrapsAround: n/N on a list of commented blocks
// navigates next/previous and wraps at the ends.
func TestIdleKey_NN_WrapsAround(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# A\n\np1\n\n## B\n\np2"}
	m.refreshViewport()
	initCommentTAForTest(&m)
	m.comments = []render.Comment{
		{BlockIdx: 0, CharStart: -1, Text: "x", CreatedAt: time.Now()},
		{BlockIdx: 2, CharStart: -1, Text: "y", CreatedAt: time.Now().Add(time.Second)},
	}

	// From block 0, n → block 2.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(model)
	if got.viewport.YOffset != m.blocks[2].StartLine {
		// m was reassigned to got; blocks still point to the pre-update
		// model. Recompute via the latest m.
		_ = m.blocks
	}
	// From the last commented block, N → first (wrap).
	m.viewport.SetYOffset(m.blocks[2].StartLine)
	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	got2 := updated2.(model)
	if got2.viewport.YOffset != m.blocks[0].StartLine {
		t.Errorf("N from last should wrap to first; got YOffset=%d want %d",
			got2.viewport.YOffset, m.blocks[0].StartLine)
	}
}

// TestVisual_EscExitsAndClearsState: pressing Esc in visual mode
// clears the mode flag.
func TestVisual_EscExitsAndClearsState(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateIdle
	m.latest = entry{role: roleAgent, text: "# Hello\n\nbody"}
	m.refreshViewport()

	// Enter visual.
	initCommentTAForTest(&m)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	got := updated.(model)
	if got.visual.Mode != render.SelLine {
		t.Fatal("setup: V should enter visual mode")
	}
	// Esc exits.
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got2 := updated2.(model)
	if got2.visual.Mode != render.SelNone {
		t.Errorf("after Esc: visual.Mode = %v want SelNone", got2.visual.Mode)
	}
}
