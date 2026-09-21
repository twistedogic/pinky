// Package tmux is a thin wrapper around the tmux CLI.
package tmux

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrTmuxNotRunning is returned when $TMUX is unset.
var ErrTmuxNotRunning = errors.New("tmux not running: $TMUX is unset")

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

// RunStderr executes tmux with the given args (and optional stdin),
// surfacing stderr on failure. Use for commands where only the
// exit status and stderr matter (e.g. load-buffer).
func RunStderr(stdin string, args ...string) error {
	cmd := exec.Command("tmux", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(buf)))
	}
	return nil
}

// RequireServer exits with a clear error if $TMUX is unset.
func RequireServer() error {
	if _, ok := os.LookupEnv("TMUX"); !ok {
		return ErrTmuxNotRunning
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
