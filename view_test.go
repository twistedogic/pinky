package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
)

func TestView_PadsPickerToFullWidth(t *testing.T) {
	m := newModel()
	m.agents = []session.AgentSession{
		{Session: "work", Window: "1", Pane: "1", PaneID: "%12", Agent: "codex"},
	}
	m.pickCursor = 0
	m.width = 80
	m.height = 24

	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if w := render.VisibleWidth(line); w != m.width {
			t.Errorf("line width = %d want %d\n  line: %q", w, m.width, line)
		}
	}
}

func TestView_PadsStatusLineToFullWidth(t *testing.T) {
	m := newModel()
	m.state = stateNav
	m.pane = "%12"
	m.width = 80
	m.height = 24
	m.viewport = viewport.New(80, 20)

	view := m.View()
	lines := strings.Split(view, "\n")
	last := lines[len(lines)-1]
	if w := render.VisibleWidth(last); w != m.width {
		t.Errorf("status line width = %d want %d\n  line: %q", w, m.width, last)
	}
}