package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/session"
)

// helper: build a minimal model that just renders View() so we can
// assert on the output.
func renderView(t *testing.T, m model) string {
	t.Helper()
	return m.View()
}

func TestView_PadsPickerToFullWidth(t *testing.T) {
	m := newModel()
	m.agents = []session.AgentSession{
		{Session: "work", Window: "1", Pane: "1", PaneID: "%12", Agent: "codex"},
	}
	m.cursor = 0
	m.width = 80
	m.height = 24

	view := renderView(t, m)
	for _, line := range strings.Split(view, "\n") {
		if w := visualLen(line); w != m.width {
			t.Errorf("line width = %d want %d\n  line: %q", w, m.width, line)
		}
	}
}

func TestView_PadsStatusLineToFullWidth(t *testing.T) {
	m := newModel()
	m.state = stateIdle
	m.pane = "%12"
	m.width = 80
	m.height = 24
	m.viewport = viewport.New(80, 20)

	view := renderView(t, m)
	lines := strings.Split(view, "\n")
	last := lines[len(lines)-1]
	if w := visualLen(last); w != m.width {
		t.Errorf("status line width = %d want %d\n  line: %q", w, m.width, last)
	}
}

// visualLen returns the visible (rune) count of s, ignoring ANSI.
// Defined as a thin wrapper around the package-level visibleWidth so
// the test reads naturally.
func visualLen(s string) int {
	return visibleWidth(s)
}

// silence unused import warning
var _ tea.Model = model{}
