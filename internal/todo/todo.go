// Package todo persists the per-workspace todo list to disk.
//
// The list lives at ~/.local/share/pinky/<workspace_name>/todos.json
// where <workspace_name> is filepath.Base(cwd). Two cwds with the
// same last segment intentionally share a single file (see the
// add-todo-tab change). Writes are atomic: <path>.tmp + os.Rename.
package todo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound is returned by Load when the file does not exist.
var ErrNotFound = errors.New("todo: file not found")

// Item is one row in the list. Insertion order is preserved by the
// surrounding slice; no per-item UUID.
type Item struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// Save writes the list to path atomically. Parent directories are
// created as needed.
func Save(path string, items []Item) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("todo: mkdir: %w", err)
	}
	data, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("todo: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("todo: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("todo: rename: %w", err)
	}
	return nil
}

// Load reads the list from path. Returns ErrNotFound if the file
// does not exist (treat as empty list at the call site).
func Load(path string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("todo: read: %w", err)
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("todo: unmarshal: %w", err)
	}
	return items, nil
}