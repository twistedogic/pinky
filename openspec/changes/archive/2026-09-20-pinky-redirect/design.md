## Context

Empty Go repo (`github.com/twistedogic/pinky` declared in `go.mod`, no source yet, Go 1.26.4). OpenSpec is set up with `spec-driven` schema but no capabilities defined. Target user runs AI coding agents (`pi`, `codex`) inside `tmux` and wants a low-friction way to send precise push-backs without re-stating context the agent just had.

## Goals / Non-Goals

**Goals:**
- Detect the agent process in the tmux pane (pi or codex only) and tail that agent's session JSONL for assistant messages.
- Provide a multi-line compose buffer with a clear send affordance.
- Inject composed text into the agent pane via `tmux set-buffer` + `paste-buffer` + `send-keys Enter`.
- Persist a JSONL history of captured agent lines and user-sent redirects.
- Stay under ~600 lines of Go; no config file in v0.
- tmux 3.0+ only.

**Non-Goals:**
- Reading agent scrollback via `tmux capture-pane` (session files are cleaner and structured).
- Any agent other than `pi` or `codex`. Unknown agents error out at startup.
- Watching multiple agent panes from one pinky instance.
- Tool calls / thinking blocks rendered in pinky (filtered at parse time).
- "Agent done speaking" detection / notifications / cursor awareness.
- Slash-commands, templates, structured redirect formats.
- Markdown rendering of captured content.
- Config file.
- Authentication, networking, telemetry.

## Decisions

1. **TUI library: `bubbletea` + `lipgloss` + `bubbles`.** Standard for Go TUIs; sufficient for the state machine, viewport, and styling.
2. **Agent detection: `tmux display-message -p '#{pane_pid}'`, then walk the descendant tree (`pgrep -P`, `ps -c -o comm=`) looking for a process named `pi` or `codex`.** `ps -c` strips the full path on macOS so `/tmp/pi` matches just as `pi` does.
2a. **Session picker at startup: `tmux list-panes -a -F '#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_id}\t#{pane_pid}'`, then walk each pane's descendants looking for pi/codex.** Only panes with a matching agent are listed; non-agent panes are silently filtered out. If zero agents are found and `--target` wasn't passed, pinky exits with a clear error. The picker is a custom list (no need for bubbles/list — too heavy for one screen). `--target <pane-id>` bypasses the picker entirely for scripting/testing.
3. **Session file discovery, per agent:**
   - `pi`: read `PI_SESSION_FILE` from the pi process's environment (via `/proc/<pid>/environ` on Linux or `ps wwE` on macOS).
   - `codex`: use `lsof -p <pid>` to find open JSONL files under `~/.codex/sessions/`. Pick the most recently modified one.
4. **Reading: tail the JSONL file by tracking byte offset. Each `NewMessages()` call returns only content appended since the previous call.** Polling interval 500ms.
5. **Parser, per agent:**
   - `pi`: extract `{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"..."}]}}`. Skip `thinking` and `toolCall` content blocks. Handle the polymorphic `content` field (string for some messages, array for others) via a custom unmarshaler.
   - `codex`: extract `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"..."}]}}`.
6. **Send mechanism: `set-buffer` → `paste-buffer` → `send-keys -t <pane> Enter`.** Standard tmux injection; preserves newlines and Unicode.
7. **Target pane: resolved at startup from `$TMUX_PANE` env var, overridable via `--target` flag.** Pinned — pinky does not retarget after start.
8. **History: append-only JSONL at `$XDG_DATA_HOME/pinky/history.jsonl`.** Entries tagged with `pane_id`, `timestamp`, `role` (`agent` | `user`). Re-seeded into the TUI on startup.
9. **Multi-line input: `Enter` = newline, `Ctrl+S` = send, `Esc` = cancel.** `Ctrl+S` chosen because modern bubbletea cannot distinguish `Ctrl+Enter` from plain `Enter` (both produce the same terminal byte).
10. **Layout: vertical split, ~35% width.** User wires it up via their own tmux binding (`bind-key p split-window -h -l 35% pinky`); pinky itself does not manage layout.
11. **Refresh: manual via `Ctrl+R`** (clears scrollback, re-polls). Auto-refresh isn't needed since session files are append-only (no pane-clear events to detect).
12. **No speculative flexibility.** Default values are the right values for v0.

## Risks / Trade-offs

- **Process tree walking is heuristic.** A future pi/codex that runs as a grandchild via multiple shell layers might be missed; mitigated by full BFS through descendants (not just direct children).
- **`/proc/<pid>/environ` is Linux-only.** On macOS we fall back to `ps wwE`, which works but is slower and slightly more fragile to ps output format changes.
- **`lsof` for codex session discovery** depends on the codex process having the session file open at the moment pinky starts. If codex hasn't written anything yet, we error and the user can retry once codex has.
- **Polling interval 500ms** means up to 500ms latency before new messages appear. Acceptable for human-scale interaction.
- **`lsof` is macOS/Linux-specific.** tmux itself is the only OS dependency, and lsof ships with both. If we ever need Windows, this changes.
- **`pinky` writes to the tmux paste buffer.** If something else is holding the buffer, behaviour is fine (tmux buffers are per-server) but worth noting.
- **Pinky crash loses in-memory scrollback** of post-restart messages (history file persists; session file is re-read from offset 0 on restart, deduplicated by history seed).
