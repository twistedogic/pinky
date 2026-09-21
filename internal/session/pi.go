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

// debug controls whether session-discovery diagnostics are printed to
// stderr. Enabled by the PINKY_DEBUG env var. Off by default.
func debug() bool {
	return os.Getenv("PINKY_DEBUG") != ""
}

func debugf(format string, args ...any) {
	if debug() {
		fmt.Fprintf(os.Stderr, "pinky: "+format+"\n", args...)
	}
}

// piSource tails a pi agent session JSONL file.
type piSource struct {
	path   string
	offset int64
}

func openPi(pid int, cwd string) (*piSource, error) {
	// 1. PI_SESSION_FILE is the canonical fast path.
	if path, ok := envValue(pid, "PI_SESSION_FILE"); ok && path != "" {
		if _, err := os.Stat(path); err == nil {
			debugf("openPi(%d): using PI_SESSION_FILE=%s", pid, path)
			return &piSource{path: path}, nil
		}
		debugf("openPi(%d): PI_SESSION_FILE set but stat failed", pid)
	} else {
		debugf("openPi(%d): PI_SESSION_FILE not set in env", pid)
	}
	// 2. cwd-based discovery (pi-mono convention, used by plannotator):
	//    <sessions>/--<encoded cwd>--/<timestamp>_<uuid>.jsonl
	//    This is reliable because the pane's cwd is what pi uses to
	//    pick the bucket directory.
	if cwd != "" {
		if path, err := openPiByCwd(cwd); err == nil {
			debugf("openPi(%d): discovered via cwd %s: %s", pid, cwd, path)
			return &piSource{path: path}, nil
		} else {
			debugf("openPi(%d): cwd-based discovery failed: %v", pid, err)
		}
	}
	// 3. Last resort: lsof on the process. Some setups (containers,
	//    wrapped agents) keep the session file open even when the cwd
	//    bucket is empty or mis-located.
	if path, err := piSessionFile(pid); err == nil {
		debugf("openPi(%d): discovered via lsof: %s", pid, path)
		return &piSource{path: path}, nil
	} else {
		debugf("openPi(%d): lsof fallback failed: %v", pid, err)
	}
	return nil, fmt.Errorf("pi process %d: no session file found\n\n"+
		"troubleshooting:\n"+
		"  - PI_SESSION_FILE not set in the process env\n"+
		"  - cwd-based lookup under %s found nothing\n"+
		"  - lsof on pid %d found no open .jsonl\n"+
		"  - run with PINKY_DEBUG=1 for verbose discovery output\n"+
		"  - bypass with --session-file /path/to/session.jsonl",
		pid, piAgentDirOrDefault(), pid)
}

func piAgentDirOrDefault() string {
	dir, err := piSessionDir()
	if err != nil {
		return "<unresolved>"
	}
	return dir
}

// openPiByCwd finds pi's session file via the cwd-based convention
// (see plannotator's pi discovery). Bucket directory:
// `<sessions>/--<encoded cwd>--/`. Newest .jsonl wins.
func openPiByCwd(cwd string) (string, error) {
	sessions, err := piSessionDir()
	if err != nil {
		return "", err
	}
	bucket := filepath.Join(sessions, piEncodedDir(cwd))
	return newestJSONL(bucket)
}

// piSessionFile finds an open .jsonl session file for pid by parsing
// `lsof` output. Picks the most recently modified match.
func piSessionFile(pid int) (string, error) {
	cmd := exec.Command("lsof", "-p", fmt.Sprintf("%d", pid), "-a", "-F", "n")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("lsof failed (is lsof installed?): %w", err)
	}
	if debug() {
		debugf("lsof -p %d (raw):\n%s", pid, indent(string(out)))
	}
	got := parseLsofJSONL(string(out))
	if got == "" {
		return "", fmt.Errorf("no .jsonl session file open (PI_SESSION_FILE not set and no .jsonl file is open)")
	}
	if _, err := os.Stat(got); err != nil {
		return "", fmt.Errorf("lsof-discovered session %q: %w", got, err)
	}
	return got, nil
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("    " + line + "\n")
	}
	return b.String()
}

// parseLsofJSONL extracts the most recently modified .jsonl file path
// from `lsof -F n` output. Lines not starting with `n` are skipped.
// The `n` prefix is stripped; the rest is taken as the path. Files
// that don't exist on disk are skipped.
//
// Exposed at package scope so tests can drive the parser without
// spawning real processes.
func parseLsofJSONL(lsofOut string) string {
	var best string
	var bestMod time.Time
	sc := bufio.NewScanner(strings.NewReader(lsofOut))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "n") {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		if !strings.HasSuffix(path, ".jsonl") {
			continue
		}
		if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, string(filepath.Separator)) {
			// Defensive: skip relative paths that lsof shouldn't emit.
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
	return best
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
