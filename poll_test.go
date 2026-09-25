package main

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"

	"github.com/twistedogic/pinky/internal/session"
)

type pollableSource struct {
	messages []session.Message
	offset   int
}

func (s *pollableSource) NewMessages() ([]session.Message, error) {
	if s.offset >= len(s.messages) {
		return nil, nil
	}
	out := s.messages[s.offset:]
	s.offset = len(s.messages)
	return out, nil
}
func (s *pollableSource) Close() error { return nil }

func newPollModel(src *pollableSource) *model {
	m := newModel()
	m.src = src
	m.state = stateNav
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	m.width = 80
	m.height = 24
	m.refreshViewport()
	return &m
}

func msg(text string) session.Message {
	return session.Message{Role: session.RoleAssistant, Text: text, Ts: time.Now()}
}

// TestSessionMsg_ReschedulesPoll_OnEntries: handler MUST return
// a non-nil Cmd (a follow-up pollCmd) when a sessionMsg with
// entries arrives. Without it, polling dies after the first
// tick and codex's later messages are never shown.
func TestSessionMsg_ReschedulesPoll_OnEntries(t *testing.T) {
	src := &pollableSource{messages: []session.Message{msg("hi")}}
	m := newPollModel(src)
	_, cmd := m.Update(sessionMsg{entries: src.messages})
	if cmd == nil {
		t.Fatal("BUG: sessionMsg with entries returned nil cmd — polling stops")
	}
}

// TestSessionMsg_ReschedulesPoll_OnEmpty: handler MUST also
// reschedule on an empty poll, otherwise a quiet codex session
// dies after one tick.
func TestSessionMsg_ReschedulesPoll_OnEmpty(t *testing.T) {
	src := &pollableSource{messages: nil}
	m := newPollModel(src)
	_, cmd := m.Update(sessionMsg{entries: nil})
	if cmd == nil {
		t.Fatal("BUG: empty sessionMsg returned nil cmd — polling stops")
	}
}

// TestSessionMsg_ContinuousPollingDeliversNewMessages: drives
// the polling cycle through the actual handler. After the first
// sessionMsg, codex appends a new message to the source; the
// rescheduled pollCmd must pick it up and update m.latest.
func TestSessionMsg_ContinuousPollingDeliversNewMessages(t *testing.T) {
	src := &pollableSource{messages: []session.Message{msg("first")}}
	m := newPollModel(src)

	// First sessionMsg: returns cmd that should re-arm the poll.
	upd, cmd := m.Update(sessionMsg{entries: src.messages})
	mv := upd.(model)
	m = &mv
	if cmd == nil {
		t.Fatal("setup: first sessionMsg returned nil cmd")
	}

	// Codex appends a new message; bump offset since the first
	// poll drained everything that was available.
	src.messages = append(src.messages, msg("second — must appear"))
	src.offset = 1

	// Fire the next pollCmd — this is what bubbletea would do
	// when the rescheduled Tick fires.
	sm := cmd().(sessionMsg)
	upd3, _ := m.Update(sm)
	mv3 := upd3.(model)
	m = &mv3

	if m.latest.Text != "second — must appear" {
		t.Errorf("BUG: latest.Text = %q; want %q (poll never re-armed)",
			m.latest.Text, "second — must appear")
	}
}
