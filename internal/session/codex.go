package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// codexSource tails a codex rollout JSONL file.
type codexSource struct {
	path   string
	offset int64
}

func openCodex(pid int, cwd string) (*codexSource, error) {
	// 1. cwd-based discovery (plannotator convention). codex doesn't
	//    depend on cwd, but the date tree makes this the most
	//    reliable fallback.
	if path, err := codexByCwd(cwd); err == nil {
		debugf("openCodex(%d): discovered via sessions root: %s", pid, path)
		return &codexSource{path: path}, nil
	} else {
		debugf("openCodex(%d): cwd-based discovery failed: %v", pid, err)
	}
	// 2. lsof fallback (existing).
	if path, err := codexSessionFile(pid); err == nil {
		debugf("openCodex(%d): discovered via lsof: %s", pid, path)
		return &codexSource{path: path}, nil
	} else {
		debugf("openCodex(%d): lsof fallback failed: %v", pid, err)
	}
	return nil, fmt.Errorf("codex process %d: no session file found\n\n"+
		"troubleshooting:\n"+
		"  - cwd-based lookup under the codex sessions root found nothing\n"+
		"  - lsof on pid %d found no open .jsonl\n"+
		"  - run with PINKY_DEBUG=1 for verbose discovery output\n"+
		"  - bypass with --session-file /path/to/session.jsonl",
		pid, pid)
}

// codexSessionFile finds the open JSONL session file for the codex pid.
// Uses `lsof` to enumerate open files and picks the one under the codex
// sessions root that ends in .jsonl. Used as a last-resort fallback.
func codexSessionFile(pid int) (string, error) {
	cmd := exec.Command("lsof", "-p", fmt.Sprintf("%d", pid), "-a", "-F", "n")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("lsof failed (is lsof installed?): %w\n\n"+
			"troubleshooting:\n"+
			"  - check open files: lsof -p %d | grep jsonl",
			err, pid)
	}
	home, homeErr := codexHome()
	if homeErr != nil {
		return "", homeErr
	}
	sessionsRoot := filepath.Join(home, "sessions")
	var best string
	var bestMod time.Time
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "n") {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		if !strings.HasSuffix(path, ".jsonl") {
			continue
		}
		if !strings.HasPrefix(path, sessionsRoot) {
			continue
		}
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if best == "" || fi.ModTime().After(bestMod) {
			best = path
			bestMod = fi.ModTime()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no codex session file open for pid %d (looked under %s)\n\n"+
			"troubleshooting:\n"+
			"  - check open files: lsof -p %d | grep jsonl",
			pid, sessionsRoot, pid)
	}
	return best, nil
}

// codexByCwd finds the newest codex session JSONL via the cwd-based
// convention (plannotator): `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-*.jsonl`.
// Newest by filename across all date subdirs.
func codexByCwd(cwd string) (string, error) {
	home, err := codexHome()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, "sessions")
	var newest string
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable subtrees
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		name := info.Name()
		// Accept both the standard `rollout-<timestamp>-<uuid>.jsonl`
		// pattern and any .jsonl under the sessions tree.
		if !strings.HasPrefix(name, "rollout-") {
			return nil
		}
		if newest == "" || name > filepath.Base(newest) {
			newest = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if newest == "" {
		return "", fmt.Errorf("no rollout-*.jsonl files under %s", root)
	}
	_ = cwd // accepted but currently unused; codex's layout is cwd-independent
	return newest, nil
}

func (s *codexSource) NewMessages() ([]Message, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(s.offset, 0); err != nil {
		return nil, err
	}

	var out []Message
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var raw codexEntry
		if err := json.Unmarshal(sc.Bytes(), &raw); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339Nano, raw.Timestamp)
		if raw.Type != "response_item" || raw.Payload == nil {
			continue
		}
		if raw.Payload.Type != "message" {
			continue
		}
		switch raw.Payload.Role {
		case "assistant":
			for _, c := range raw.Payload.Content {
				if (c.Type == "output_text" || c.Type == "text") && c.Text != "" {
					out = append(out, Message{Role: RoleAssistant, Text: c.Text, Ts: ts})
				}
			}
		case "user":
			for _, c := range raw.Payload.Content {
				if (c.Type == "input_text" || c.Type == "text") && c.Text != "" {
					out = append(out, Message{Role: RoleUser, Text: c.Text, Ts: ts})
				}
			}
		}
	}

	end, _ := f.Seek(0, os.SEEK_CUR)
	s.offset = end
	return out, sc.Err()
}

func (s *codexSource) Close() error { return nil }

// codexEntry mirrors codex rollout JSONL lines.
type codexEntry struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Payload   *codexPayload  `json:"payload,omitempty"`
}

type codexPayload struct {
	Type    string        `json:"type"`
	Role    string        `json:"role,omitempty"`
	Content []codexContent `json:"content,omitempty"`
}

type codexContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
