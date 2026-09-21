package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// piEncodedDir encodes a cwd into the bucket directory name pi uses:
// two leading dashes, the cwd with leading slashes stripped and `/`,
// `\`, `:` replaced by `-`, then two trailing dashes. See plannotator
// (pi-mono convention): `<sessions>/--<encoded cwd>--/<timestamp>_<uuid>.jsonl`.
func piEncodedDir(cwd string) string {
	trimmed := strings.TrimLeft(cwd, "/\\")
	body := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(trimmed)
	return "--" + body + "--"
}

// newestJSONL returns the newest-by-filename `.jsonl` file directly
// under dir. Filenames in pi/codex session directories all start with
// an ISO-style timestamp (or `rollout-<timestamp>-` for codex), so
// lexicographic sort matches chronological newest.
func newestJSONL(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".jsonl" {
			continue
		}
		candidates = append(candidates, filepath.Join(dir, name))
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no .jsonl files in %s", dir)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	return candidates[0], nil
}

// piSessionDir returns the pi sessions directory, honoring
// `PI_CODING_AGENT_SESSION_DIR` and `PI_CODING_AGENT_DIR`. Default is
// `~/.pi/agent/sessions` (matching plannotator's convention for pi-mono).
func piSessionDir() (string, error) {
	if v := os.Getenv("PI_CODING_AGENT_SESSION_DIR"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if v := os.Getenv("PI_CODING_AGENT_DIR"); v != "" {
		return filepath.Join(v, "sessions"), nil
	}
	return filepath.Join(home, ".pi", "agent", "sessions"), nil
}

// codexHome returns the codex home directory, honoring `CODEX_HOME`.
// Default is `~/.codex` (matching plannotator's convention).
func codexHome() (string, error) {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}
