## MODIFIED Requirements

### Requirement: Block-level annotation via `c`

The system SHALL allow the user to attach a free-text comment to
the currently-focused block in the nav view. Pressing `c` in nav
while a block is focused SHALL open a comment composer
pre-anchored to that whole block (anchor `(BlockIdx, 0,
len(Source))`). Pressing `Enter` in the comment composer SHALL
save the new comment with anchor `(BlockIdx, -1, -1)`
(block-level sentinel for "no inline byte offsets") and SHALL
return to the nav view. `Enter` SHALL NOT dispatch any comments
to the agent — staging and sending are decoupled. The flush
happens via `s` in `stateNav` (see `### Requirement: One-shot
submit all comments via \`s\``). Cancelling (`Esc`) SHALL discard
the composer text and return to the nav view without creating or
sending a comment.

#### Scenario: `c` opens composer for current block

- **WHEN** the user presses `c` in `stateNav` and a block is
  focused
- **THEN** the TUI enters comment composer state with the
  composer textarea focused and empty

#### Scenario: Enter saves the comment and returns to nav without sending

- **WHEN** the user is in comment composer state with text in
  the textarea and presses `Enter`
- **THEN** a comment is stored with anchor `(BlockIdx, -1, -1)`
  where `BlockIdx` is the index of the currently-focused block,
  the TUI returns to `stateNav`, and **no** `sendToPane` call
  is made

#### Scenario: Multiple `c → Enter` cycles accumulate comments in memory

- **WHEN** the user repeats `c → type → Enter` twice, producing
  two comments anchored to different blocks
- **THEN** after the second `Enter`, `m.comments` contains both
  comments in insertion order, the TUI is in `stateNav`, and
  `sendToPane` has not been called

#### Scenario: Enter on empty composer is a no-op

- **WHEN** the user is in comment composer state with an empty
  textarea and presses `Enter`
- **THEN** no comment is stored, `sendToPane` is not called,
  and the TUI returns to `stateNav`

#### Scenario: Cancel discards composer

- **WHEN** the user is in comment composer state and presses `Esc`
- **THEN** no comment is stored, no inject call is made, the TUI
  returns to nav state, and the previously-focused block is
  unchanged

#### Scenario: `c` with no focused block is a no-op

- **WHEN** the user presses `c` in `stateNav` and no block is
  currently focused (empty state or cursor outside any block)
- **THEN** the TUI remains in `stateNav` and no composer is
  opened

### Requirement: One-shot submit all comments via `s`

Pressing `s` in `stateNav` SHALL dispatch every comment currently in
`m.comments` as a single redirect through the existing inject
pipeline, without requiring the user to enter compose mode or
type any text. The redirect payload SHALL be exactly the
comments appendix body (no leading user text and **no leading
`---` separator** — `render.FormatCommentsAppendix` emits only
the count line and entries). On successful send the comment
slice SHALL be cleared; on inject failure the comments SHALL be
kept so the user can retry and the error SHALL be surfaced via
the same `[send failed: ...]` placeholder the compose path uses.

This requirement is the **primary** send path for comments;
`Enter` in the comment composer only stages (see `### Requirement:
Block-level annotation via \`c\``).

#### Scenario: `s` submits all accumulated comments

- **WHEN** the user presses `s` in `stateNav` and there is at
  least one comment
- **THEN** the inject pipeline receives a payload equal to
  `render.FormatCommentsAppendix(comments, blocks)`, the comment
  slice is cleared, and no compose-mode interaction is required

#### Scenario: `s` with no comments is a no-op

- **WHEN** the user presses `s` in `stateNav` and the comment
  slice is empty
- **THEN** no inject call is made and no state changes

#### Scenario: send failure keeps comments

- **WHEN** the user presses `s` and the inject call returns an
  error
- **THEN** the comment slice is NOT cleared and the error is
  surfaced via the same `[send failed: ...]` placeholder the
  compose path uses

### Requirement: Single-line comment composer

The comment composer SHALL be a single-line textarea. `Enter` SHALL
save the comment to memory and return to `stateNav` (see `###
Requirement: Block-level annotation via \`c\``); `Enter` SHALL NOT
insert a newline and SHALL NOT dispatch any comments to the agent.
`Esc` SHALL cancel as before. Pasted text containing newlines SHALL
be accepted verbatim and committed on `Enter`.

#### Scenario: Enter saves without inserting a newline

- **WHEN** the user is in comment composer state and presses `Enter`
- **THEN** the composer is committed, the TUI returns to nav state,
  and the saved comment text contains no newline characters

#### Scenario: Enter does not dispatch any comments

- **WHEN** the user is in comment composer state with text in
  the textarea and presses `Enter`
- **THEN** no `sendToPane` call is made — staging and sending
  are decoupled

#### Scenario: Pasted multiline text is committed on Enter

- **WHEN** the user pastes text containing one or more newlines
  into the comment composer and presses `Enter`
- **THEN** the saved comment text contains the pasted newlines
  verbatim and the TUI returns to nav state

