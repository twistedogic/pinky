// Package history persists agent and user lines as append-only JSONL.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry is one persisted line: either an agent capture or a user redirect.
type Entry struct {
	Ts     time.Time `json:"ts"`
	PaneID string    `json:"pane_id"`
	Role   string    `json:"role"` // "agent" or "user"
	Text   string    `json:"text"`
}

// History is an append-only JSONL writer for a given tmux pane.
// ponytail: Append opens/writes/closes per call (no persistent handle).
// The volume (one line per agent message + one per user redirect)
// is too low for the open-file overhead to matter.
type History struct {
	path   string
	paneID string
}

// Open resolves the history path and returns a History bound to paneID.
func Open(paneID string) (*History, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	return &History{path: path, paneID: paneID}, nil
}

func defaultPath() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "pinky", "history.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "pinky", "history.jsonl"), nil
}

// Append writes one entry to the history file.
func (h *History) Append(role, text string) error {
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(&Entry{
		Ts:     time.Now(),
		PaneID: h.paneID,
		Role:   role,
		Text:   text,
	})
}
