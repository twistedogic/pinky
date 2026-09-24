// Package session reads agent messages from session files (pi, codex).
// It detects which agent is running in a tmux pane and tails that
// agent's session JSONL for assistant messages.
package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Role identifies who produced a message.
type Role string

const (
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
)

// Message is one unit of agent output (one text block from an assistant
// turn, or one user prompt).
type Message struct {
	Role Role
	Text string
	Ts   time.Time
}

// scannerMaxLine caps agent JSONL line size at 16 MiB. The
// 16 MiB covers any plausible single assistant output and avoids
// bufio.Scanner's default 64 KiB which trips on a long streamed
// message.
const scannerMaxLine = 16 * 1024 * 1024

// Source reads new messages from an agent session. Implementations
// track their own read offset internally; NewMessages returns only
// content appended since the previous call (or since Open).
type Source interface {
	NewMessages() ([]Message, error)
	Close() error
}

// OpenFile opens a Source from an explicit JSONL path. Used as the
// --session-file escape hatch when pane-based discovery can't find the
// file. The format is auto-detected by the first line's `type` field
// (pi uses "message", codex uses "response_item").
func OpenFile(path string) (Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file %q: %w", path, err)
	}
	defer f.Close()

	// Read the first non-empty line to detect format.
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), scannerMaxLine)
	var firstLine []byte
	for sc.Scan() {
		firstLine = sc.Bytes()
		break
	}
	if firstLine == nil {
		return nil, fmt.Errorf("session file %q is empty", path)
	}

	var raw struct {
		Type    string `json:"type"`
		Payload struct {
			Type string `json:"type"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(firstLine, &raw); err != nil {
		return nil, fmt.Errorf("session file %q: not valid JSONL: %w", path, err)
	}
	switch raw.Type {
	case "message":
		return &piSource{path: path}, nil
	case "response_item":
		return &codexSource{path: path}, nil
	default:
		return nil, fmt.Errorf("session file %q: unrecognized format (type=%q)", path, raw.Type)
	}
}

// ErrUnsupportedAgent is returned when the pane's agent isn't pi or codex.
var ErrUnsupportedAgent = errors.New("unsupported agent: pinky only supports pi or codex")

// AgentSession is one running agent inside a tmux pane.
type AgentSession struct {
	Session string // tmux session name
	Window  string // window index (e.g., "0")
	Pane    string // pane index within the window (e.g., "1")
	PaneID  string // tmux pane id (e.g., "%12")
	Agent   string // "pi" or "codex"
}

// ListAgents enumerates every tmux pane whose descendant tree contains a
// pi or codex process. Picker uses this at startup to populate the list.
func ListAgents() ([]AgentSession, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_id}\t#{pane_pid}").Output()
	if err != nil {
		return nil, err
	}
	var agents []AgentSession
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) < 5 {
			continue
		}
		panePID, err := strconv.Atoi(parts[4])
		if err != nil {
			continue
		}
		info, err := findAgent(panePID)
		if err != nil {
			continue
		}
		agents = append(agents, AgentSession{
			Session: parts[0],
			Window:  parts[1],
			Pane:    parts[2],
			PaneID:  parts[3],
			Agent:   info.name,
		})
	}
	return agents, nil
}

// Open returns a Source for the agent running in the given tmux pane.
// It errors out if the agent isn't pi or codex (per design — no fallback).
func Open(tmuxPane string) (Source, error) {
	pid, err := panePID(tmuxPane)
	if err != nil {
		return nil, fmt.Errorf("pane pid: %w", err)
	}
	agent, err := findAgent(pid)
	if err != nil {
		return nil, err
	}
	cwd, _ := PaneCwd(tmuxPane) // best-effort; cwd-based discovery is one of several fallbacks
	switch agent.name {
	case "pi":
		return openPi(agent.pid, cwd)
	case "codex":
		return openCodex(agent.pid, cwd)
	default:
		return nil, fmt.Errorf("%w: found %q", ErrUnsupportedAgent, agent.name)
	}
}

// PaneCwd returns the current working directory of the given tmux pane,
// via `tmux display-message -p '#{pane_current_path}'`. Empty string on
// error (caller should treat as "cwd unknown" rather than failing).
// Exported so the TUI model can display the workspace root for the
// file review tab.
func PaneCwd(tmuxPane string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-t", tmuxPane, "-p", "#{pane_current_path}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type procInfo struct {
	pid  int
	name string
}

// panePID returns the main PID of the tmux pane.
func panePID(pane string) (int, error) {
	out, err := exec.Command("tmux", "display-message", "-t", pane, "-p", "#{pane_pid}").Output()
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse pane pid %q: %w", out, err)
	}
	return pid, nil
}

// findAgent walks descendants of root looking for pi or codex.
func findAgent(root int) (procInfo, error) {
	// Check root itself first.
	if name, err := comm(root); err == nil && (name == "pi" || name == "codex") {
		return procInfo{pid: root, name: name}, nil
	}
	// BFS through descendants.
	queue := []int{root}
	visited := map[int]bool{root: true}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		children, err := childrenOf(pid)
		if err != nil {
			continue
		}
		for _, c := range children {
			if visited[c] {
				continue
			}
			visited[c] = true
			if name, err := comm(c); err == nil && (name == "pi" || name == "codex") {
				return procInfo{pid: c, name: name}, nil
			}
			queue = append(queue, c)
		}
	}
	return procInfo{}, fmt.Errorf("%w: no pi or codex descendant of pane pid %d", ErrUnsupportedAgent, root)
}

// comm returns the executable name (basename) of pid.
// Uses `ps -c` to strip the full path on macOS.
func comm(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-c", "-o", "comm=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// childrenOf returns the direct child PIDs of pid.
func childrenOf(pid int) ([]int, error) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		// pgrep exits 1 when no children found — not an error for our purposes.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var kids []int
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if k, err := strconv.Atoi(strings.TrimSpace(sc.Text())); err == nil {
			kids = append(kids, k)
		}
	}
	return kids, nil
}

// readEnvs reads a process's environment, returning a map. macOS-friendly
// via `ps -wwE`; works on Linux too.
func readEnvs(pid int) (map[string]string, error) {
	out, err := exec.Command("ps", "wwE", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("ps output too short")
	}
	header := strings.Fields(lines[0])
	envStart := -1
	for i, h := range header {
		if h == "COMMAND" || strings.HasPrefix(h, "ARGS") {
			envStart = i
			break
		}
	}
	if envStart < 0 {
		// fallback: parse last space-separated tokens of line[1] as KEY=VAL pairs
		envStart = len(header)
	}
	tail := strings.Join(strings.Fields(lines[1])[envStart:], " ")
	// On macOS, ps output has COMMAND then env vars space-separated at the end.
	envs := map[string]string{}
	for _, kv := range strings.Fields(tail) {
		if i := strings.IndexByte(kv, '='); i > 0 {
			envs[kv[:i]] = kv[i+1:]
		}
	}
	return envs, nil
}

// readPIDEnvs reads /proc/<pid>/environ (Linux fast path). Returns nil
// on any non-Linux platform or when the file is unreadable, so the
// caller falls through to readEnvs (ps wwE).
func readPIDEnvs(pid int) map[string]string {
	if runtime.GOOS != "linux" {
		return nil
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil
	}
	envs := map[string]string{}
	for _, kv := range bytes.Split(data, []byte{0}) {
		if i := bytes.IndexByte(kv, '='); i > 0 {
			envs[string(kv[:i])] = string(kv[i+1:])
		}
	}
	return envs
}

// envValue returns the value of key from the process env, trying
// /proc/<pid>/environ first (Linux) then ps (macOS).
func envValue(pid int, key string) (string, bool) {
	if envs := readPIDEnvs(pid); envs != nil {
		if v, ok := envs[key]; ok {
			return v, true
		}
		return "", false
	}
	envs, err := readEnvs(pid)
	if err != nil {
		return "", false
	}
	v, ok := envs[key]
	return v, ok
}
