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
	flag.Parse()

	if err := tmux.RequireServer(); err != nil {
		fail(err)
	}

	model := newModel()
	if *target != "" {
		pane, err := resolveTarget(*target)
		if err != nil {
			fail(err)
		}
		if err := model.attach(pane); err != nil {
			fail(err)
		}
	} else {
		agents, err := session.ListAgents()
		if err != nil {
			fail(err)
		}
		if len(agents) == 0 {
			fail(errors.New("no active pi or codex agents found in any tmux pane"))
		}
		model.setAgents(agents)
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pinky:", err)
	os.Exit(1)
}
