## MODIFIED Requirements

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
| `w`  | move cursor to the start of the next word (crosses `\n`; blank lines are separators); `preferred = max(preferred, charPos)` |
| `b`  | move cursor to the start of the previous word (crosses `\n`; blank lines are separators); `preferred` unchanged |
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

#### Scenario: `w` advances to the next word within the current line
- **WHEN** the cursor is on a source line, the byte at
  `charPos` is inside a word or between two words, and the user
  presses `w`
- **THEN** `charPos` becomes the byte offset of the first rune
  of the next word on the current line (the next run of
  `[A-Za-z0-9_]`, or the next punctuation run if no such word
  exists on the current line); `preferred` becomes
  `max(preferred, charPos)`; `lineIdx` is unchanged unless the
  next word is on a later line (see "Word motion in stateNav")

#### Scenario: `b` retreats to the previous word within the current line
- **WHEN** the cursor is on a source line, `charPos > 0`, and the
  user presses `b`
- **THEN** `charPos` becomes the byte offset of the first rune
  of the previous word on the current line; `preferred` is
  unchanged; `lineIdx` is unchanged unless the previous word is
  on an earlier line (see "Word motion in stateNav")

## ADDED Requirements

### Requirement: Word motion in stateNav

The keys `w` and `b` SHALL move the cursor to the start of the
next / previous word respectively. A word SHALL be defined as a
maximal run of runes in `[A-Za-z0-9_]` (vim's default
`iskeyword`); a punctuation run (one or more non-word,
non-whitespace runes) SHALL also be a word on its own. Whitespace
and punctuation runs both act as word separators.

Word motion SHALL cross `\n` boundaries freely. When the cursor
sits on the last word of a source line, `w` SHALL advance to the
first word of the next non-blank source line; blank lines SHALL
be treated as separators (skipped, the same way ` ` / `\t` runs
are). When the cursor sits on the first word of a source line,
`b` SHALL retreat to the first word of the previous non-blank
source line by the same rule. When no next / previous word
exists in the source, the cursor SHALL remain at its current
position (no wrap-around).

The `preferred` column rule SHALL mirror `l` / `h`: `w` sets
`preferred = max(preferred, charPos)`; `b` leaves `preferred`
unchanged. Subsequent `j` / `k` SHALL still use the rightmost
column reached so far.

In visual mode, `w` and `b` SHALL extend the selection's
`byteC` to the new cursor byte (`byteA` is preserved, set by
the `v` that entered visual mode). Visual mode and `preferred`
interact exactly as for `l` / `h` — see "Visual mode extends
across block boundaries".

#### Scenario: `w` advances within a punctuation-rich line
- **WHEN** the cursor is at `charPos` inside the word `foo` of
  the source line `"foo.bar.baz"` and the user presses `w`
- **THEN** `charPos` becomes the byte offset of the first rune
  of the next word (`.`); `preferred` becomes
  `max(preferred, charPos)`; on the next `w`, `charPos` becomes
  the byte offset of `bar`; on the third `w`, `charPos` becomes
  the byte offset of `baz`

#### Scenario: `w` crosses a `\n` boundary into the next line
- **WHEN** the cursor is on the last byte of a non-blank source
  line, that byte is the last rune of a word, and the user
  presses `w`
- **THEN** `lineIdx` advances to the next source line whose
  contents are not all whitespace, and `charPos` becomes the
  byte offset of the first rune of the first word on that line;
  `preferred` becomes `max(preferred, charPos)`

#### Scenario: `w` skips blank lines as separators
- **WHEN** the cursor is on a source line and the lines between
  the cursor's line and the next non-blank source line are all
  empty or whitespace-only, and the user presses `w`
- **THEN** the cursor lands on the first word of the next
  non-blank source line, skipping every blank line in between

#### Scenario: `b` on the first word of a non-blank line jumps to the previous non-blank line
- **WHEN** the cursor is at `charPos == 0` of a non-blank source
  line and that byte is the first rune of a word, and the user
  presses `b`
- **THEN** `lineIdx` retreats to the previous non-blank source
  line, `charPos` becomes the byte offset of the first rune of
  its first word; `preferred` is unchanged

#### Scenario: `w` / `b` are no-ops at the source boundaries
- **WHEN** the cursor is on the last word of the last source
  line and the user presses `w`, OR the cursor is on the first
  word of the first source line and the user presses `b`
- **THEN** the cursor's `(lineIdx, charPos, preferred)` is
  unchanged and `sel.ByteC` is unchanged when visual is active

#### Scenario: `vw` extends a visual selection across words
- **WHEN** the user is in visual mode with `byteA` set by `v`,
  the cursor is in a word, and the user presses `w`
- **THEN** the cursor's `(lineIdx, charPos)` moves to the next
  word's start as in `w` outside visual, `sel.ByteA` is
  unchanged, `sel.ByteC` is updated to the byte offset of the
  new cursor position, and the rendered highlight covers every
  rendered line that intersects `[min(byteA, byteC),
  max(byteA, byteC)]`

#### Scenario: `vb` shrinks a visual selection toward the anchor
- **WHEN** the user is in visual mode with a non-empty
  selection and presses `b` such that the cursor retreats past its
  current position but not past the anchor
- **THEN** `sel.ByteC` is updated to the byte offset of the new
  cursor position, `sel.ByteA` is unchanged, and the rendered
  highlight shrinks to cover every rendered line that
  intersects the new range; if the new `byteC` equals `byteA`,
  the selection becomes empty (visual still active)