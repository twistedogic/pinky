## ADDED Requirements

### Requirement: Fuzzy search via `/` in stateNav

In `stateNav` the system SHALL provide a fuzzy search affordance
for the rendered message text. The key surface and behaviour is:

| key     | action |
|---------|--------|
| `/`     | open a one-line prompt at the bottom of the view; the query starts empty and the prompt shows `/` followed by a half-block cursor |
| `<rune>` | append the rune to the query; the visible highlight updates as the query changes; the prompt cursor advances by one cell |
| `Backspace` | drop the last rune from the query; the visible highlight updates |
| `Enter` | close the prompt, move the nav cursor to the byte position of the first match (in document order) at or after the cursor's current `(lineIdx, charPos)`, and set the current-match index to that match |
| `Esc`   | close the prompt, clear the query, clear every highlight, leave the cursor at its current position |
| `n`     | when a search is active OR `m.navSearch.hits` is non-empty: advance the current-match index to the next hit in document order (wrapping from the last hit to the first), move the cursor to that hit's `(lineIdx, charPos)`, and re-render. When no search is active: enter compose mode (the existing behaviour) |
| `N`     | when a search is active OR `m.navSearch.hits` is non-empty: retreat the current-match index to the previous hit in document order (wrapping from the first hit to the last), move the cursor to that hit's `(lineIdx, charPos)`, and re-render. When no search is active: no-op (no existing binding collides) |

While the prompt is open, every other printable rune (including
`j`/`k`/`h`/`l`/`w`/`b`/`c`/`s`/`v`/`r`/`q`) is consumed by the
query; the existing nav bindings are inactive. Arrow keys
(`↑`/`↓`/`PageDown`/`PageUp`/`Home`/`End`) are NOT consumed by the
prompt — they continue to scroll the viewport, matching the
file-navigator `/` behaviour.

The match algorithm SHALL be the same subsequence matcher the
file navigator's `/` uses: every rune of the (case-folded) query
appears in the (case-folded) source line in order. Every
non-overlapping left-to-right match on each line SHALL be
returned as a `(lineIdx, byteA, byteC)` triple, with the full
list ordered by `(lineIdx, byteA)`.

The match algorithm SHALL operate on the raw source lines
(`m.lines`); it SHALL NOT operate on the rendered ANSI string or
on any glamour-transformed text. Markdown markers in the source
(`#`, `*`, backticks, `|` etc.) SHALL be treated as ordinary
characters for matching purposes.

An empty query SHALL produce no hits and no highlight; a query
with no matching characters SHALL produce no hits and no
highlight.

#### Scenario: `/` opens a prompt with empty query
- **WHEN** the user presses `/` in `stateNav`
- **THEN** `m.navSearch.active` becomes `true`, the prompt is
  rendered at the bottom of the view as `/` followed by a
  half-block cursor, and no highlight is drawn

#### Scenario: typing a query updates the highlight
- **WHEN** the user types `t`, `h`, `e` while the prompt is
  open and the source contains `the` somewhere
- **THEN** every non-overlapping `the` match on every line is
  highlighted with the dim background colour; the prompt
  shows `/the`; the cursor is unchanged

#### Scenario: Backspace trims the query
- **WHEN** the user types `the` and then `Backspace`
- **THEN** the query becomes `th`, the highlight re-renders to
  show only `th` matches, and the prompt shows `/th`

#### Scenario: `Enter` jumps cursor to the first match
- **WHEN** the user types `the` and presses `Enter` and the
  cursor is on line 0 before the first match
- **THEN** the prompt closes, the cursor's `(lineIdx, charPos)`
  is the byte position of the first match, the viewport
  scrolls to keep the cursor visible, and the current-match
  index points at the first match (which is now visible as the
  cursor's inverted-block cell)

#### Scenario: `Enter` with no matches leaves the cursor where it is
- **WHEN** the user types `zzz` and presses `Enter` and no
  `zzz` match exists in the source
- **THEN** the prompt closes, no highlight remains, the cursor
  is unchanged, and the current-match index is `-1`

#### Scenario: `Esc` clears query and highlights
- **WHEN** the user types `the` and presses `Esc`
- **THEN** the prompt closes, the query is empty, no highlight
  is drawn, and the cursor is unchanged

#### Scenario: `n` cycles to the next match
- **WHEN** the current match is the second of three and the
  user presses `n`
- **THEN** the cursor moves to the third match's byte
  position, the viewport scrolls to keep it visible, and the
  current-match index advances by one

#### Scenario: `n` wraps from last to first
- **WHEN** the current match is the third (last) of three and
  the user presses `n`
- **THEN** the cursor moves to the first match's byte position
  and the current-match index becomes `0`

#### Scenario: `N` cycles to the previous match
- **WHEN** the current match is the second of three and the
  user presses `N`
- **THEN** the cursor moves to the first match's byte
  position and the current-match index retreats by one

#### Scenario: `N` wraps from first to last
- **WHEN** the current match is the first of three and the
  user presses `N`
- **THEN** the cursor moves to the third match's byte position
  and the current-match index becomes `2`

#### Scenario: `n` falls through to compose when no search is active
- **WHEN** the user presses `n` and no search is currently
  active and `m.navSearch.hits` is empty
- **THEN** compose mode is entered (the existing behaviour is
  preserved)

#### Scenario: `j`/`k` reset the current-match index
- **WHEN** the user has cycled to the second of three matches
  with `n` and then presses `j`
- **THEN** the cursor moves down one line and the current-match
  index becomes `-1`; the dim highlight is preserved on the
  non-current matches; the next `n` advances from the new
  cursor position to the first hit at or after that byte

### Requirement: System SHALL highlight non-current matches with a background colour

The system SHALL render every non-current match in the viewport
with a dim background colour while a search is active OR
`m.navSearch.hits` is non-empty. The current match SHALL be
rendered through the existing cursor's inverted-block style.

The dim background colour SHALL be `\x1b[48;5;58m` (dark teal).
The splice SHALL restore the parent background on close, so a
hit inside a glamour-styled region (a fenced code block, today)
preserves the region's background on the cells immediately
before and after the hit.

The cursor's existing inverted-block splice SHALL run after the
dim splice, so on the current match the cursor's `\x1b[7m` paints
over the dim background. The visual result is "dim cells on
either side, inverted cell on the current match".

#### Scenario: dim background on a non-current match
- **WHEN** the query produces three matches and the cursor sits
  on the first match
- **THEN** the second and third matches are painted with
  `\x1b[48;5;58m` backgrounds on every cell in their byte
  range; the first match is painted with the cursor's
  inverted-block style on the first byte of its range

#### Scenario: code-block background preserved around a hit
- **WHEN** the source line is a fenced code block (glamour
  applies `\x1b[38;5;228;48;5;236m` per cell) and a hit lands
  on the line
- **THEN** the hit's cells have `\x1b[48;5;58m` background; the
  cells immediately before and after the hit in the same line
  have `\x1b[48;5;236m` background; no cells in the line drop
  to the terminal default background

#### Scenario: long line with a hit that crosses a wrap boundary
- **WHEN** a source line is longer than the wrap width, the
  hit starts on the first wrapped sub-line and ends on the
  second sub-line
- **THEN** the first sub-line is dim-highlighted from the
  hit's `byteA` to the end of the sub-line; the second
  sub-line is dim-highlighted from its sub-line start to the
  hit's `byteC` mapped into the second sub-line; the cursor's
  position is mapped through the same wrap walk to the
  correct sub-line

#### Scenario: highlight cleared on `Esc`
- **WHEN** the user has typed a query and pressed `Enter` to
  commit, then pressed `Esc`
- **THEN** the prompt is closed, the query is empty, no
  `\x1b[48;5;58m` background appears anywhere in the rendered
  text, and the viewport re-renders to the no-highlight state

#### Scenario: highlight cleared on a new message
- **WHEN** the agent session surfaces a new assistant message
  that differs from `m.latest.Text`
- **THEN** `m.navSearch.hits` is set to nil, the current-match
  index is `-1`, the prompt is closed if open, and the new
  message renders with no search highlight
