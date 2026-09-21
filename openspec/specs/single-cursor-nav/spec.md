# single-cursor-nav Specification

## Purpose
TBD - created by archiving change vim-keymap. Update Purpose after archive.

## Requirements
### Requirement: Single cursor drives viewport and selection

The system SHALL maintain a single navigation cursor with two
fields:

- `blockIdx` — an index into `m.blocks` (the markdown block index).
- `charPos` — a byte offset into the source text of
  `m.blocks[blockIdx]`.

The cursor SHALL be the single source of truth for: which block is
"focused", what the viewport's `YOffset` is (derived from
`block.StartLine` plus the rendered line within the block that
contains `charPos`), and (when visual mode is active) the tail of
the selection range. The system SHALL NOT maintain a separate
viewport-YOffset-derived cursor or a separate visual-mode cursor.

#### Scenario: Cursor lands at the start of a block on `j`

- **WHEN** the cursor is on block `i` and the user presses `j`
- **THEN** the cursor's `blockIdx` is `i + 1` (clamped to
  `[0, len(blocks)-1]`) and the cursor's `charPos` is `0` (the
  start of the new block's source)

#### Scenario: Cursor advances one rune on `l`

- **WHEN** the cursor's `charPos` is `k` (less than
  `len(block.Source)`) and the user presses `l`
- **THEN** the cursor's `blockIdx` is unchanged and the cursor's
  `charPos` is `k + n` where `n` is the byte length of the rune at
  source offset `k`

#### Scenario: Cursor retreats one rune on `h`

- **WHEN** the cursor's `charPos` is `k` (greater than `0`) and the
  user presses `h`
- **THEN** the cursor's `blockIdx` is unchanged and the cursor's
  `charPos` is `k - n` where `n` is the byte length of the rune
  immediately preceding offset `k`

#### Scenario: Cursor clamps at byte boundary on `h`

- **WHEN** the cursor's `charPos` is `0` and the user presses `h`
- **THEN** the cursor's `charPos` remains `0` (does not wrap or
  underflow)

### Requirement: Single-letter nav surface in stateNav

In the nav state (`stateNav`, formerly `stateIdle`), the keys
listed below SHALL be bound to the listed actions. All bindings
SHALL be single-letter except `?` (help) and `Esc` (exit visual).
The previous `Ctrl+C`, `Ctrl+N`, `Ctrl+R`, `Ctrl+S`, `Ctrl+I`,
`gg`, `]]`, `[[`, `G`, `{`, `}`, `m`, `M`, `e`, `d`, `n`, `N`, and
two-key state machine bindings SHALL NOT exist in `stateNav`.

| key  | action |
|------|--------|
| `j`  | move cursor to next block |
| `k`  | move cursor to previous block |
| `h`  | move cursor one rune left within the current block |
| `l`  | move cursor one rune right within the current block |
| `v`  | enter visual mode (toggle); on re-press while in visual, exit visual |
| `Esc`| exit visual mode (no-op when not in visual) |
| `c`  | open comment composer (selection-anchored when visual is active, block-anchored otherwise) |
| `s`  | send to agent (idle batch when comments exist; compose otherwise; no-op in nav with no comments) |
| `n`  | enter compose mode |
| `r`  | refresh the tailed session and re-render the latest message |
| `q`  | quit pinky |
| `?`  | toggle short / full help overlay |

#### Scenario: `j` advances to the next block

- **WHEN** the user presses `j` in `stateNav` with a non-empty
  block list
- **THEN** the cursor's `blockIdx` increments by one (clamped at
  the last block) and the viewport scrolls to keep the cursor
  visible

#### Scenario: `k` retreats to the previous block

- **WHEN** the user presses `k` in `stateNav` with a non-empty
  block list
- **THEN** the cursor's `blockIdx` decrements by one (clamped at
  the first block) and the viewport scrolls to keep the cursor
  visible

#### Scenario: `h` retreats within the current block

- **WHEN** the cursor's `charPos` is greater than `0` and the user
  presses `h`
- **THEN** the cursor's `charPos` decreases by the byte length of
  the rune preceding it; the viewport scrolls only if the cursor
  leaves the visible range

#### Scenario: `l` advances within the current block

- **WHEN** the cursor's `charPos` is less than
  `len(block.Source)` and the user presses `l`
- **THEN** the cursor's `charPos` increases by the byte length of
  the rune at the current offset

#### Scenario: `v` enters visual

- **WHEN** the user presses `v` in `stateNav`
- **THEN** the visual flag transitions to "active" and the visual
  anchor is seeded from the current cursor position

#### Scenario: `v` toggles visual off

- **WHEN** the user is in visual mode and presses `v` again
- **THEN** the visual flag transitions to "inactive" and the
  selection range is cleared

#### Scenario: `Esc` exits visual

- **WHEN** the user is in visual mode and presses `Esc`
- **THEN** the visual flag transitions to "inactive" and the
  selection range is cleared (same effect as the second `v`)

#### Scenario: `c` over a selection opens a selection-anchored composer

- **WHEN** the user is in visual mode and presses `c`
- **THEN** the comment composer opens, focused, with anchor
  `(selection.blockIdx, charA, charC)` and visual mode transitions
  to inactive

#### Scenario: `c` without a selection opens a block-anchored composer

- **WHEN** the user is in `stateNav` (not in visual mode) and
  presses `c`
- **THEN** the comment composer opens, focused, with anchor
  `(cursor.blockIdx, 0, len(block.Source))` (whole-block range)

#### Scenario: `s` in nav with accumulated comments sends the batch

- **WHEN** the user presses `s` in `stateNav` and
  `len(m.comments) > 0`
- **THEN** the existing comments batch is formatted and sent via
  `inject.Send` to the target pane (same as the previous
  one-shot submit path); on success `m.comments` is cleared

#### Scenario: `s` in nav with no comments is a no-op

- **WHEN** the user presses `s` in `stateNav` and
  `len(m.comments) == 0`
- **THEN** the `s` is ignored and the TUI does not send anything

#### Scenario: `n` enters compose

- **WHEN** the user presses `n` in `stateNav`
- **THEN** the TUI transitions to `stateCompose` with the
  textarea reset and focused

#### Scenario: `r` triggers a refresh poll

- **WHEN** the user presses `r` in `stateNav`
- **THEN** the session source is re-polled from its current offset
  and the latest-message view is re-rendered with any newly
  surfaced content

#### Scenario: `q` quits

- **WHEN** the user presses `q` in `stateNav`
- **THEN** pinky exits (sends `tea.Quit`)

#### Scenario: `?` toggles the help overlay

- **WHEN** the user presses `?` in `stateNav`
- **THEN** the help footer transitions between short and full
  mode (same behaviour as before)

### Requirement: Viewport follows cursor with a 1-line cushion

When the cursor moves, the viewport's `YOffset` SHALL be adjusted
so that the rendered line containing the cursor's `(blockIdx,
charPos)` sits inside `[YOffset, YOffset + Height)` with a
1-line cushion (above and below) when possible. The cushion SHALL
shrink at the start and end of the rendered message so the
viewport does not underflow or overflow the total line count.

#### Scenario: Motion into off-screen territory scrolls the viewport

- **WHEN** the cursor moves (via `j`, `k`, `h`, `l`) such that its
  rendered line falls outside `[YOffset, YOffset + Height)`
- **THEN** `YOffset` is updated so the cursor's line is inside the
  visible range with the 1-line cushion preserved

#### Scenario: Motion inside the visible range leaves YOffset alone

- **WHEN** the cursor moves such that its rendered line is already
  inside `[YOffset, YOffset + Height)`
- **THEN** `YOffset` is unchanged (no per-rune jitter)

### Requirement: Visual mode extends across block boundaries

When the cursor is in visual mode and the user presses `j` (or
`k`), the selection range SHALL extend to the byte-offset end (or
start) of the destination block, crossing block boundaries when
the cursor moves across them. Anchor and cursor are tracked
independently; the selection range is `[min(anchor, cursor),
max(anchor, cursor)]` interpreted as a byte range across the
blocks touched.

#### Scenario: `v j` selects across two blocks

- **WHEN** the user presses `v` then `j` such that the cursor
  crosses a block boundary
- **THEN** the selection range covers the anchor's
  `(blockIdx, charA)` through the cursor's `(blockIdx`, end of
  source)` and the rendered highlight covers the rendered lines
  that intersect the byte range

#### Scenario: `v k` shrinks / clears a multi-block selection

- **WHEN** the user has a multi-block selection and presses `k`
  such that the cursor retreats below the anchor
- **THEN** the selection range is recomputed as `[cursor, anchor]`
  and the highlight visually reflects the new range

### Requirement: Universal send across nav and compose

The key `s` SHALL be the single "send to agent" key in `stateNav`
and `stateCompose`. The dispatch SHALL branch on state:

- `stateNav` → batch-send via the existing `submitAllComments`
  function (no-op when `len(m.comments) == 0`).
- `stateCompose` → send the textarea contents via `inject.Send`,
  appending the comments appendix when `Ctrl+I` (the include
  toggle) is on.

The `Ctrl+S` binding SHALL NOT exist in `stateNav` or
`stateCompose`. The `Ctrl+S` binding SHALL remain ONLY in
`stateCommentComposer` to save multi-line comment text.

#### Scenario: `s` from compose sends the textarea contents

- **WHEN** the user is in `stateCompose` with non-empty textarea
  and presses `s`
- **THEN** the textarea contents are sent via `inject.Send`,
  optionally followed by the comments appendix if
  `m.includeComments` is true

#### Scenario: `s` from compose with empty textarea is a no-op

- **WHEN** the user is in `stateCompose` with empty textarea and
  presses `s`
- **THEN** nothing is sent and the TUI remains in `stateCompose`

#### Scenario: `Ctrl+S` does not send in compose

- **WHEN** the user is in `stateCompose` and presses `Ctrl+S`
- **THEN** the keypress is sent to the textarea as a literal
  character (the model does not intercept it)

#### Scenario: `Ctrl+S` saves in comment composer

- **WHEN** the user is in `stateCommentComposer` and presses
  `Ctrl+S`
- **THEN** the comment is saved with the current anchor and the
  TUI returns to `stateNav`
