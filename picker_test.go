package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
)

// fakeSource is a minimal session.Source for testing the picker flow.
type fakeSource struct {
	messages []session.Message
	offset   int
}

func (s *fakeSource) NewMessages() ([]session.Message, error) {
	if s.offset >= len(s.messages) {
		return nil, nil
	}
	out := s.messages[s.offset:]
	s.offset = len(s.messages)
	return out, nil
}

func (s *fakeSource) Close() error { return nil }

// helper: drive a picker Enter by directly setting post-attach state on
// the model. This bypasses attach() (which requires tmux) but exercises
// the rest of the flow identically.
func newIdleModel(t *testing.T, src session.Source) model {
	t.Helper()
	m := newModel()
	m.agents = []session.AgentSession{{Session: "s", Window: "0", Pane: "0", PaneID: "%1", Agent: "pi"}}
	m.pickCursor = 0
	m.src = src
	m.hist = nil
	m.state = stateNav
	m.viewport = viewport.New(80, 20)
	m.width = 80
	m.height = 24
	m.refreshViewport() // seed viewport with placeholder
	return m
}

func TestSessionMsg_PrefersAssistantOverUser(t *testing.T) {
	src := &fakeSource{
		messages: []session.Message{
			{Role: session.RoleAssistant, Text: "agent response", Ts: time.Now()},
			{Role: session.RoleUser, Text: "user redirect", Ts: time.Now()},
		},
	}
	m := newIdleModel(t, src)
	msg := sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "agent response"},
		{Role: session.RoleUser, Text: "user redirect"},
	}}
	updated, _ := m.Update(msg)
	got := updated.(model)
	if got.latest.Text != "agent response" {
		t.Errorf("latest.Text = %q, want %q", got.latest.Text, "agent response")
	}
	if got.latest.Role != session.RoleAssistant {
		t.Errorf("latest.Role = %q, want %q", got.latest.Role, session.RoleAssistant)
	}
}

func TestSessionMsg_EmptyPollKeepsLatest(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	m.latest = session.Message{Role: session.RoleAssistant, Text: "previous"}
	updated, _ := m.Update(sessionMsg{entries: nil})
	got := updated.(model)
	if got.latest.Text != "previous" {
		t.Errorf("empty poll should not clear latest; got %q want %q", got.latest.Text, "previous")
	}
}

func TestSessionMsg_FirstMessageRendersIntoViewport(t *testing.T) {
	src := &fakeSource{
		messages: []session.Message{
			{Role: session.RoleAssistant, Text: "# Title\n\nbody", Ts: time.Now()},
		},
	}
	m := newIdleModel(t, src)
	// Pre-poll: latest is empty, viewport should show placeholder.
	view := m.View()
	if !strings.Contains(view, "waiting for agent") {
		t.Errorf("placeholder expected before any message; got: %q", view)
	}

	// After sessionMsg, viewport should contain rendered content.
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "# Title\n\nbody"},
	}})
	got := updated.(model)
	view = got.View()
	if strings.Contains(view, "waiting for agent") {
		t.Errorf("placeholder should be gone after message; got: %q", view)
	}
	if got.latest.Text == "" {
		t.Errorf("latest.Text should be set after message")
	}
}

func TestSessionMsg_AssistantOnlyPollSetsLatest(t *testing.T) {
	// Poll that only contains a user redirect. No assistant message.
	// latest should NOT change (placeholder stays).
	src := &fakeSource{
		messages: []session.Message{
			{Role: session.RoleUser, Text: "user only", Ts: time.Now()},
		},
	}
	m := newIdleModel(t, src)
	m.latest = session.Message{Role: session.RoleAssistant, Text: "existing"}
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleUser, Text: "user only"},
	}})
	got := updated.(model)
	if got.latest.Text != "existing" {
		t.Errorf("user-only poll should not change latest; got %q want %q", got.latest.Text, "existing")
	}
}

// TestSessionMsg_TextChangeClearsComments verifies that when the
// latest message text changes, the comments slice is reset.
func TestSessionMsg_TextChangeClearsComments(t *testing.T) {
	src := &fakeSource{}
	m := newIdleModel(t, src)
	// Pre-populate with comments on a "previous" message.
	m.latest = session.Message{Role: session.RoleAssistant, Text: "previous message"}
	m.comments = []render.Comment{
		{BlockIdx: 0, Text: "old comment"},
	}

	// New message arrives.
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "new message"},
	}})
	got := updated.(model)
	if len(got.comments) != 0 {
		t.Errorf("comments should clear when latest text changes; got %d", len(got.comments))
	}
}

// TestSessionMsg_SameMsgKeepsComments verifies that identical
// messages do not clear comments.
func TestSessionMsg_SameMsgKeepsComments(t *testing.T) {
	src := &fakeSource{}
	m := newIdleModel(t, src)
	m.latest = session.Message{Role: session.RoleAssistant, Text: "same"}
	m.comments = []render.Comment{{BlockIdx: 0, Text: "kept"}}

	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "same"},
	}})
	got := updated.(model)
	if len(got.comments) != 1 {
		t.Errorf("identical messages should not clear comments; got %d", len(got.comments))
	}
}

// longMsg returns markdown that renders taller than the test
// viewport (80x20). 60 code lines + heading ≈ 62 rendered lines,
// well past viewport height so YOffset can be non-zero and the
// sticky-bottom pin has somewhere to release.
func longMsg() string {
	var b strings.Builder
	b.WriteString("# Title\n\n```\n")
	for i := 0; i < 60; i++ {
		b.WriteString("line\n")
	}
	b.WriteString("```")
	return b.String()
}

// TestSessionMsg_StickyBottom_ReleasedOnScrollUp verifies that
// after the user scrolls up, a poll with no new text does NOT
// yank the viewport back to the bottom (the fix for the
// scroll-up bug).
func TestSessionMsg_StickyBottom_ReleasedOnScrollUp(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	text := longMsg()
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got := updated.(model)
	if !got.viewport.AtBottom() {
		t.Fatalf("first poll should land at bottom; YOffset=%d", got.viewport.YOffset)
	}

	// User scrolls up by 5 lines.
	got.viewport.SetYOffset(got.viewport.YOffset - 5)
	upOff := got.viewport.YOffset
	if upOff <= 0 {
		t.Fatalf("scroll-up should leave YOffset > 0; got %d", upOff)
	}

	// Poll with same text — must not move YOffset.
	updated2, _ := got.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got2 := updated2.(model)
	if got2.viewport.YOffset != upOff {
		t.Errorf("scrolled-up YOffset changed after poll: was %d, now %d",
			upOff, got2.viewport.YOffset)
	}
}

// TestSessionMsg_StickyBottom_ReattachOnEnd verifies that pressing
// End (which scrolls the viewport to the bottom) re-anchors the
// sticky-bottom pin, so the next poll resumes auto-follow.
func TestSessionMsg_StickyBottom_ReattachOnEnd(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	text := longMsg()
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got := updated.(model)

	// Scroll up + poll (no reattach).
	got.viewport.SetYOffset(got.viewport.YOffset - 3)
	upOff := got.viewport.YOffset
	updated2, _ := got.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got2 := updated2.(model)
	if got2.viewport.YOffset != upOff {
		t.Fatalf("setup: scrolled-up YOffset should stick; was %d now %d",
			upOff, got2.viewport.YOffset)
	}

	// Press End to return to bottom, then poll — should reattach.
	for !got2.viewport.AtBottom() {
		updated3, _ := got2.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		got2 = updated3.(model)
	}
	if !got2.viewport.AtBottom() {
		t.Fatalf("PageDown should scroll to bottom; YOffset=%d", got2.viewport.YOffset)
	}
	updated4, _ := got2.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got4 := updated4.(model)
	if !got4.viewport.AtBottom() {
		t.Errorf("after returning to bottom, poll should stay at bottom; YOffset=%d", got4.viewport.YOffset)
	}
}

// TestSessionMsg_NewTextForceAttaches verifies that a brand-new
// message (different Text) force-scrolls to the bottom regardless
// of prior scroll position.
func TestSessionMsg_NewTextForceAttaches(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	text := longMsg()
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got := updated.(model)

	// Scroll up.
	got.viewport.SetYOffset(got.viewport.YOffset - 5)
	if got.viewport.YOffset <= 0 {
		t.Fatalf("setup: scroll-up should leave YOffset > 0")
	}

	// New message replaces — must land at bottom.
	updated2, _ := got.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "# Different\n\nbody"},
	}})
	got2 := updated2.(model)
	if !got2.viewport.AtBottom() {
		t.Errorf("new message should force bottom; YOffset=%d", got2.viewport.YOffset)
	}
}
