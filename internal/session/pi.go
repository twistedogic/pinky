package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// piSource tails a pi agent session JSONL file.
type piSource struct {
	path   string
	offset int64
}

func openPi(pid int) (*piSource, error) {
	path, ok := envValue(pid, "PI_SESSION_FILE")
	if !ok || path == "" {
		return nil, fmt.Errorf("pi process %d: PI_SESSION_FILE not set", pid)
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("pi session file %q: %w", path, err)
	}
	return &piSource{path: path}, nil
}

func (s *piSource) NewMessages() ([]Message, error) {
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
		var raw piEntry
		if err := json.Unmarshal(sc.Bytes(), &raw); err != nil {
			continue
		}
		if raw.Type != "message" || raw.Message == nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339Nano, raw.Timestamp)
		msgs := extractPi(raw.Message, ts)
		out = append(out, msgs...)
	}

	end, _ := f.Seek(0, os.SEEK_CUR)
	s.offset = end
	return out, sc.Err()
}

func (s *piSource) Close() error { return nil }

// extractPi pulls assistant text and user text out of a pi message.
// pi's `content` field is polymorphic: string for some messages,
// array of typed blocks for others. We handle both.
func extractPi(m *piMsg, ts time.Time) []Message {
	var out []Message
	switch m.Role {
	case "assistant":
		// Assistant content is always an array of typed blocks.
		for _, c := range m.ContentItems {
			if c.Type == "text" && c.Text != "" {
				out = append(out, Message{Role: RoleAssistant, Text: c.Text, Ts: ts})
			}
		}
	case "user":
		// User content may be a plain string or an array.
		if m.Content != "" {
			out = append(out, Message{Role: RoleUser, Text: m.Content, Ts: ts})
			return out
		}
		for _, c := range m.ContentItems {
			if c.Type == "text" && c.Text != "" {
				out = append(out, Message{Role: RoleUser, Text: c.Text, Ts: ts})
			}
		}
	}
	return out
}

// piEntry mirrors pi session-manager JSONL entries.
type piEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   *piMsg `json:"message,omitempty"`
}

// piMsg captures what we need. Content is polymorphic (string OR array)
// so we use a custom UnmarshalJSON.
type piMsg struct {
	Role         string      `json:"role"`
	Content      string      `json:"-"` // populated from string form
	ContentItems []piContent `json:"-"` // populated from array form
}

type piContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// UnmarshalJSON handles the polymorphic content field.
func (m *piMsg) UnmarshalJSON(data []byte) error {
	var aux struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Role = aux.Role
	if len(aux.Content) == 0 {
		return nil
	}
	switch aux.Content[0] {
	case '"':
		var s string
		if err := json.Unmarshal(aux.Content, &s); err != nil {
			return err
		}
		m.Content = s
	case '[':
		var arr []piContent
		if err := json.Unmarshal(aux.Content, &arr); err != nil {
			return err
		}
		m.ContentItems = arr
	}
	return nil
}
