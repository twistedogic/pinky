# Spec Delta

## MODIFIED Requirements

### Requirement: Single cursor drives viewport and selection

The system SHALL maintain a single navigation cursor with three
fields:

- `lineIdx` — a 1-based index into the message's source lines.
- `charPos` — a byte offset into the source text of
  `m.lines[lineIdx]`, clamped to `[0, len(currentLine)]` after
  every cursor move, and snapped to the nearest rune boundary
  on every motion (`h` / `l` / `j` / `k`) — no cursor position
  SHALL fall in the middle of a multi-byte UTF-8 rune.
- `preferred` — the last intended column, updated only by `h` /
  `l` (set to `max(preferred, charPos)` on every rune move),
  held unchanged by `j` / `k`, and reset to `0` whenever a new
  message replaces `m.latest`.

The cursor SHALL be the single source of truth for: which
source line the cursor sits on, what the viewport's `YOffset` is
(derived from the rendered-line projection of `(lineIdx,
charPos)`), and (when visual mode is active) the tail of the
selection range. The system SHALL NOT maintain a separate
viewport-YOffset-derived cursor or a separate visual-mode
cursor.

#### Scenario: Cursor lands at the start of a block on `j`
- **WHEN** the cursor is on line `L` and the user presses `j`
- **THEN** the cursor's `lineIdx` is `L + 1` (clamped to
  `[0, len(lines)-1]`) and the cursor's `charPos` is
  `min(preferred, len(newLine))`; `preferred` is unchanged

#### Scenario: Cursor advances one rune on `l`
- **WHEN** the cursor's `charPos` is `k` (less than
  `len(currentLine)`) and the user presses `l`
- **THEN** the cursor's `lineIdx` is unchanged, the cursor's
  `charPos` is `k + n` where `n` is the byte length of the rune
  at source offset `k`, and `preferred` becomes
  `max(preferred, charPos)`

#### Scenario: Cursor retreats one rune on `h`
- **WHEN** the cursor's `charPos` is `k` (greater than `0`) and
  the user presses `h`
- **THEN** the cursor's `lineIdx` is unchanged, the cursor's
  `charPos` is `k - n` where `n` is the byte length of the rune
  immediately preceding offset `k`, and `preferred` is unchanged

#### Scenario: Cursor clamps at byte boundary on `h`
- **WHEN** the cursor's `charPos` is `0` and the user presses `h`
- **THEN** the cursor's `charPos` remains `0` (does not wrap or
  underflow); `preferred` is unchanged

### Requirement: Single-letter nav surface in stateNav

In the nav state (`stateNav`, formerly `stateIdle`), the keys
listed below SHALL be bound to the listed actions. All bindings
SHALL be single-letter or single-symbol except `?` (help),
`Esc` (exit visual), and the named scroll keys (`PageUp`,
`PageDown`, `Home`, `End`). The previous `Ctrl+C`, `Ctrl+N`,
`Ctrl+R`, `Ctrl+S`, `Ctrl+I`, `gg`, `]]`, `[[`, `G`, `{`, `}`,
`m`, `M`, `e`, `d`, `n`, `N`, and two-key state machine bindings
SHALL NOT exist in `stateNav`.

| key  | action |
|------|--------|
| `j`  | move cursor to next source line; `charPos = min(preferred, len(newLine))`; `preferred` unchanged |
| `k`  | move cursor to previous source line; same `charPos` rule as `j` |
| `h`  | move cursor one rune left within the current source line; `preferred` unchanged |
| `l`  | move cursor one rune right within the current source line; `preferred = max(preferred, charPos)` |
| `v`  | enter visual mode (toggle); on re-press while in visual, exit visual |
| `Esc`| exit visual mode (no-op when not in visual) |
| `c`  | open comment composer (selection-anchored when visual is active with a non-empty range, whole-current-line-anchored otherwise) |
| `s`  | send to agent (idle batch when comments exist; compose otherwise; no-op in nav with no comments) |
| `n`  | enter compose mode |
| `r`  | refresh the tailed session and re-render the latest message |
| `q`  | quit pinky |
| `?`  | toggle short / full help overlay |
| `↓`  | scroll viewport one line down (cursor unchanged) |
| `↑`  | scroll viewport one line up (cursor unchanged) |
| `PageDown` | scroll viewport one page down (cursor unchanged) |
| `PageUp`   | scroll viewport one page up (cursor unchanged) |
| `Home` | scroll viewport to top (cursor unchanged) |
| `End`  | scroll viewport to bottom (cursor unchanged) |

#### Scenario: `j` advances to the next block
- **WHEN** the user presses `j` in `stateNav` with a non-empty
  source-line list
- **THEN** the cursor's `lineIdx` increments by one (clamped at
  the last line), `charPos` becomes `min(preferred, len(newLine))`,
  `preferred` is unchanged, and the viewport scrolls to keep the
  cursor visible

#### Scenario: `k` retreats to the previous block
- **WHEN** the user presses `k` in `stateNav` with a non-empty
  source-line list
- **THEN** the cursor's `lineIdx` decrements by one (clamped at
  `0`), `charPos` becomes `min(preferred, len(newLine))`,
  `preferred` is unchanged, and the viewport scrolls to keep the
  cursor visible

#### Scenario: `h` retreats within the current block
- **WHEN** the cursor's `charPos` is greater than `0` and the
  user presses `h`
- **THEN** the cursor's `charPos` decreases by the byte length of
  the rune preceding it; `preferred` is unchanged; the viewport
  scrolls only if the cursor leaves the visible range

#### Scenario: `l` advances within the current block
- **WHEN** the cursor's `charPos` is less than `len(currentLine)`
  and the user presses `l`
- **THEN** the cursor's `charPos` increases by the byte length of
  the rune at the current offset and `preferred` becomes
  `max(preferred, charPos)`

#### Scenario: `v` enters visual
- **WHEN** the user presses `v` in `stateNav`
- **THEN** the visual flag transitions to "active" and the visual
  anchor is seeded from the current cursor position
  `(lineIdx, charPos)`

#### Scenario: `v` toggles visual off
- **WHEN** the user is in visual mode and presses `v` again
- **THEN** the visual flag transitions to "inactive" and the
  selection range is cleared

#### Scenario: `Esc` exits visual
- **WHEN** the user is in visual mode and presses `Esc`
- **THEN** the visual flag transitions to "inactive" and the
  selection range is cleared (same effect as the second `v`)

#### Scenario: `c` over a selection opens a selection-anchored composer
- **WHEN** the user is in visual mode with a non-empty selection
  range and presses `c`
- **THEN** the comment composer opens, focused, with anchor
  `(byteA, byteC)` covering the selection range and visual mode
  transitions to inactive

#### Scenario: `c` without a selection opens a block-anchored composer
- **WHEN** the user is in `stateNav` (not in visual mode, or in
  visual mode with an empty range) and presses `c`
- **THEN** the comment composer opens, focused, with anchor
  `(byteA, byteC)` covering the whole current source line
  (every byte from the line's start to its end, inclusive of any
  trailing newline)

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

#### Scenario: scroll keys move viewport only
- **WHEN** the user presses `↓`, `↑`, `PageDown`, `PageUp`,
  `Home`, or `End` in `stateNav`
- **THEN** the viewport's `YOffset` is updated and the cursor's
  `(lineIdx, charPos, preferred)` is unchanged

### Requirement: Viewport follows cursor with a 1-line cushion

When the cursor moves, the viewport's `YOffset` SHALL be adjusted
so that the rendered line containing the cursor's `(lineIdx,
charPos)` sits inside `[YOffset, YOffset + Height)` with a
1-line cushion (above and below) when possible. The cushion SHALL
shrink at the start and end of the rendered message so the
viewport does not underflow or overflow the total line count.

#### Scenario: Motion into off-screen territory scrolls the viewport
- **WHEN** the cursor moves (via `j`, `k`, `h`, `l`) such that
  its rendered line falls outside `[YOffset, YOffset + Height)`
- **THEN** `YOffset` is updated so the cursor's line is inside
  the visible range with the 1-line cushion preserved

#### Scenario: Motion inside the visible range leaves YOffset alone
- **WHEN** the cursor moves such that its rendered line is
  already inside `[YOffset, YOffset + Height)`
- **THEN** `YOffset` is unchanged (no per-rune jitter)

#### Scenario: Scroll keys do not retrift viewport-follows-cursor
- **WHEN** the user presses a scroll key (`↓`, `↑`, `PageDown`,
  `PageUp`, `Home`, `End`) and the resulting `YOffset` causes
  the cursor's rendered line to fall outside the visible range
- **THEN** `YOffset` is NOT corrected to bring the cursor back
  into view (the user is in control of the viewport while
  scroll-keying)

### Requirement: Visual mode extends across block boundaries

When the cursor is in visual mode and the user presses `j` (or
`k`), the selection range SHALL extend to the byte-offset on
the destination source line, crossing line boundaries when the
cursor moves across them. Anchor and cursor are tracked
independently; the selection range is a global byte range
`(byteA, byteC)` over `m.latest.Text`, recomputed on every
cursor move as the byte range from the anchor's `(lineIdx,
charPos)` to the cursor's `(lineIdx, charPos)`. `j` / `k` in
visual SHALL apply the preferred-column rule: `charPos` becomes
`min(preferred, len(newLine))`, `preferred` is unchanged.

#### Scenario: `v j` selects across two blocks
- **WHEN** the user presses `v` then `j` such that the cursor's
  `lineIdx` advances by one
- **THEN** the selection range's `byteC` becomes the byte offset
  of the cursor's new `(lineIdx, charPos)` and the rendered
  highlight covers every source line and rendered line that
  intersects the byte range

#### Scenario: `v k` shrinks / clears a multi-block selection
- **WHEN** the user has a multi-line selection and presses `k`
  such that the cursor retreats below the anchor
- **THEN** the selection range is recomputed as the byte range
  from the cursor's `(lineIdx, charPos)` to the anchor's
  `(lineIdx, charPos)` (whichever is lower byte-wise is `byteA`),
  and the highlight visually reflects the new range

#### Scenario: `v h` and `v l` adjust selection within the line
- **WHEN** the user is in visual mode and presses `h` or `l`
- **THEN** the selection range's `byteA` or `byteC` is adjusted
  by the byte length of the rune the cursor traverses; `h` does
  not change `preferred`, `l` sets `preferred = max(preferred,
  charPos)`

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

## ADDED Requirements

### Requirement: Scroll keys move viewport only

The keys `↑`, `↓`, `PageUp`, `PageDown`, `Home`, and `End` in
`stateNav` SHALL move the viewport's `YOffset` only. The cursor's
`(lineIdx, charPos, preferred)` SHALL NOT change when a scroll
key is pressed, regardless of whether the cursor's rendered
line remains visible. The viewport-follows-cursor behaviour
(see Viewport follows cursor with a 1-line cushion) is suspended
while the user is scroll-keying; `j` / `k` re-engage the
viewport-follows-cursor behaviour on the next motion key.

#### Scenario: scroll keys do not move the cursor
- **WHEN** the user presses any scroll key in `stateNav`
- **THEN** `YOffset` updates per the key's action (one line,
  one page, top, bottom) and the cursor's `(lineIdx, charPos,
  preferred)` is unchanged

#### Scenario: scroll key away from cursor leaves cursor off-screen
- **WHEN** the user presses `Home` (or any scroll key whose
  effect is to leave the cursor's rendered line outside
  `[YOffset, YOffset + Height)`)
- **THEN** the cursor's `(lineIdx, charPos, preferred)` is
  unchanged and the viewport does NOT auto-correct to bring the
  cursor back into view

#### Scenario: j / k after scroll key re-engage viewport-follows-cursor
- **WHEN** the user has scrolled the viewport away from the
  cursor and then presses `j` or `k`
- **THEN** the cursor moves by one source line and the viewport
  scrolls to keep the cursor visible with the 1-line cushion
  (viewport-follows-cursor is restored)