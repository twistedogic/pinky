# agent-redirect

## Purpose

TBD
## Requirements
### Requirement: Show session picker at startup
The system SHALL start by enumerating all tmux panes whose process tree contains a pi or codex agent, and present them in a picker. The user selects one with arrow keys + Enter; pinky then transitions to its running state bound to that pane.

#### Scenario: Multiple agent sessions found
- **WHEN** pinky starts without `--target` and two or more tmux panes contain a pi or codex agent
- **THEN** the picker SHALL list all such sessions with their tmux session name, pane id, and agent name

#### Scenario: Single agent session found
- **WHEN** pinky starts without `--target` and exactly one tmux pane contains an agent
- **THEN** the picker SHALL show that single entry; user selects it with Enter to proceed

#### Scenario: No agent sessions found
- **WHEN** pinky starts without `--target` and no tmux pane contains a pi or codex agent
- **THEN** pinky SHALL exit with a clear error: "no active pi or codex agents found in any tmux pane"

#### Scenario: --target bypasses the picker
- **WHEN** pinky starts with `--target <pane-id>`
- **THEN** the picker SHALL NOT be shown; pinky proceeds directly to running state bound to that pane

#### Scenario: Picker navigation
- **WHEN** the picker is shown
- **THEN** `↑`/`↓` SHALL move the selection cursor (wrapping), `Enter` SHALL confirm, `Ctrl+C` SHALL quit

### Requirement: Detect agent process in tmux pane
The system SHALL detect which agent is running in the target tmux pane (pi or codex) by walking the pane's process descendants and reading each process's executable name. Pinky SHALL error out at startup if the agent is neither pi nor codex — no fallback to other agents or to `tmux capture-pane`.

#### Scenario: pi running in pane
- **WHEN** the target pane's process tree contains a process named `pi`
- **THEN** pinky opens that pi process's session file (via `PI_SESSION_FILE` env var) and begins tailing it

#### Scenario: codex running in pane
- **WHEN** the target pane's process tree contains a process named `codex`
- **THEN** pinky opens that codex process's active session file (via `lsof`) under `~/.codex/sessions/` and begins tailing it

### Requirement: Tail agent session JSONL
The system SHALL continuously tail the agent's session JSONL file and surface new assistant messages in the latest-message view within one second of their appearance. Implementation tracks a per-source byte offset and only returns content appended since the previous poll.

#### Scenario: New assistant text appears
- **WHEN** the agent appends a new assistant text message to its session file
- **THEN** the message replaces the current message in the latest-message view

#### Scenario: Polling yields no new content
- **WHEN** no new lines have been appended to the session file
- **THEN** pinky does not modify the latest-message view

#### Scenario: pi message with thinking + toolCall + text
- **WHEN** pi writes a message entry whose `content` array contains `thinking`, `toolCall`, and `text` blocks
- **THEN** only the `text` block is surfaced; thinking and toolCall blocks are skipped

#### Scenario: codex assistant message
- **WHEN** codex writes a `response_item` with `payload.role: assistant` and `content: [{type: output_text, text: ...}]`
- **THEN** the `output_text` content is surfaced

#### Scenario: Manual refresh via `r`
- **WHEN** the user presses `r` in `stateNav`
- **THEN** the session source is re-polled from its current offset and the latest-message view is refreshed with any newly surfaced content

### Requirement: Compose multi-line redirect
The system SHALL provide a multi-line text input area where the user composes a redirect message. `Enter` inserts a newline; `s` sends.

#### Scenario: Enter inserts newline
- **WHEN** the user presses `Enter` in compose mode
- **THEN** a newline is inserted at the cursor position and the message is NOT sent

#### Scenario: Send via `s`
- **WHEN** the user presses `s` in compose mode with non-empty input
- **THEN** the composed text is sent to the target pane via tmux paste-buffer + send-keys

#### Scenario: Cancel via Esc
- **WHEN** the user presses `Esc` in compose mode
- **THEN** the compose buffer is discarded and the TUI returns to nav

#### Scenario: Empty input cannot be sent
- **WHEN** the compose buffer is empty and the user presses `s`
- **THEN** the system SHALL NOT send (no-op)

### Requirement: Inject composed redirect into target pane
The system SHALL inject composed text into the target tmux pane using `tmux set-buffer` + `paste-buffer` + `send-keys Enter`, preserving all characters including newlines and Unicode.

#### Scenario: Multi-line redirect sent
- **WHEN** the user sends a redirect containing newlines
- **THEN** the entire text including newlines appears verbatim in the target pane's input

### Requirement: Persist and restore history
The system SHALL append every surfaced assistant message and every user-sent redirect to a JSONL history file at `$XDG_DATA_HOME/pinky/history.jsonl` (fallback `~/.local/share/pinky/history.jsonl`). On startup, existing history SHALL NOT be loaded into the scrollback view — pinky starts fresh and surfaces only newly-arriving content from the live session.

#### Scenario: Restart continuity
- **WHEN** `pinky` restarts
- **THEN** the latest-message view starts empty (or with the placeholder) and waits for the next assistant message from the live session; existing history is NOT seeded into the view

#### Scenario: History entry schema
- **WHEN** a line is recorded
- **THEN** the entry contains `ts` (timestamp), `pane_id` (target tmux pane), `role` (`agent` | `user`), and `text` (the line content)

### Requirement: tmux-only runtime
The system SHALL require a running tmux server and a target pane at startup. tmux is the only supported multiplexer — no other terminal multiplexer is detected, supported, or planned. The system SHALL exit with a clear error if `$TMUX` is unset or the target pane does not exist.

#### Scenario: tmux not running
- **WHEN** `$TMUX` is unset
- **THEN** pinky exits with a message indicating tmux is required

#### Scenario: Target pane missing
- **WHEN** the resolved target pane ID does not exist
- **THEN** pinky exits with a message naming the missing pane
