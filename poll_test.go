package main

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

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

// TestSessionMsg_ReschedulesPoll_OnEntries: handler must NOT
// return a follow-up pollCmd once the init poll has resolved —
// `r` is the only post-attach fetch trigger. If this flips
// back, the 500ms tick loop returns and comments silently
// evaporate every time codex appends to the same message.
func TestSessionMsg_ReschedulesPoll_OnEntries(t *testing.T) {
	src := &pollableSource{messages: []session.Message{msg("hi")}}
	m := newPollModel(src)
	_, cmd := m.Update(sessionMsg{entries: src.messages})
	if cmd != nil {
		t.Fatalf("BUG: sessionMsg with entries rescheduled a poll (cmd=%v); manual-refresh contract violated", cmd)
	}
}

// TestSessionMsg_ReschedulesPoll_OnEmpty: an empty sessionMsg
// must NOT reschedule a follow-up poll — after attach polling
// stops and `r` is the only fetch trigger. Rescheduling here
// would silently re-introduce the 500ms tick loop.
func TestSessionMsg_ReschedulesPoll_OnEmpty(t *testing.T) {
	src := &pollableSource{messages: nil}
	m := newPollModel(src)
	_, cmd := m.Update(sessionMsg{entries: nil})
	if cmd != nil {
		t.Fatalf("BUG: empty sessionMsg rescheduled a poll (cmd=%v); manual-refresh contract violated", cmd)
	}
}

// TestSessionMsg_ManualRefreshDeliversNewMessages: with the
// 500ms tick loop removed, the second message must come from a
// second `r` press, not from a rescheduled pollCmd. Drives
// the same scenario as the old auto-polling test: codex appends
// a message, the user presses `r`, and the new message lands.
func TestSessionMsg_ManualRefreshDeliversNewMessages(t *testing.T) {
	src := &pollableSource{messages: []session.Message{msg("first")}}
	m := newPollModel(src)

	// (a) First sessionMsg: the init poll resolved with [first].
	// No follow-up Cmd under the manual-refresh contract.
	upd, cmd := m.Update(sessionMsg{entries: src.messages})
	mv := upd.(model)
	m = &mv
	if cmd != nil {
		t.Fatalf("setup: first sessionMsg rescheduled a poll (cmd=%v); manual-refresh contract violated", cmd)
	}
	if m.latest.Text != "first" {
		t.Fatalf("setup: latest.Text = %q; want %q", m.latest.Text, "first")
	}

	// (b) Codex appends a second message. Bump offset to mirror
	// the production flow: the init poll already drained
	// everything that was available, so the source returns
	// only the appended message on the next fetch.
	src.messages = append(src.messages, msg("second — must appear"))
	src.offset = 1

	// (c) User presses `r`; the handler hits ActionRefresh and
	// returns exactly one pollCmd(m.src).
	rKey := tea.KeyPressMsg{Code: 'r'}
	upd2, cmd2 := m.Update(rKey)
	mv2 := upd2.(model)
	m = &mv2
	if cmd2 == nil {
		t.Fatal("BUG: `r` press returned nil cmd — manual refresh broken")
	}

	// Fire r's pollCmd — pulls the appended message (tea.Tick
	// blocks for pollInterval, ~500ms, by design).
	sm := cmd2().(sessionMsg)
	upd3, _ := m.Update(sm)
	mv3 := upd3.(model)
	m = &mv3

	if m.latest.Text != "second — must appear" {
		t.Errorf("BUG: latest.Text = %q; want %q (manual refresh did not deliver second message)",
			m.latest.Text, "second — must appear")
	}
}
