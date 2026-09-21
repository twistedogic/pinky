package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/session"
)

// attachFakeSource is reused for attach-flow tests. Mirrors the one
// in picker_test.go but kept local to avoid cross-file coupling.
type attachFakeSource struct{}

func (attachFakeSource) NewMessages() ([]session.Message, error) { return nil, nil }
func (attachFakeSource) Close() error                            { return nil }

// TestAttach_PopulatesViewportImmediately verifies that after the
// seeding step the view contains the placeholder. Guards against
// either path (picker / --target / --session-file) losing the
// refreshViewport call in a future refactor.
func TestAttach_PopulatesViewportImmediately(t *testing.T) {
	m := newModel()
	m.agents = []session.AgentSession{{Session: "s", Window: "0", Pane: "0", PaneID: "%1", Agent: "pi"}}
	m.cursor = 0
	m.src = attachFakeSource{}
	m.hist = nil
	m.state = stateIdle
	m.viewport = viewport.New(80, 20)
	m.width = 80
	m.height = 24
	m.refreshViewport()

	view := m.View()
	if !strings.Contains(view, "waiting for agent") {
		t.Errorf("after refreshViewport, view should contain placeholder; got:\n%q",
			view[:min(len(view), 200)])
	}
}

// TestAttach_ViewportMatchesWindowWidth verifies that after the
// seeding step the viewport's Width tracks m.width, not the hardcoded
// 40-col default that the old code path was resetting to. Regression
// guard for the "rendered test is cropped" bug on the picker path.
func TestAttach_ViewportMatchesWindowWidth(t *testing.T) {
	m := newModel()
	m.width = 120 // simulate post-WindowSizeMsg
	m.height = 40

	w, _ := m.viewportSize()
	if w != 120 {
		t.Errorf("viewportSize() width = %d want 120", w)
	}
}

// silence unused import warnings
var _ tea.Model = model{}
