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
	m.cursor = 0
	m.src = src
	m.hist = nil
	m.state = stateIdle
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
	msg := sessionMsg{entries: []entry{
		{role: roleAgent, text: "agent response"},
		{role: roleUser, text: "user redirect"},
	}}
	updated, _ := m.Update(msg)
	got := updated.(model)
	if got.latest.text != "agent response" {
		t.Errorf("latest.text = %q, want %q", got.latest.text, "agent response")
	}
	if got.latest.role != roleAgent {
		t.Errorf("latest.role = %q, want %q", got.latest.role, roleAgent)
	}
}

func TestSessionMsg_EmptyPollKeepsLatest(t *testing.T) {
	m := newIdleModel(t, &fakeSource{})
	m.latest = entry{role: roleAgent, text: "previous"}
	updated, _ := m.Update(sessionMsg{entries: nil})
	got := updated.(model)
	if got.latest.text != "previous" {
		t.Errorf("empty poll should not clear latest; got %q want %q", got.latest.text, "previous")
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
	updated, _ := m.Update(sessionMsg{entries: []entry{
		{role: roleAgent, text: "# Title\n\nbody"},
	}})
	got := updated.(model)
	view = got.View()
	if strings.Contains(view, "waiting for agent") {
		t.Errorf("placeholder should be gone after message; got: %q", view)
	}
	if got.latest.text == "" {
		t.Errorf("latest.text should be set after message")
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
	m.latest = entry{role: roleAgent, text: "existing"}
	updated, _ := m.Update(sessionMsg{entries: []entry{
		{role: roleUser, text: "user only"},
	}})
	got := updated.(model)
	if got.latest.text != "existing" {
		t.Errorf("user-only poll should not change latest; got %q want %q", got.latest.text, "existing")
	}
}

// silence unused import warnings if any
var _ tea.Model = model{}

// TestSessionMsg_FlipsMsgHashClearsComments verifies that when the
// latest message text changes, the comments slice is reset.
func TestSessionMsg_FlipsMsgHashClearsComments(t *testing.T) {
	src := &fakeSource{}
	m := newIdleModel(t, src)
	// Pre-populate with comments on a "previous" message.
	m.latest = entry{role: roleAgent, text: "previous message"}
	m.msgHash = commentHash("previous message")
	m.comments = []render.Comment{
		{BlockIdx: 0, Text: "old comment"},
	}

	// New message arrives.
	updated, _ := m.Update(sessionMsg{entries: []entry{
		{role: roleAgent, text: "new message"},
	}})
	got := updated.(model)
	if len(got.comments) != 0 {
		t.Errorf("comments should clear on msgHash flip; got %d", len(got.comments))
	}
	if got.msgHash != commentHash("new message") {
		t.Errorf("msgHash should update to new text's hash")
	}
}

// TestSessionMsg_SameMsgKeepsComments verifies that identical
// messages do not clear comments.
func TestSessionMsg_SameMsgKeepsComments(t *testing.T) {
	src := &fakeSource{}
	m := newIdleModel(t, src)
	m.latest = entry{role: roleAgent, text: "same"}
	m.msgHash = commentHash("same")
	m.comments = []render.Comment{{BlockIdx: 0, Text: "kept"}}

	updated, _ := m.Update(sessionMsg{entries: []entry{
		{role: roleAgent, text: "same"},
	}})
	got := updated.(model)
	if len(got.comments) != 1 {
		t.Errorf("identical messages should not clear comments; got %d", len(got.comments))
	}
}
