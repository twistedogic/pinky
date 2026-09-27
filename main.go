// pinky is a persistent TUI for capturing the last agent message from
// pi or codex (by tailing their session JSONL) and injecting precise
// push-backs back into the agent's tmux pane via paste-buffer + send-keys.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/twistedogic/pinky/internal/session"
	"github.com/twistedogic/pinky/internal/tmux"
)

const (
	pollInterval = 500 * time.Millisecond
)

func main() {
	// tmux is a hard prerequisite (we shell out to it constantly).
	// If it's not running, exit before constructing any TUI state —
	// there's no useful TUI to show.
	if err := tmux.RequireServer(); err != nil {
		fail(err)
	}

	model := newModel()
	agents, err := session.ListAgents()
	if err != nil {
		model.err = err
		model.state = stateError
	} else if len(agents) == 0 {
		model.err = errors.New("no active pi or codex agents found in any tmux pane")
		model.state = stateError
	} else {
		model.setAgents(agents)
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fail(err)
	}
}

// fail prints to stderr and exits. Used only for unrecoverable startup
// errors where no TUI can render (tmux not running, bubbletea itself
// failing to start).
func fail(err error) {
	fmt.Fprintln(os.Stderr, "pinky:", err)
	os.Exit(1)
}
