# message-comments Specification

## Purpose
TBD - created by archiving change add-block-and-inline-comments. Update Purpose after archive.
## Requirements
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

### Requirement: Character-granularity visual selection

The system SHALL provide a character-granularity visual mode for
selecting a byte range within a block. Pressing `v` in `stateNav`
SHALL enter visual mode and seed both the selection's anchor and
cursor at the cursor's current `(blockIdx, charPos)`. While in
visual mode, `h` SHALL retreat the cursor one rune within the
block, `l` SHALL advance the cursor one rune, `j` SHALL extend
the selection to the destination block's start byte, and `k`
SHALL retreat to the previous block's start byte. Pressing `Esc`
or `v` again SHALL exit visual mode without saving. Pressing `c`
SHALL open the comment composer pre-anchored to the selection
range `(selection.blockIdx, selection.charA, selection.charC)`.

#### Scenario: `v` enters visual at the cursor

- **WHEN** the user presses `v` in `stateNav`
- **THEN** the TUI enters visual line mode with the selection
  anchored at the cursor's current `(blockIdx, charPos)`

#### Scenario: `l` extends visual cursor right

- **WHEN** the user is in visual mode and presses `l`
- **THEN** the selection's `charC` advances by the byte length of
  the rune at the current `charC` offset (clamped at
  `len(block.Source)`)

#### Scenario: `h` extends visual cursor left

- **WHEN** the user is in visual mode and presses `h`
- **THEN** the selection's `charC` retreats by the byte length of
  the rune preceding the current offset (clamped at `0`)

#### Scenario: `j` extends visual selection across blocks

- **WHEN** the user is in visual mode and presses `j`
- **THEN** the cursor advances to the next block, the selection's
  `blockIdx` is updated to the new block, and `charC` is `0` (the
  start of the new block's source); the selection now spans the
  anchor to the start of the destination block

#### Scenario: `Esc` exits visual without saving

- **WHEN** the user is in visual mode and presses `Esc`
- **THEN** the TUI returns to nav state, the visual flag and
  selection are cleared, and no comment is stored

#### Scenario: `v` toggles visual off

- **WHEN** the user is in visual mode and presses `v` again
- **THEN** the TUI exits visual mode (same effect as `Esc`)

#### Scenario: `c` over a selection opens the composer

- **WHEN** the user is in visual mode and presses `c`
- **THEN** the TUI enters comment composer state with the
  composer textarea focused and empty, and the saved anchor is
  the selection range `(selection.blockIdx, selection.charA,
  selection.charC)`

### Requirement: Inline selection anchors by byte offset

When the comment composer is opened from visual mode, the saved
anchor SHALL be `(BlockIdx, CharStart, CharEnd)` where
`BlockIdx` is the index of the block containing the selection,
`CharStart` is `selection.charA` (a byte offset into the block's
source), and `CharEnd` is `selection.charC` (a byte offset into
the block's source). The byte offsets are the verbatim anchor
set by `v` and extended by `h`/`l`/`j`/`k`; no further
projection through goldmark `Lines()` segments is required
because the cursor already lives in source-byte space.

#### Scenario: Inline anchor uses byte offsets

- **WHEN** the user opens the comment composer from visual mode
  and saves
- **THEN** the saved comment has `Kind = BlockParagraph` (or
  whatever the containing block's kind is), `BlockIdx` matches
  the selection's block, and `CharStart` and `CharEnd` are
  non-negative byte offsets into the block's source

#### Scenario: Inline anchor preserves source text

- **WHEN** a comment is saved with an inline anchor
- **THEN** the verbatim slice `block.Source[CharStart:CharEnd]`
  is stored alongside the comment so the original text can be
  quoted in the redirect appendix

### Requirement: Auto-scroll during cursor motion

In `stateNav`, when the cursor moves such that its rendered line
is no longer inside the viewport's visible range, the viewport
SHALL scroll so that the cursor's line sits inside the visible
range with a 1-line cushion above and below. When the cursor
moves within the visible range, the viewport SHALL be left
alone.

#### Scenario: Motion off-screen scrolls the viewport

- **WHEN** the cursor moves (via `j`, `k`, `h`, `l`) such that
  its rendered line is not inside `[YOffset, YOffset + Height)`
- **THEN** `YOffset` is updated so the cursor's line is visible
  with the 1-line cushion preserved

#### Scenario: Motion within the visible range leaves YOffset alone

- **WHEN** the cursor moves and its rendered line is already
  inside `[YOffset, YOffset + Height)`
- **THEN** `YOffset` is unchanged

### Requirement: Comment footnote rendering

The system SHALL render saved comments as footnote lines
immediately following the block they annotate. Each comment
footnote SHALL be prefixed with a comment marker (`▸` for
block-level comments, `•` for inline comments) and SHALL display
the comment text. The rendered line range of the commented block
SHALL also receive a background colour tint to make the commented
region visually distinct. The commented block SHALL be marked in
the left-margin gutter column with the character `▍` in yellow
(foreground colour `228`) on every rendered line that falls
inside the block's `StartLine..EndLine` range, regardless of
whether the block is currently focused. No first-line `▸` or `•`
glyph SHALL be prepended to the block's first rendered line in
the document body.

#### Scenario: Footnote appears below commented block

- **WHEN** a block-level comment is saved
- **THEN** the rendered view shows the block's content followed
  by a footnote line containing the `▸` marker and the comment
  text, before the next block's content begins

#### Scenario: Inline comment footnote includes excerpt

- **WHEN** an inline comment is saved
- **THEN** the footnote line contains the `•` marker and the
  comment text, and the verbatim source excerpt stored with the
  comment is included in the footnote for context

#### Scenario: Yellow gutter spans the commented block

- **WHEN** a block has at least one comment
- **THEN** every rendered line within that block's
  `StartLine..EndLine` range has a yellow `▍` in its leftmost
  column

#### Scenario: No first-line gutter marker inside the block

- **WHEN** a block has at least one comment and is not the
  currently-focused block
- **THEN** the block's first rendered line does NOT begin with
  a `▸` or `•` glyph; the only annotation signal in the block
  body is the yellow gutter

#### Scenario: Background tint on commented lines

- **WHEN** a block has at least one comment
- **THEN** every rendered line within that block's
  `StartLine..EndLine` range receives the comment background
  colour tint

### Requirement: In-memory storage only

Comments SHALL be stored in memory on the model and SHALL NOT be
written to disk. When the assistant message content changes (the
`latest.text` field is replaced by a new assistant message), all
existing comments SHALL be discarded. When the TUI detaches from
the session or quits, all comments SHALL be discarded.

#### Scenario: New message clears comments

- **WHEN** a new assistant message arrives and replaces the
  currently-displayed message
- **THEN** all existing comments are discarded and the rendered
  view shows no comment footnotes or highlights

#### Scenario: Detach clears comments

- **WHEN** the TUI detaches from the current session
- **THEN** all existing comments are discarded

#### Scenario: Quit clears comments

- **WHEN** the user quits the TUI
- **THEN** no comment state is preserved; the next attach starts
  with zero comments

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

### Requirement: Border and comment highlight coexist

When the currently-focused block is also a commented block, both
the cyan focus gutter and the yellow comment gutter SHALL be
applied to the block's lines. The focus signal (cyan) SHALL take
precedence over the comment signal (yellow): the gutter
character for a focused-and-commented block SHALL be `▍` cyan.
The background tint over the commented line range SHALL still
cover the block's content lines regardless of focus.

> **Note:** The prior version of this requirement described the
> heavy horizontal border around the focused block. The border
> has been replaced by the cyan left-gutter focus indicator
> defined in the `latest-message-view` spec (`### Requirement:
> Block focus indicator`). This requirement is preserved because
> the *coexistence* rule still applies (the gutter's cyan state
> and the comment tint's yellow gutter can both apply to the
> same block).

#### Scenario: Focused commented block shows cyan gutter over tint

- **WHEN** a block is both the currently-focused block and has
  comments
- **THEN** the rendered view shows the cyan `▍` gutter on every
  line in the block's range, and the background tint over those
  lines is unchanged

### Requirement: Cursor drives the gutter highlight

The cyan left-gutter focus indicator SHALL track the cursor's
currently-focused block (`cursor.blockIdx`) — the same field
that drives the viewport scroll and the selection range. As the
cursor moves between blocks (via `j`/`k`) the cyan gutter SHALL
move with it. The nav handler MUST mutate the cursor (and call
`refreshViewport`) before the gutter can reflect the new block;
otherwise the highlight would stay on whatever block
`viewport.YOffset` happened to be on.

#### Scenario: `j` moves gutter to next block

- **WHEN** the user presses `j` in `stateNav` and there is a
  next block
- **THEN** the cyan gutter moves to the next block, regardless
  of where `viewport.YOffset` sits

#### Scenario: `k` leaves gutter in place within the same block

- **WHEN** the user presses `k` in `stateNav` and the cursor
  stays within the same block (impossible at the block
  boundary, but the rule covers the case)
- **THEN** the cyan gutter stays on the same block

#### Scenario: `Esc` returns gutter to cursor-driven block

- **WHEN** the user presses `Esc` in visual mode
- **THEN** the visual flag clears and the cyan gutter continues
  to follow the cursor's `blockIdx` (which is unchanged by Esc)

### Requirement: Visual mode indicator chip

While visual line mode is active the status line SHALL display a
distinct "VISUAL" chip in addition to the normal status content,
so the user has unambiguous feedback that `v` was registered. The
chip SHALL use a high-contrast style (foreground `232`,
background `51` — cyan — bold) so it stands out from the dim
status background and is consistent with the cyan selection
colour used by the document gutter.

#### Scenario: `v` toggles indicator on

- **WHEN** the user presses `v` in `stateNav`
- **THEN** the status line shows the "VISUAL" indicator (cyan
  background, dark foreground, bold) while visual mode is active

#### Scenario: `Esc` clears indicator

- **WHEN** the user presses `Esc` in visual mode
- **THEN** the status line returns to its non-visual content (no
  "VISUAL" chip)

#### Scenario: Indicator does not appear outside visual mode

- **WHEN** the TUI is in any non-visual state (nav, compose,
  picker, error)
- **THEN** the status line SHALL NOT show the "VISUAL" chip

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

