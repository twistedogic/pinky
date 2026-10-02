package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestCommentComposer_QDismisses: pressing q inside the comment
// composer must cancel (mirror the hover modal's q-handling) —
// otherwise muscle memory closes the wrong surface. The literal
// character `q` as the first letter of a comment is sacrificed for
// the much more common case: "open, rethink, dismiss".
func TestCommentComposer_QDismisses(t *testing.T) {
	m := openCommentComposerForTest(t)

	upd, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = updatedModelPtr(upd)

	if m.state == stateCommentComposer {
		t.Errorf("q should dismiss the comment composer; still in stateCommentComposer")
	}
	if cmd != nil {
		t.Errorf("dismiss must not return a cmd (quit path fired? got %T)", cmd)
	}
	if got := m.commentTa.Value(); got != "" {
		t.Errorf("dismiss should not retain typed text; got %q", got)
	}
}

// TestCommentComposer_EscDismissesAlreadyCovered: regression — Esc
// was the original cancel; this asserts the equivalent q-path leaves
// the same model state (stateNav, no saved comment, no leftover
// textarea text).
func TestCommentComposer_EscAndQDismissesEquivalently(t *testing.T) {
	m1 := openCommentComposerForTest(t)
	upd, _ := m1.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m1 = updatedModelPtr(upd)

	m2 := openCommentComposerForTest(t)
	upd, _ = m2.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m2 = updatedModelPtr(upd)

	if m1.state != m2.state {
		t.Errorf("Esc and q should land on same state; Esc=%d, Q=%d", m1.state, m2.state)
	}
	if len(m1.comments) != 0 || len(m2.comments) != 0 {
		t.Errorf("dismiss should not save a comment; Esc=%d Q=%d", len(m1.comments), len(m2.comments))
	}
}

// TestCommentComposer_QDoesNotSaveAsCommentText: q cancels, but
// sanity-check the textarea is also cleared (the commentTa may have
// been prefocused and picked up no input; this asserts no accidental
// string survived).
func TestCommentComposer_QDoesNotSaveAsCommentText(t *testing.T) {
	m := openCommentComposerForTest(t)
	// Type a few non-q chars first.
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updatedModelPtr(upd)
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updatedModelPtr(upd)

	// q dismisses — the `hi` typed before should NOT survive
	// (textarea is reset on enterCommentComposer / cancel).
	upd, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = updatedModelPtr(upd)

	if got := m.commentTa.Value(); strings.TrimSpace(got) != "" {
		t.Errorf("commentTa should be empty; got %q", got)
	}
	if len(m.comments) != 0 {
		t.Errorf("dismiss should not save; comments=%d", len(m.comments))
	}
}

// openCommentComposerForTest sets up a model with the comment
// composer open (stateCommentComposer). Helper to keep the three
// dismiss tests above independent of fixture plumbing.
func openCommentComposerForTest(t *testing.T) *model {
	t.Helper()
	m := attachFileFixture(t)
	openFile(t, m, "src/main.go")
	// c opens the composer (whole-file comment since file is open
	// in stateFileView; the exact anchor shape doesn't matter for
	// these tests, only that stateCommentComposer is active).
	upd, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updatedModelPtr(upd)
	if m.state != stateCommentComposer {
		t.Fatalf("setup: expected stateCommentComposer; got %v", m.state)
	}
	return m
}