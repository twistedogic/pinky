## MODIFIED Requirements

### Requirement: Compose multi-line redirect

The system SHALL provide a multi-line text input area where the
user composes a redirect message. `Enter` SHALL insert a newline.
The user SHALL enter compose mode from the nav surface by
pressing `n` (replacing the previous `Ctrl+N` trigger). The
user SHALL send the composed redirect by pressing `s`
(replacing the previous `Ctrl+S` send key). The user SHALL
cancel the compose buffer by pressing `Esc`. The `Ctrl+S`
binding SHALL NOT exist in compose mode (it is now reserved
for saving inside `stateCommentComposer`).

When `includeComments` is on, the sent text SHALL include a
trailing comments appendix as described in the `message-comments`
capability.

#### Scenario: User enters compose via `n`

- **WHEN** the user presses `n` in `stateNav`
- **THEN** the TUI transitions to `stateCompose` with the
  textarea reset and focused

#### Scenario: Enter inserts newline

- **WHEN** the user presses `Enter` in compose mode
- **THEN** a newline is inserted at the cursor position and the
  message is NOT sent

#### Scenario: Send via `s`

- **WHEN** the user presses `s` in compose mode with non-empty
  input
- **THEN** the composed text is sent to the target pane via
  tmux paste-buffer + send-keys; if `includeComments` is on and
  there is at least one comment, the comments appendix is
  appended to the outgoing text before send

#### Scenario: Cancel via Esc

- **WHEN** the user presses `Esc` in compose mode
- **THEN** the compose buffer is discarded and the TUI returns
  to `stateNav`

#### Scenario: Empty input cannot be sent

- **WHEN** the compose buffer is empty and the user presses `s`
- **THEN** the system SHALL NOT send (no-op)

#### Scenario: Ctrl+S no longer sends in compose

- **WHEN** the user presses `Ctrl+S` in compose mode
- **THEN** the key is delivered to the textarea as a literal
  character; no send is triggered

### Requirement: Manual refresh via single letter `r`

In `stateNav`, the user SHALL trigger a manual refresh of the
tailed session by pressing `r`. The session source SHALL be
re-polled from its current offset and the latest-message view
SHALL be re-rendered with any newly surfaced content. The
previous `Ctrl+R` binding SHALL NOT exist.

#### Scenario: Refresh via `r`

- **WHEN** the user presses `r` in `stateNav`
- **THEN** the session source is re-polled from its current
  offset and the latest-message view is refreshed with any
  newly surfaced content
