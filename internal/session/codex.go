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

func openCodex(pid int) (*codexSource, error) {
	path, err := codexSessionFile(pid)
	if err != nil {
		return nil, err
	}
	return &codexSource{path: path}, nil
}

// codexSessionFile finds the open JSONL session file for the codex pid.
// Uses `lsof` to enumerate open files and picks the one under
// ~/.codex/sessions/ that ends in .jsonl.
func codexSessionFile(pid int) (string, error) {
	out, err := exec.Command("lsof", "-p", fmt.Sprintf("%d", pid), "-a", "-F", "n").Output()
	if err != nil {
		return "", fmt.Errorf("lsof: %w", err)
	}
	home, _ := os.UserHomeDir()
	sessionsRoot := filepath.Join(home, ".codex", "sessions")
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
		return "", fmt.Errorf("no codex session file open for pid %d (looked under %s)", pid, sessionsRoot)
	}
	return best, nil
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
