## 1. Scaffold

- [x] 1.1 Confirm `go.mod` is `github.com/twistedogic/pinky` (Go 1.26.4); otherwise initialize
- [x] 1.2 Add `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/lipgloss` to `go.mod`
- [x] 1.3 Create `main.go` at project root with a stub `main()` that exits 0
- [x] 1.4 `go build ./...` succeeds

## 2. Session reader (replaces capture-pane)

- [x] 2.1 `internal/session/session.go`: detect agent process in tmux pane via pane_pid + tree walk
- [x] 2.2 `internal/session/pi.go`: tail pi session file via `PI_SESSION_FILE` env var; parse pi's JSONL (assistant text content)
- [x] 2.3 `internal/session/codex.go`: find codex session file via `lsof -p <pid>`; parse codex rollout JSONL (assistant output_text)
- [x] 2.4 `Source` interface (`NewMessages() ([]Message, error)`, `Close() error`) with offset tracking
- [x] 2.5 Strict scope: error out if pane's agent is not pi or codex (no fallback)
- [x] 2.6 Unit tests for both parsers (sample JSONL → expected messages)
- [x] 2.7 Polling loop on a 500ms tick; NewMessages returns only new content since last call
- [x] 2.8 Manual refresh keybinding: `Ctrl+R` clears scrollback and re-polls
- [x] 2.9 `session.ListAgents()`: enumerate all tmux panes, filter to those with pi/codex descendants
- [x] 2.10 Session picker TUI: `statePicking` state, ↑/↓ cursor, Enter selects, Ctrl+C quits
- [x] 2.11 `--target <pane-id>` flag bypasses picker and jumps directly to running state

## 3. Inject

- [x] 3.1 `internal/inject/inject.go`: `set-buffer` → `paste-buffer` → `send-keys -t <pane> Enter`
- [x] 3.2 Resolve target pane from `$TMUX_PANE` env var, overridable via `--target <pane-id>` flag
- [x] 3.3 Validate target pane exists at startup (`tmux display-message -t <pane> -p '#{pane_id}'`); exit with clear error if not
- [x] 3.4 Refuse to run if `$TMUX` is unset, with a one-line message

## 4. TUI

- [x] 4.1 Bubbletea model with states: `idle` and `compose`
- [x] 4.2 Scrollback viewport (agent + user lines, role-styled)
- [x] 4.3 Compose input area at bottom (multi-line, wraps)
- [x] 4.4 Keybindings: `Ctrl+N` enter compose, `Enter` newline, `Ctrl+S` send, `Esc` cancel, `Ctrl+R` refresh, `PgUp`/`PgDn` scroll, `Ctrl+C` quit
- [x] 4.5 Status line: target pane, capture lag, line count
- [x] 4.6 Subtle "streaming" indicator when capture changes between polls

## 5. History

- [x] 5.1 `internal/history/history.go`: append-only JSONL writer
- [x] 5.2 Path resolution: `$XDG_DATA_HOME/pinky/history.jsonl` → `~/.local/share/pinky/history.jsonl` fallback
- [x] 5.3 Entry schema: `{ts, pane_id, role, text}`
- [x] 5.4 On startup: load existing history, seed scrollback
- [x] 5.5 On new capture: append agent entries to history
- [x] 5.6 On send: append user entries to history before injection

## 6. Verification

- [x] 6.1 `go build ./...` succeeds with no warnings
- [x] 6.2 Manual smoke test: open `pinky` next to a real agent, send a redirect, confirm it lands in the agent's input
- [x] 6.3 README with install + tmux binding example
- [x] 6.4 Integration test: `session.ListAgents` correctly filters non-agent panes
