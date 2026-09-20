// Package tmux is a thin wrapper around the tmux CLI.
package tmux

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrTmuxNotRunning is returned when $TMUX is unset.
var ErrTmuxNotRunning = errors.New("tmux not running: $TMUX is unset")

// ErrPaneMissing is returned when a target pane does not exist.
var ErrPaneMissing = errors.New("tmux pane not found")

// Run executes tmux with the given args and returns trimmed stdout.
func Run(args ...string) (string, error) {
	out, err := exec.Command("tmux", args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

// RunStdin executes tmux with the given args, feeding stdin from s.
func RunStdin(s string, args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stdin = strings.NewReader(s)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.New(strings.TrimSpace(stderr.String()))
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
	out, err := Run("display-message", "-t", pane, "-p", "#{pane_id}")
	if err != nil || out == "" {
		return false
	}
	return true
}
