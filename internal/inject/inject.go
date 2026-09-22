// Package inject sends user-composed text into a tmux pane.
package inject

import (
	"strings"

	"github.com/twistedogic/pinky/internal/tmux"
)

const (
	bracketedPasteStart = "\x1b[200~"
	bracketedPasteEnd   = "\x1b[201~"
)

// Send injects text into pane via send-keys. Multi-line payloads
// are wrapped with bracketed-paste sequences so the terminal/agent
// treats embedded newlines as literal (not Enter key events); the
// trailing send-keys Enter submits the block atomically.
// ponytail: the \n-detection is a heuristic — if the agent's input
// mode changes (e.g., a new agent type), replace with a per-agent
// config or `tmux set-window-option -p bracketed-paste on` toggle.
func Send(pane, text string) error {
	if strings.ContainsRune(text, '\n') {
		text = bracketedPasteStart + text + bracketedPasteEnd
	}
	if _, err := tmux.Run("", "send-keys", "-t", pane, text); err != nil {
		return err
	}
	_, err := tmux.Run("", "send-keys", "-t", pane, "Enter")
	return err
}
