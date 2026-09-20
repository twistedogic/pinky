// Package history persists agent and user lines as append-only JSONL.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
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
type History struct {
	path   string
	paneID string
	f      io.Closer
	enc    *json.Encoder
}

// Open creates (or appends to) the history file and returns a History
// bound to paneID.
func Open(paneID string) (*History, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &History{path: path, paneID: paneID, f: f, enc: json.NewEncoder(f)}, nil
}

// Path returns the resolved history file path (computed even before Open).
func Path() (string, error) { return defaultPath() }

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
	return h.enc.Encode(&Entry{
		Ts:     time.Now(),
		PaneID: h.paneID,
		Role:   role,
		Text:   text,
	})
}

// Close flushes and closes the history file.
func (h *History) Close() error {
	if h.f == nil {
		return nil
	}
	return h.f.Close()
}

// Load reads all history entries for the given pane, oldest first.
// Malformed lines are skipped.
func Load(paneID string) ([]Entry, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if e.PaneID == paneID {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}
