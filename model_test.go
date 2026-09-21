package main

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
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

// TestNavKey_C_NoSelectionOpensBlockComposer: pressing `c` in nav
// with no selection opens the comment composer pre-anchored to the
// current block (per design D5 — the old `m` key is gone).
func TestNavKey_C_NoSelectionOpensBlockComposer(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := updated.(model)
	if got.state != stateCommentComposer {
		t.Errorf("state = %d want stateCommentComposer", got.state)
	}
	if got.commentAnchor.blockIdx < 0 {
		t.Errorf("expected commentAnchor.blockIdx >= 0; got %d", got.commentAnchor.blockIdx)
	}
	if got.commentAnchor.charA != 0 {
		t.Errorf("block-level anchor should start at 0; got %d", got.commentAnchor.charA)
	}
}

// TestNavKey_V_EntersVisual: pressing `v` enters visual line mode.
func TestNavKey_V_EntersVisual(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	initCommentTAForTest(&m)
	got := updated.(model)
	if got.nav.Visual != render.NavLine {
		t.Errorf("nav.Visual = %v want NavLine", got.nav.Visual)
	}
}

// TestNavKey_V_ThenV_Exits: pressing `v` twice toggles visual mode
// off (round-trip — single source of truth on the visual flag).
func TestNavKey_V_ThenV_Exits(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := updated.(model)
	if got.nav.Visual != render.NavLine {
		t.Fatal("first v should enter visual")
	}
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got2 := updated2.(model)
	if got2.nav.Visual != render.NavNone {
		t.Errorf("second v should exit visual; got %v", got2.nav.Visual)
	}
}

// TestNavKey_V_ThenEsc_Exits: visual mode's Esc path.
func TestNavKey_V_ThenEsc_Exits(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := updated.(model)
	if got.nav.Visual != render.NavLine {
		t.Fatal("v should enter visual")
	}
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got2 := updated2.(model)
	if got2.nav.Visual != render.NavNone {
		t.Errorf("Esc in visual should exit; got %v", got2.nav.Visual)
	}
}

// TestNavKey_EscOutsideVisualIsNoop: Esc in nav (no visual) is a
// no-op — visual mode's only Esc behaviour.
func TestNavKey_EscOutsideVisualIsNoop(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if got.state != stateNav {
		t.Errorf("Esc outside visual should leave state in nav; got %d", got.state)
	}
	if cmd != nil {
		t.Errorf("Esc outside visual should not produce a cmd; got %v", cmd)
	}
}

// TestNavKey_JK_MovesBlockCursor: `j`/`k` move the block cursor.
// Per design D1 the cursor drives both the visual highlight and the
// viewport scroll, so this is the single-source-of-truth path.
func TestNavKey_JK_MovesBlockCursor(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# A\n\np1\n\n## B\n\np2"}
	m.refreshViewport()
	if len(m.blocks) < 3 {
		t.Fatalf("setup: expected >=3 blocks, got %d", len(m.blocks))
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(model)
	if got.cursor.BlockIdx == 0 {
		t.Errorf("j should advance cursor blockIdx; still at 0")
	}
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got2 := updated2.(model)
	if got2.cursor.BlockIdx != 0 {
		t.Errorf("k should restore cursor to block 0; got %d", got2.cursor.BlockIdx)
	}
}

// TestNavKey_HL_MovesCharCursor: `h`/`l` move the byte cursor
// within the block.
func TestNavKey_HL_MovesCharCursor(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hi\n\nabc def ghi"}
	m.refreshViewport()
	// Park the cursor on a multi-byte paragraph.
	m.cursor.BlockIdx = 1
	m.cursor.CharPos = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := updated.(model)
	if got.cursor.CharPos != 1 {
		t.Errorf("l: CharPos = %d want 1", got.cursor.CharPos)
	}
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got2 := updated2.(model)
	if got2.cursor.CharPos != 0 {
		t.Errorf("h: CharPos = %d want 0", got2.cursor.CharPos)
	}
}

// TestNavKey_R_TriggersPoll: `r` triggers a poll (was Ctrl+R).
func TestNavKey_R_TriggersPoll(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.src = &fakeSource{}
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello"}
	m.refreshViewport()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("r: expected poll cmd")
	}
}

// TestNavKey_C_WithSelection_AnchorsSelection: pressing `c` while a
// selection is active anchors the composer to the selection range
// (per design D5 — was line-range, now byte-range).
func TestNavKey_C_WithSelection_AnchorsSelection(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nabcdef"}
	m.refreshViewport()
	initCommentTAForTest(&m)
	// Seed a selection on block 1 bytes [2, 5].
	m.nav.Visual = render.NavLine
	m.selection = render.NavSelection{BlockIdx: 1, CharA: 2, CharC: 5}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := updated.(model)
	if got.state != stateCommentComposer {
		t.Errorf("state = %d want stateCommentComposer", got.state)
	}
	if got.commentAnchor.charA != 2 || got.commentAnchor.charC != 5 {
		t.Errorf("anchor = (%d,%d) want (2,5)", got.commentAnchor.charA, got.commentAnchor.charC)
	}
}

// TestNavKey_S_NavWithCommentsSends: `s` in nav with comments
// batch-sends (universal send per design D4).
func TestNavKey_S_NavWithCommentsSends(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# title\n\nbody"}
	m.refreshViewport()
	m.comments = []render.Comment{
		{BlockIdx: 0, Text: "fix the title", CreatedAt: time.Now()},
	}

	var sentText string
	prev := sendToPane
	sendToPane = func(_, text string) error { sentText = text; return nil }
	t.Cleanup(func() { sendToPane = prev })

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(model)
	if len(got.comments) != 0 {
		t.Errorf("comments should clear after successful send; got %d", len(got.comments))
	}
	if sentText == "" {
		t.Error("expected sendToPane to be called")
	}
}

// TestNavKey_S_NavWithoutCommentsIsNoop: `s` in nav with no comments
// is a no-op (no inject call).
func TestNavKey_S_NavWithoutCommentsIsNoop(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello"}
	m.refreshViewport()

	called := false
	prev := sendToPane
	sendToPane = func(_, _ string) error { called = true; return nil }
	t.Cleanup(func() { sendToPane = prev })

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if called {
		t.Error("s with no comments should not call inject")
	}
}
