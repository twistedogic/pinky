// pinky is a persistent TUI for capturing the last agent message from
// pi or codex (by tailing their session JSONL) and injecting precise
// push-backs back into the agent's tmux pane via paste-buffer + send-keys.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/twistedogic/pinky/internal/session"
	"github.com/twistedogic/pinky/internal/tmux"
)

const (
	pollInterval = 500 * time.Millisecond
)

func main() {
	target := flag.String("target", "", "tmux pane id to watch (skips the picker)")
	sessionFile := flag.String("session-file", "", "explicit path to the agent's session JSONL (skips PI_SESSION_FILE / lsof discovery)")
	flag.Parse()

	// tmux is a hard prerequisite (we shell out to it constantly).
	// If it's not running, exit before constructing any TUI state —
	// there's no useful TUI to show.
	if err := tmux.RequireServer(); err != nil {
		fail(err)
	}

	model := newModel()
	if *sessionFile != "" {
		if err := model.attachWithFile(*sessionFile); err != nil {
			model.err = err
			model.state = stateError
		}
	} else if *target != "" {
		pane, err := resolveTarget(*target)
		if err != nil {
			model.err = err
			model.state = stateError
		} else if err := model.attach(pane); err != nil {
			model.err = err
			model.state = stateError
		}
	} else {
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
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fail(err)
	}
}

func resolveTarget(flagVal string) (string, error) {
	pane := flagVal
	if pane == "" {
		pane = os.Getenv("TMUX_PANE")
	}
	if pane == "" {
		return "", errors.New("no target pane: pass --target or run inside a tmux pane (TMUX_PANE)")
	}
	if !tmux.PaneExists(pane) {
		return "", fmt.Errorf("target pane %q does not exist", pane)
	}
	return pane, nil
}

// fail prints to stderr and exits. Used only for unrecoverable startup
// errors where no TUI can render (tmux not running, bubbletea itself
// failing to start).
func fail(err error) {
	fmt.Fprintln(os.Stderr, "pinky:", err)
	os.Exit(1)
}
