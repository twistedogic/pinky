## MODIFIED Requirements

### Requirement: Block-level annotation via `c`

The system SHALL allow the user to attach a free-text comment to
the currently-focused block in the nav view. Pressing `c` in nav
while a block is focused SHALL open a comment composer
pre-anchored to that whole block (anchor `(BlockIdx, 0,
len(Source))`). Pressing `Enter` in the comment composer SHALL
save the new comment with anchor `(BlockIdx, -1, -1)`
(block-level sentinel for "no inline byte offsets"), return to
the nav view, AND dispatch every accumulated comment (the new
one plus any prior ones since the last successful send) to the
agent in a single tmux inject via the existing `submitAllComments`
path. Cancelling (`Esc`) SHALL discard the composer text and
return to the nav view without creating or sending a comment.

#### Scenario: `c` opens composer for current block

- **WHEN** the user presses `c` in `stateNav` and a block is
  focused
- **THEN** the TUI enters comment composer state with the
  composer textarea focused and empty

#### Scenario: Enter sends all comments in one batch

- **WHEN** the user is in comment composer state with text in
  the textarea and presses `Enter`
- **THEN** a comment is stored with anchor `(BlockIdx, -1, -1)`
  where `BlockIdx` is the index of the currently-focused block,
  the entire `m.comments` slice is dispatched to the agent pane
  as one tmux paste-buffer + send-keys (via `submitAllComments`),
  the TUI returns to nav state, and the block becomes flagged as
  commented

#### Scenario: Enter sends multi-comment batch in one inject

- **WHEN** the user is in comment composer state with N comments
  already accumulated, types a new comment, and presses `Enter`
- **THEN** the new comment is appended to `m.comments` (now
  N+1 entries), the entire slice is dispatched in a single
  `sendToPane` call, and `m.comments` is cleared on successful
  send

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
comments appendix (no leading user text). On successful send the
comment slice SHALL be cleared; on inject failure the comments
SHALL be kept so the user can retry and the error SHALL be
surfaced via the same `[send failed: ...]` placeholder the
compose path uses.

This requirement now describes the retry path: the primary
send path is `Enter` in the comment composer (see `### Requirement:
Block-level annotation via \`c\``). `s` in nav is the recovery
mechanism when a previous `Enter`-triggered send failed and
`m.comments` was kept.

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