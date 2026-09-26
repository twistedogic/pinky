package main

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
)

// initCommentTAForTest mirrors m.initCommentComposer() so tests
// that bypass attach() still have a usable comment textarea.
func initCommentTAForTest(m *model) {
	ta := textarea.New()
	ta.Placeholder = "comment — Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	got := updated.(model)
	if got.state != stateCommentComposer {
		t.Errorf("state = %d want stateCommentComposer", got.state)
	}
	if got.commentAnchor.blockIdx < 0 {
		t.Errorf("expected commentAnchor.blockIdx >= 0; got %d", got.commentAnchor.blockIdx)
	}
	if got.commentAnchor.charA != -1 || got.commentAnchor.charC != -1 {
		t.Errorf("block-level anchor should have charA=charC=-1; got (%d,%d)",
			got.commentAnchor.charA, got.commentAnchor.charC)
	}
}

// TestNavKey_V_EntersVisual: pressing `v` enters visual line mode.
func TestNavKey_V_EntersVisual(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	got := updated.(model)
	if got.nav.Visual != render.NavLine {
		t.Fatal("first v should enter visual")
	}
	updated2, _ := got.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	got := updated.(model)
	if got.nav.Visual != render.NavLine {
		t.Fatal("v should enter visual")
	}
	updated2, _ := got.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
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

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got := updated.(model)
	if got.cursor.BlockIdx == 0 {
		t.Errorf("j should advance cursor blockIdx; still at 0")
	}
	updated2, _ := got.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	got := updated.(model)
	if got.cursor.CharPos != 1 {
		t.Errorf("l: CharPos = %d want 1", got.cursor.CharPos)
	}
	updated2, _ := got.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
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

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
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

	_, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if called {
		t.Error("s with no comments should not call inject")
	}
}

// TestCommentComposer_EnterSavesAndExits: pressing Enter in the
// comment composer saves the new comment and returns to nav
// without dispatching anything. Flush happens only via `s` in nav.
func TestCommentComposer_EnterSavesAndExits(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	var sentCalls int
	prev := sendToPane
	sendToPane = func(_, _ string) error { sentCalls++; return nil }
	t.Cleanup(func() { sendToPane = prev })

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(model)
	if m.state != stateCommentComposer {
		t.Fatalf("setup: state = %d want stateCommentComposer", m.state)
	}
	m.commentTa.SetValue("rename to foo")

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := updated.(model)
	if got.state != stateNav {
		t.Errorf("state = %d want stateNav after Enter", got.state)
	}
	if sentCalls != 0 {
		t.Errorf("sendToPane calls = %d want 0 (Enter must not dispatch)", sentCalls)
	}
	if len(got.comments) != 1 {
		t.Fatalf("comments = %d want 1 (Enter must save)", len(got.comments))
	}
	if got.comments[0].Text != "rename to foo" {
		t.Errorf("saved comment text = %q want %q", got.comments[0].Text, "rename to foo")
	}
	if got.comments[0].BlockIdx < 0 || got.comments[0].BlockIdx >= len(got.blocks) {
		t.Errorf("saved comment anchor BlockIdx = %d out of range", got.comments[0].BlockIdx)
	}
}

// TestCommentComposer_MultipleCommentsAccumulate: two `c → type →
// Enter` cycles accumulate both comments in memory without firing
// any inject; flush is the user's separate action.
func TestCommentComposer_MultipleCommentsAccumulate(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# title\n\nbody"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	var sentCalls int
	prev := sendToPane
	sendToPane = func(_, _ string) error { sentCalls++; return nil }
	t.Cleanup(func() { sendToPane = prev })

	// First comment
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(model)
	m.commentTa.SetValue("fix the title")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)

	// Second comment
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(model)
	m.commentTa.SetValue("expand the body")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := updated.(model)

	if sentCalls != 0 {
		t.Errorf("sendToPane calls = %d want 0 (Enter must not dispatch)", sentCalls)
	}
	if len(got.comments) != 2 {
		t.Fatalf("comments = %d want 2 (accumulated in memory)", len(got.comments))
	}
	if got.comments[0].Text != "fix the title" || got.comments[1].Text != "expand the body" {
		t.Errorf("comments out of order: %q, %q", got.comments[0].Text, got.comments[1].Text)
	}
	if got.state != stateNav {
		t.Errorf("state = %d want stateNav", got.state)
	}
}

// TestCommentComposer_EmptyEnterIsNoop: pressing Enter with an empty
// composer returns to nav without saving a comment or firing any
// inject — covered by saveComment's empty-text guard.
func TestCommentComposer_EmptyEnterIsNoop(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	var sentCalls int
	prev := sendToPane
	sendToPane = func(_, _ string) error { sentCalls++; return nil }
	t.Cleanup(func() { sendToPane = prev })

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(model)
	if m.state != stateCommentComposer {
		t.Fatalf("setup: state = %d want stateCommentComposer", m.state)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := updated.(model)
	if got.state != stateNav {
		t.Errorf("state = %d want stateNav", got.state)
	}
	if len(got.comments) != 0 {
		t.Errorf("comments = %d want 0 (empty Enter must not save)", len(got.comments))
	}
	if sentCalls != 0 {
		t.Errorf("sendToPane calls = %d want 0", sentCalls)
	}
}

// TestCommentComposer_EnterDoesNotInsertNewline: Enter in the
// single-line composer commits the buffer verbatim — no newline
// inserted by the keypress itself. (Checks the saved comment text,
// because Enter no longer dispatches.)
func TestCommentComposer_EnterDoesNotInsertNewline(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "# Hello\n\nbody"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(model)
	m.commentTa.SetValue("plain comment")

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := updated.(model)
	if len(got.comments) != 1 {
		t.Fatalf("comments = %d want 1", len(got.comments))
	}
	if strings.Contains(got.comments[0].Text, "\n") {
		t.Errorf("saved comment must not contain an embedded newline; got %q", got.comments[0].Text)
	}
	if got.comments[0].Text != "plain comment" {
		t.Errorf("saved comment text = %q want %q", got.comments[0].Text, "plain comment")
	}
	if got.state != stateNav {
		t.Errorf("state = %d want stateNav", got.state)
	}
}

// (TestCommentComposer_EnterSendsMultipleInOneCall removed: under
// the new contract, Enter no longer dispatches. Multi-comment
// accumulation is covered by TestCommentComposer_MultipleCommentsAccumulate;
// flushing the accumulated batch via `s` is covered by
// TestSubmitComments_ClearsOnSuccess and TestNavKey_S_NavWithCommentsSends.)

// TestCompose_I_TogglesIncludeComments: regression for
// single-key-keymap; pressing `i` in compose toggles the
// include-comments flag (replaces the prior Ctrl+I).
func TestCompose_I_TogglesIncludeComments(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.enterCompose()
	if m.includeComments {
		t.Fatalf("setup: includeComments should start false")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(model)
	if !m.includeComments {
		t.Errorf("includeComments = false want true after first `i`")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(model)
	if m.includeComments {
		t.Errorf("includeComments = true want false after second `i`")
	}
}

// TestNavKey_BlockKindWholeBlockUsesBlockMarker: pressing `c` in
// stateNav (no visual selection) produces a whole-block comment
// that flushes with marker=`block`, not marker=`inline`. Same
// root-cause shape as the file-inline "" bug — the commentAnchor
// was defaulting charA to 0, falsely registering as inline byte-0.
func TestNavKey_BlockKindWholeBlockUsesBlockMarker(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.latest = session.Message{Role: session.RoleAssistant, Text: "alpha block"}
	m.refreshViewport()
	initCommentTAForTest(&m)

	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = upd.(model)
	if m.commentAnchor.charA != -1 || m.commentAnchor.charC != -1 {
		t.Fatalf("whole-block anchor should have charA=charC=-1; got (%d,%d)",
			m.commentAnchor.charA, m.commentAnchor.charC)
	}

	m.commentTa.SetValue("block note")
	upd, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = upd.(model)

	prev := sendToPane
	var sent string
	sendToPane = func(_, text string) error { sent = text; return nil }
	t.Cleanup(func() { sendToPane = prev })
	upd, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = upd.(model)

	if !strings.Contains(sent, "- block ") || strings.Contains(sent, "inline") {
		t.Errorf("appendix should use `block` marker for whole-block; payload:\n%s", sent)
	}
}
