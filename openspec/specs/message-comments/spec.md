# message-comments Specification

## Purpose
TBD - created by archiving change add-block-and-inline-comments. Update Purpose after archive.

## Requirements

### Requirement: Visual block-level annotation

The system SHALL allow the user to attach a free-text comment to the
currently-focused block in the idle view. Pressing `m` while a block is
focused SHALL open a comment composer pre-anchored to that block.
Saving the composer (Ctrl+S) SHALL store the comment with anchor
`(BlockIdx, -1, -1)` (block-level only, no inline byte offsets) and
return to the idle view. Cancelling (Esc) SHALL discard the composer
text and return to the idle view without creating a comment.

#### Scenario: Mark current block opens composer

- **WHEN** the user presses `m` in idle state and a block is focused
- **THEN** the TUI enters comment composer state with the composer
  textarea focused and empty

#### Scenario: Save creates a block-level comment

- **WHEN** the user is in comment composer state with text in the
  textarea and presses Ctrl+S
- **THEN** a comment is stored with anchor `(BlockIdx, -1, -1)` where
  `BlockIdx` is the index of the currently-focused block, the TUI
  returns to idle state, and the block becomes flagged as commented

#### Scenario: Cancel discards composer

- **WHEN** the user is in comment composer state and presses Esc
- **THEN** no comment is stored, the TUI returns to idle state, and
  the previously-focused block is unchanged

#### Scenario: Mark with no focused block is a no-op

- **WHEN** the user presses `m` in idle state and no block is
  currently focused (empty state or YOffset outside any block)
- **THEN** the TUI remains in idle state and no composer is opened

### Requirement: Visual inline line selection

The system SHALL provide a vim-style visual line mode for selecting a
contiguous range of rendered lines within a single block. Pressing
`V` in idle state SHALL enter visual line mode and park both the
anchor and cursor at the rendered line containing the viewport's
top YOffset. While in visual mode, `j` SHALL extend the cursor down
by one rendered line, `k` SHALL extend the cursor up by one rendered
line, `}` SHALL extend the cursor down to the first line of the next
block, and `{` SHALL extend the cursor up to the first line of the
previous block. Pressing `Esc` SHALL exit visual mode without saving.
Pressing `c` SHALL open the comment composer pre-anchored to the
selected line range.

#### Scenario: V enters visual mode at current line

- **WHEN** the user presses `V` in idle state
- **THEN** the TUI enters visual line mode with both anchor and
  cursor set to the rendered line containing the viewport's current
  YOffset

#### Scenario: j extends visual cursor down

- **WHEN** the user is in visual line mode and presses `j`
- **THEN** the visual cursor moves down by one rendered line and the
  selected range is highlighted accordingly

#### Scenario: k extends visual cursor up

- **WHEN** the user is in visual line mode and presses `k`
- **THEN** the visual cursor moves up by one rendered line and the
  selected range is highlighted accordingly

#### Scenario: } extends visual cursor by block

- **WHEN** the user is in visual line mode and presses `}` and there
  is a next block
- **THEN** the visual cursor moves to the first rendered line of the
  next block

#### Scenario: Esc exits visual mode without saving

- **WHEN** the user is in visual line mode and presses `Esc`
- **THEN** the TUI returns to idle state, the visual cursor and
  anchor are cleared, and no comment is stored

#### Scenario: c opens composer for selection

- **WHEN** the user is in visual line mode and presses `c`
- **THEN** the TUI enters comment composer state with the composer
  textarea focused and empty, and the saved anchor is the line
  range currently selected

### Requirement: Inline selection anchors by byte offset

When the comment composer is opened from visual line mode, the saved
anchor SHALL be `(BlockIdx, CharStart, CharEnd)` where `BlockIdx` is
the index of the block containing the selection, `CharStart` is the
byte offset into the original assistant message text corresponding to
the anchor line, and `CharEnd` is the byte offset into the original
assistant message text corresponding to the cursor line. The byte
offsets SHALL be derived from the goldmark AST `Lines()` segments of
the relevant block node.

#### Scenario: Inline anchor uses byte offsets

- **WHEN** the user opens the comment composer from visual line mode
  and saves
- **THEN** the saved comment has `Kind = BlockParagraph` (or whatever
  the containing block's kind is), `BlockIdx` matches the containing
  block, and `CharStart` and `CharEnd` are non-negative byte offsets
  into the original assistant message text

#### Scenario: Inline anchor preserves source text

- **WHEN** a comment is saved with an inline anchor
- **THEN** the verbatim slice `originalText[CharStart:CharEnd]` is
  stored alongside the comment so the original text can be quoted in
  the redirect appendix

### Requirement: Auto-scroll during visual selection

While in visual line mode, when the visual cursor moves to a rendered
line that is not currently visible in the viewport (above the top or
below the bottom of the visible range), the viewport SHALL scroll so
that the cursor line becomes visible.

#### Scenario: j past bottom scrolls down

- **WHEN** the user is in visual line mode with the cursor at the
  last visible line of the viewport and presses `j`
- **THEN** the viewport scrolls down by one rendered line so the
  new cursor line is visible

#### Scenario: k past top scrolls up

- **WHEN** the user is in visual line mode with the cursor at the
  first visible line of the viewport and presses `k`
- **THEN** the viewport scrolls up by one rendered line so the new
  cursor line is visible

### Requirement: Comment footnote rendering

The system SHALL render saved comments as footnote lines
immediately following the block they annotate. Each comment
footnote SHALL be prefixed with a comment marker (`▸` for
block-level comments, `•` for inline comments) and SHALL display
the comment text. The rendered line range of the commented block
SHALL also receive a background colour tint to make the commented
region visually distinct. The commented block SHALL be marked in
the left-margin gutter column with the character `▍` in yellow
(foreground colour `228`) on every rendered line that falls inside
the block's `StartLine..EndLine` range, regardless of whether the
block is currently focused. No first-line `▸` or `•` glyph SHALL
be prepended to the block's first rendered line in the document
body.

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

### Requirement: Block-local edit and delete

When the currently-focused block has one or more saved comments, the
system SHALL expose edit and delete operations scoped to that block.
Pressing `e` in idle state while focused on a commented block SHALL
open the comment composer pre-filled with the most recent comment's
text. Pressing `d` in idle state while focused on a commented block
SHALL delete the most recent comment. Pressing `n` SHALL jump the
viewport to the first line of the next commented block (or the first
commented block if none has been visited yet). Pressing `N` SHALL
jump to the first line of the previous commented block.

#### Scenario: e edits most recent comment on current block

- **WHEN** the user presses `e` in idle state and the focused block
  has at least one comment
- **THEN** the TUI enters comment composer state with the textarea
  pre-filled with the text of the most recent comment on that block

#### Scenario: d deletes most recent comment on current block

- **WHEN** the user presses `d` in idle state and the focused block
  has at least one comment
- **THEN** the most recent comment on that block is removed, the
  view re-renders without that comment's footnote and highlight, and
  the TUI remains in idle state

#### Scenario: e on uncommented block is a no-op

- **WHEN** the user presses `e` in idle state and the focused block
  has no comments
- **THEN** the TUI remains in idle state and no composer is opened

#### Scenario: d on uncommented block is a no-op

- **WHEN** the user presses `d` in idle state and the focused block
  has no comments
- **THEN** the TUI remains in idle state and nothing is deleted

#### Scenario: n jumps to next commented block

- **WHEN** the user presses `n` in idle state and there is a
  commented block after the focused block
- **THEN** the viewport scrolls so the first line of the next
  commented block is at the top of the viewport

#### Scenario: N jumps to previous commented block

- **WHEN** the user presses `N` in idle state and there is a
  commented block before the focused block
- **THEN** the viewport scrolls so the first line of the previous
  commented block is at the top of the viewport

### Requirement: In-memory storage only

Comments SHALL be stored in memory on the model and SHALL NOT be
written to disk. When the assistant message content changes (the
`latest.text` field is replaced by a new assistant message), all
existing comments SHALL be discarded. When the TUI detaches from the
session or quits, all comments SHALL be discarded.

#### Scenario: New message clears comments

- **WHEN** a new assistant message arrives and replaces the
  currently-displayed message
- **THEN** all existing comments are discarded and the rendered view
  shows no comment footnotes or highlights

#### Scenario: Detach clears comments

- **WHEN** the TUI detaches from the current session
- **THEN** all existing comments are discarded

#### Scenario: Quit clears comments

- **WHEN** the user quits the TUI
- **THEN** no comment state is preserved; the next attach starts
  with zero comments

### Requirement: Include comments in redirect

While in compose state, pressing `Ctrl+I` SHALL toggle whether the
queued redirect will be sent with a comments appendix. When the flag
is enabled and the user sends the redirect (Ctrl+S), the inject
payload SHALL be the redirect text followed by a separator line
(`---`) and a numbered list of all current comments in chronological
order (oldest first). Each entry SHALL include the comment kind
(`block` or `inline`), a short quote of the source text (block source
or inline excerpt, truncated to ~40 chars with an ellipsis if longer),
the block's line range, and the comment text.

#### Scenario: Ctrl+I toggles include flag

- **WHEN** the user is in compose state and presses `Ctrl+I`
- **THEN** the include-comments flag is toggled, and the status line
  reflects the new state (e.g., `[I] include N comments — ON/OFF`)

#### Scenario: Send with flag ON appends appendix

- **WHEN** the user is in compose state, the include flag is ON, and
  the user presses `Ctrl+S` with non-empty text
- **THEN** the inject payload sent to the agent's pane is
  `<redirect text>\n\n---\nN comments:\n- <comment 1>\n- <comment 2>\n...`
  where each comment line follows the format
  `block "<excerpt>" (lines X-Y): <text>` or
  `inline "<excerpt>" (line Z): <text>`

#### Scenario: Send with flag OFF sends plain redirect

- **WHEN** the user is in compose state, the include flag is OFF,
  and the user presses `Ctrl+S` with non-empty text
- **THEN** the inject payload is exactly the redirect text with no
  comments appendix

#### Scenario: Send with flag ON and zero comments sends plain redirect

- **WHEN** the user is in compose state, the include flag is ON, and
  there are no comments, and the user presses `Ctrl+S`
- **THEN** the inject payload is exactly the redirect text (no
  appendix because the count is zero)

### Requirement: Border and comment highlight coexist

When the currently-focused block is also a commented block, both
the cyan focus gutter and the yellow comment gutter SHALL be
applied to the block's lines. The focus signal (cyan) SHALL take
precedence over the comment signal (yellow): the gutter character
for a focused-and-commented block SHALL be `▍` cyan. The
background tint over the commented line range SHALL still cover
the block's content lines regardless of focus.

> **Note:** The prior version of this requirement described the
> heavy horizontal border around the focused block. The border
> has been replaced by the cyan left-gutter focus indicator
> defined in the `latest-message-view` spec
> (`### Requirement: Block focus indicator`). This requirement
> is preserved because the *coexistence* rule still applies
> (the gutter's cyan state and the comment tint's yellow gutter
> can both apply to the same block).

#### Scenario: Focused commented block shows cyan gutter over tint

- **WHEN** a block is both the currently-focused block and has
  comments
- **THEN** the rendered view shows the cyan `▍` gutter on every
  line in the block's range, and the background tint over those
  lines is unchanged

### Requirement: Visual cursor drives the gutter highlight

While in visual line mode, the cyan left-gutter focus indicator
SHALL track the visual cursor's currently-focused block, not the
viewport-driven `YOffset`. As the visual cursor moves between
blocks (via `j`/`k`/`}`/`{`) the cyan gutter SHALL move with it.
The visual mode handler MUST short-circuit `j`/`k`/`}`/`{`
before the idle-view line-movement bindings can match them;
otherwise the visual cursor would never advance and the gutter
would stay on whatever block `viewport.YOffset` happened to be on.

#### Scenario: } moves gutter to next block
- **WHEN** the user presses `}` in visual line mode and there is a
  next block
- **THEN** the cyan gutter moves to the next block, regardless of
  where `viewport.YOffset` sits

#### Scenario: j leaves gutter in place within the same block
- **WHEN** the user presses `j` in visual line mode and the
  visual cursor stays within the same block
- **THEN** the cyan gutter stays on the same block (the cursor
  moved within it)

#### Scenario: Esc returns gutter to viewport-driven block
- **WHEN** the user presses `Esc` in visual line mode
- **THEN** the cyan gutter returns to the block containing
  `viewport.YOffset` (the normal idle-view behavior)

### Requirement: Visual mode indicator chip

While visual line mode is active the status line SHALL display a
distinct "VISUAL" chip in addition to the normal status content,
so the user has unambiguous feedback that `V` was registered. The
chip SHALL use a high-contrast style (foreground `232`, background
`51` — cyan — bold) so it stands out from the dim status
background and is consistent with the cyan selection colour used
by the document gutter.

#### Scenario: V toggles indicator on
- **WHEN** the user presses `V` in idle state
- **THEN** the status line shows the "VISUAL" indicator (cyan
  background, dark foreground, bold) while visual mode is active

#### Scenario: Esc clears indicator
- **WHEN** the user presses `Esc` in visual line mode
- **THEN** the status line returns to its non-visual content (no
  "VISUAL" chip)

#### Scenario: Indicator does not appear outside visual mode
- **WHEN** the TUI is in any non-visual state (idle, compose,
  picker, error)
- **THEN** the status line SHALL NOT show the "VISUAL" chip

### Requirement: One-shot submit all comments

Pressing `s` in idle state SHALL submit every accumulated comment
as a single redirect through the existing inject pipeline, without
requiring the user to enter compose mode or type any text. The
redirect payload SHALL be exactly the comments appendix (no leading
user text). On successful send the comment slice SHALL be cleared;
on inject failure the comments SHALL be kept so the user can retry
and the error SHALL be surfaced via the same `[send failed: ...]`
placeholder the compose path uses.

#### Scenario: s submits all accumulated comments
- **WHEN** the user presses `s` in idle state and there is at
  least one comment
- **THEN** the inject pipeline receives a payload equal to
  `render.FormatCommentsAppendix(comments, blocks)`, the comment
  slice is cleared, and no compose-mode interaction is required

#### Scenario: s with no comments is a no-op
- **WHEN** the user presses `s` in idle state and the comment
  slice is empty
- **THEN** no inject call is made and no state changes

#### Scenario: send failure keeps comments
- **WHEN** the user presses `s` and the inject call returns an
  error
- **THEN** the comment slice is NOT cleared and the error is
  surfaced via the same `[send failed: ...]` placeholder the
  compose path uses
