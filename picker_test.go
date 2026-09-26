package main

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"

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
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
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
	// ponytail: non-empty poll buffers into pendingLatest; the empty
	// poll that follows is the turn-boundary signal that commits it.
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "agent response"},
		{Role: session.RoleUser, Text: "user redirect"},
	}})
	got := updated.(model)
	updated, _ = got.Update(sessionMsg{entries: nil})
	got = updated.(model)
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
	if !strings.Contains(view.Content, "waiting for agent") {
		t.Errorf("placeholder expected before any message; got: %q", view.Content)
	}

	// ponytail: live streaming is buffered; the viewport stays on
	// placeholder while the batch arrives. The empty poll that
	// follows commits the buffered text and renders it.
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "# Title\n\nbody"},
	}})
	got := updated.(model)
	updated, _ = got.Update(sessionMsg{entries: nil})
	got = updated.(model)
	view = got.View()
	if strings.Contains(view.Content, "waiting for agent") {
		t.Errorf("placeholder should be gone after turn-end; got: %q", view.Content)
	}
	if got.latest.Text == "" {
		t.Errorf("latest.Text should be set after turn-end")
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

	// New message arrives — commits immediately and clears comments.
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "new message"},
	}})
	got := updated.(model)
	if len(got.comments) != 0 {
		t.Errorf("new-text poll should clear comments; got %d", len(got.comments))
	}
}

// TestSessionMsg_SameMsgKeepsComments verifies that an empty poll
// with no pending text does not clear comments (no turn-end commit).
func TestSessionMsg_SameMsgKeepsComments(t *testing.T) {
	src := &fakeSource{}
	m := newIdleModel(t, src)
	m.latest = session.Message{Role: session.RoleAssistant, Text: "same"}
	m.comments = []render.Comment{{BlockIdx: 0, Text: "kept"}}

	// Empty poll with nothing pending — no commit, comments stay.
	updated, _ := m.Update(sessionMsg{entries: nil})
	got := updated.(model)
	if len(got.comments) != 1 {
		t.Errorf("empty poll with no pending should not clear comments; got %d", len(got.comments))
	}
}

// longMsg returns markdown that renders taller than the test
// viewport (80x20). 60 code lines + heading ≈ 62 rendered lines,
// well past viewport height so YOffset can be non-zero and the
// sticky-bottom pin has somewhere to release.
func longMsg() string {
	var b strings.Builder
	b.WriteString("# Title\n\n```\n")
	for range 60 {
		b.WriteString("line\n")
	}
	b.WriteString("```")
	return b.String()
}

// TestSessionMsg_EmptyPollPreservesYOffset: under buffered-mode
// semantics an empty poll with nothing pending never touches
// YOffset — regardless of where the user has scrolled. This
// replaces the old sticky-bottom tests: there's no auto-scroll
// pin to release, just "your scroll position is yours".
func TestSessionMsg_EmptyPollPreservesYOffset(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	text := longMsg()
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got := updated.(model)
	updated, _ = got.Update(sessionMsg{entries: nil})
	got = updated.(model)

	maxOff := got.viewport.TotalLineCount() - got.viewport.Height()
	if maxOff < 6 {
		t.Fatalf("test fixture too short to scroll; maxOff=%d", maxOff)
	}

	// Simulate "user is at the bottom, then scrolls up 5 lines".
	got.viewport.SetYOffset(maxOff - 5)
	upOff := got.viewport.YOffset()
	if upOff <= 0 {
		t.Fatalf("setup: scroll-up should leave YOffset > 0; got %d", upOff)
	}

	// Empty poll — must not move YOffset.
	updated2, _ := got.Update(sessionMsg{entries: nil})
	got2 := updated2.(model)
	if got2.viewport.YOffset() != upOff {
		t.Errorf("empty poll moved YOffset: was %d, now %d", upOff, got2.viewport.YOffset())
	}
}

// TestSessionMsg_NewCommitGoesToTop: every new assistant message
// pins YOffset to the top so the first sentence is visible. This
// is the regression test for the "first sentence missing" bug —
// Bubble Tea's SetContent preserves YOffset, so a fresh commit
// would otherwise inherit the prior scroll position.
func TestSessionMsg_NewCommitGoesToTop(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	text := longMsg()
	updated, _ := m.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: text},
	}})
	got := updated.(model)

	maxOff := got.viewport.TotalLineCount() - got.viewport.Height()
	got.viewport.SetYOffset(maxOff - 3)
	upOff := got.viewport.YOffset()
	if upOff <= 0 {
		t.Fatalf("setup: scroll-up should leave YOffset > 0; got %d", upOff)
	}

	// New message — must land at top regardless of prior scroll.
	updated2, _ := got.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: "# Different\n\nbody"},
	}})
	got2 := updated2.(model)
	if got2.viewport.YOffset() != 0 {
		t.Errorf("new message should land at top; YOffset=%d", got2.viewport.YOffset())
	}
}
