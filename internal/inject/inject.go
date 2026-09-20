// Package inject sends user-composed text into a tmux pane.
package inject

import (
	"github.com/twistedogic/pinky/internal/tmux"
)

// Send injects text into pane via set-buffer → paste-buffer → send-keys Enter.
// Preserves newlines and Unicode verbatim.
func Send(pane, text string) error {
	if err := tmux.RunStdin(text, "load-buffer", "-"); err != nil {
		return err
	}
	if _, err := tmux.Run("paste-buffer", "-t", pane); err != nil {
		return err
	}
	if _, err := tmux.Run("send-keys", "-t", pane, "Enter"); err != nil {
		return err
	}
	return nil
}
