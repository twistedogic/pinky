// Package tmux is a thin wrapper around the tmux CLI.
package tmux

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Run executes tmux with the given args and returns trimmed stdout.
// If stdin is non-empty it is piped to the command.
func Run(stdin string, args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

// errTmuxNotRunning is returned when $TMUX is unset.
var errTmuxNotRunning = errors.New("tmux not running: $TMUX is unset")

// RequireServer exits with a clear error if $TMUX is unset.
func RequireServer() error {
	if _, ok := os.LookupEnv("TMUX"); !ok {
		return errTmuxNotRunning
	}
	return nil
}

// PaneExists checks whether the given tmux pane id resolves.
func PaneExists(pane string) bool {
	out, err := Run("", "display-message", "-t", pane, "-p", "#{pane_id}")
	if err != nil || out == "" {
		return false
	}
	return true
}
