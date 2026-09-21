## MODIFIED Requirements

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

#### Scenario: Manual refresh via Ctrl+R
- **WHEN** the user presses `Ctrl+R` in idle state
- **THEN** the session source is re-polled from its current offset and the latest-message view is refreshed with any newly surfaced content

### Requirement: Persist and restore history
The system SHALL append every surfaced assistant message and every user-sent redirect to a JSONL history file at `$XDG_DATA_HOME/pinky/history.jsonl` (fallback `~/.local/share/pinky/history.jsonl`). On startup, existing history SHALL NOT be loaded into the scrollback view — pinky starts fresh and surfaces only newly-arriving content from the live session.

#### Scenario: Restart continuity
- **WHEN** `pinky` restarts
- **THEN** the latest-message view starts empty (or with the placeholder) and waits for the next assistant message from the live session; existing history is NOT seeded into the view

#### Scenario: History entry schema
- **WHEN** a line is recorded
- **THEN** the entry contains `ts` (timestamp), `pane_id` (target tmux pane), `role` (`agent` | `user`), and `text` (the line content)
