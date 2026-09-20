## Why

When using AI coding agents (`pi`, `codex`), users regularly need to correct or redirect the agent's last message. Typing the correction by hand means restating context the agent just had — slow and lossy. A purpose-built tool that captures the agent's last message and lets the user compose a precise redirect — then injects it back into the agent's input — preserves precision and reduces friction.

For v0, pinky supports exactly two agents: `pi` and `codex`. Both write session state to JSONL files (`pi` via `$PI_SESSION_FILE`, `codex` under `~/.codex/sessions/`). Reading from those files is cleaner than scraping `tmux capture-pane` — no ANSI parsing, structured message boundaries, and direct access to the agent's true output.

## What Changes

New Go binary `pinky` (Go module `github.com/twistedogic/pinky`) that runs as a persistent TUI in a tmux split-pane alongside an agent TUI. It detects which agent is running in the pane, tails that agent's session JSONL, displays the assistant's messages, and lets the user compose a multi-line redirect that is injected back into the agent's input via `tmux paste-buffer` + `send-keys`. History is persisted to a JSONL file under `$XDG_DATA_HOME`.

## Capabilities

### New Capabilities
- `agent-redirect`: detect pi or codex in a tmux pane, tail their session JSONL for assistant messages, render them in a persistent TUI, and inject user-composed redirects back into the pane.

### Modified Capabilities
None.

## Impact

- New module `github.com/twistedogic/pinky` (already declared in `go.mod`)
- New binary `pinky` at `main.go` (project root)
- Dependencies: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/bubbles`
- External: requires `tmux` 3.0+ and `lsof` on `PATH` (for codex session discovery). On Linux, `/proc` is read for process env.
- Storage: appends to `$XDG_DATA_HOME/pinky/history.jsonl` (fallback `~/.local/share/pinky/history.jsonl`)
- No config file in v0; runtime knobs via flags only

**Out of scope (v0):** any agent other than pi or codex (errors out); `tmux capture-pane`-based capture; zellij or any other multiplexer (tmux is the only supported multiplexer, period); tool call / thinking block rendering; agent "done speaking" detection; multi-pane watching; templates/slash-commands; markdown rendering; config file; networking; telemetry.
