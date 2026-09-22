## MODIFIED Requirements

### Requirement: Block-level annotation via `c`

The system SHALL allow the user to attach a free-text comment to
the currently-focused block in the nav view. Pressing `c` in nav
while a block is focused SHALL open a comment composer
pre-anchored to that whole block (anchor `(BlockIdx, 0,
len(Source))`). Saving the composer (`Enter`) SHALL store the
comment with anchor `(BlockIdx, -1, -1)` (block-level sentinel
for "no inline byte offsets") and return to the nav view.
Cancelling (`Esc`) SHALL discard the composer text and return to
the nav view without creating a comment.

#### Scenario: `c` opens composer for current block

- **WHEN** the user presses `c` in `stateNav` and a block is
  focused
- **THEN** the TUI enters comment composer state with the
  composer textarea focused and empty

#### Scenario: Save creates a block-level comment

- **WHEN** the user is in comment composer state with text in
  the textarea and presses `Enter`
- **THEN** a comment is stored with anchor `(BlockIdx, -1, -1)`
  where `BlockIdx` is the index of the currently-focused block,
  the TUI returns to nav state, and the block becomes flagged as
  commented

#### Scenario: Cancel discards composer

- **WHEN** the user is in comment composer state and presses `Esc`
- **THEN** no comment is stored, the TUI returns to nav state,
  and the previously-focused block is unchanged

#### Scenario: `c` with no focused block is a no-op

- **WHEN** the user presses `c` in `stateNav` and no block is
  currently focused (empty state or cursor outside any block)
- **THEN** the TUI remains in `stateNav` and no composer is
  opened

### Requirement: Include comments in redirect

While in compose state, pressing `i` SHALL toggle whether
the queued redirect will be sent with a comments appendix. When
the flag is enabled and the user sends the redirect (`s`), the
inject payload SHALL be the redirect text followed by a separator
line (`---`) and a numbered list of all current comments in
chronological order (oldest first). Each entry SHALL include the
comment kind (`block` or `inline`), a short quote of the source
text (block source or inline excerpt, truncated to ~40 chars
with an ellipsis if longer), the block's line range, and the
comment text.

#### Scenario: `i` toggles include flag

- **WHEN** the user is in compose state and presses `i`
- **THEN** the include-comments flag is toggled, and the status
  line reflects the new state (e.g., `[I] include N comments —
  ON/OFF`)

#### Scenario: `s` with flag ON appends appendix

- **WHEN** the user is in compose state, the include flag is ON,
  and the user presses `s` with non-empty text
- **THEN** the inject payload sent to the agent's pane is
  `<redirect text>\n\n---\nN comments:\n- <comment 1>\n- <comment 2>\n...`
  where each comment line follows the format
  `block "<excerpt>" (lines X-Y): <text>` or
  `inline "<excerpt>" (line Z): <text>`

#### Scenario: `s` with flag OFF sends plain redirect

- **WHEN** the user is in compose state, the include flag is OFF,
  and the user presses `s` with non-empty text
- **THEN** the inject payload is exactly the redirect text with
  no comments appendix

#### Scenario: `s` with flag ON and zero comments sends plain redirect

- **WHEN** the user is in compose state, the include flag is ON,
  and there are no comments, and the user presses `s`
- **THEN** the inject payload is exactly the redirect text (no
  appendix because the count is zero)

## ADDED Requirements

### Requirement: Single-line comment composer

The comment composer SHALL be a single-line textarea. `Enter` SHALL
save the comment (see `### Requirement: Block-level annotation via
\`c\``) and SHALL NOT insert a newline. `Esc` SHALL cancel as
before. Pasted text containing newlines SHALL be accepted
verbatim and committed on `Enter`.

#### Scenario: Enter saves without inserting a newline

- **WHEN** the user is in comment composer state and presses `Enter`
- **THEN** the composer is committed, the TUI returns to nav state,
  and the saved comment text contains no newline characters

#### Scenario: Pasted multiline text is committed on Enter

- **WHEN** the user pastes text containing one or more newlines
  into the comment composer and presses `Enter`
- **THEN** the saved comment text contains the pasted newlines
  verbatim and the TUI returns to nav state