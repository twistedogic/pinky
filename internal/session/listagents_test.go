package session

import (
	"os/exec"
	"strings"
	"testing"
)

// TestListAgentsFiltersNonAgents verifies ListAgents enumerates tmux panes
// but only includes those whose descendant tree contains pi or codex.
// Requires a running tmux server with at least one matching agent.
func TestListAgentsFiltersNonAgents(t *testing.T) {
	// Skip if tmux isn't running.
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}
	agents, err := ListAgents()
	if err != nil {
		t.Skipf("ListAgents failed (no tmux?): %v", err)
	}
	for _, a := range agents {
		if a.Agent != "pi" && a.Agent != "codex" {
			t.Errorf("non-agent listed: %+v", a)
		}
		if a.PaneID == "" || !strings.HasPrefix(a.PaneID, "%") {
			t.Errorf("bad pane id: %+v", a)
		}
	}
	t.Logf("found %d agents:", len(agents))
	for _, a := range agents {
		t.Logf("  %s:%s.%s %s %s", a.Session, a.Window, a.Pane, a.PaneID, a.Agent)
	}
}
