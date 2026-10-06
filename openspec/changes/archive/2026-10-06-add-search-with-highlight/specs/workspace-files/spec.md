## ADDED Requirements

### Requirement: Fuzzy search via `/` in stateFileView

In `stateFileView` the system SHALL provide a fuzzy search
affordance for the visible file content. The key surface and
behaviour is:

| key     | action |
|---------|--------|
| `/`     | open a one-line prompt at the bottom of the view; the query starts empty and the prompt shows `/` followed by a half-block cursor |
| `<rune>` | append the rune to the query; the visible highlight updates as the query changes; the prompt cursor advances by one cell |
| `Backspace` | drop the last rune from the query; the visible highlight updates |
| `Enter` | close the prompt, move the file viewer's cursor to the byte position of the first match (in document order: line ascending, then byte ascending) at or after the cursor's current `(cursor, charPos)`, reset `preferred` to the hit's `charPos`, and set the current-match index to that match |
| `Esc`   | close the prompt, clear the query, clear every highlight, leave the cursor at its current position, do not change `preferred` |
| `n`     | when a search is active OR `m.fileSearch.hits` is non-empty: advance the current-match index to the next hit in document order (wrapping from the last hit to the first), move the cursor to that hit's `(cursor, charPos)`, reset `preferred` to the hit's `charPos`, and re-render. When no search is active: no-op (no prior binding collides) |
| `N`     | when a search is active OR `m.fileSearch.hits` is non-empty: retreat the current-match index to the previous hit in document order (wrapping from the first hit to the last), move the cursor to that hit's `(cursor, charPos)`, reset `preferred` to the hit's `charPos`, and re-render. When no search is active: no-op |

While the prompt is open, every other printable rune (including
the file viewer's `j`/`k`/`h`/`l`/`w`/`b`/`v`/`c`/`d`/`R`/`K`/
`s`/`q`) is consumed by the query; the existing file-viewer
bindings are inactive. Arrow keys (`↑`/`↓`/`PageDown`/`PageUp`)
are NOT consumed by the prompt — they continue to scroll the
viewport, matching the file-navigator `/` behaviour.

The match algorithm SHALL be the same subsequence matcher the
file navigator's `/` and the message viewer's `/` use: every
rune of the (case-folded) query appears in the (case-folded)
source line in order. Every non-overlapping left-to-right match
on each line SHALL be returned as a `(lineIdx, byteA, byteC)`
triple, with the full list ordered by `(lineIdx, byteA)`. The
match algorithm SHALL operate on `m.fileViewer.lines` (the raw
file content split on `'\n'`); it SHALL NOT operate on the
rendered ANSI string.

An empty query SHALL produce no hits and no highlight; a query
with no matching characters SHALL produce no hits and no
highlight.

#### Scenario: `/` opens a prompt with empty query
- **WHEN** the user presses `/` in `stateFileView`
- **THEN** `m.fileSearch.active` becomes `true`, the prompt is
  rendered at the bottom of the view as `/` followed by a
  half-block cursor, and no highlight is drawn

#### Scenario: typing a query updates the highlight
- **WHEN** the user types `func` while the prompt is open and
  the file content contains `func` somewhere
- **THEN** every non-overlapping `func` match on every line is
  highlighted with the dim background colour; the prompt
  shows `/func`; the cursor is unchanged

#### Scenario: Backspace trims the query
- **WHEN** the user types `func` and then `Backspace`
- **THEN** the query becomes `fun`, the highlight re-renders
  to show only `fun` matches, and the prompt shows `/fun`

#### Scenario: `Enter` jumps cursor to the first match
- **WHEN** the user types `func` and presses `Enter` and the
  cursor is on line 1, charPos 0, and the first `func` match
  is on line 3, charPos 12
- **THEN** the prompt closes, the file viewer's
  `(cursor, charPos)` becomes `(3, 12)`, `preferred` is set
  to `12`, the viewport scrolls to keep the cursor visible,
  and the current-match index points at the first match (which
  is now visible as the cursor's inverted-block cell)

#### Scenario: `Enter` with no matches leaves the cursor where it is
- **WHEN** the user types `zzz` and presses `Enter` and no
  `zzz` match exists in the file
- **THEN** the prompt closes, no highlight remains, the cursor
  is unchanged, `preferred` is unchanged, and the
  current-match index is `-1`

#### Scenario: `Esc` clears query and highlights
- **WHEN** the user types `func` and presses `Esc`
- **THEN** the prompt closes, the query is empty, no highlight
  is drawn, the cursor is unchanged, and `preferred` is
  unchanged

#### Scenario: `n` cycles to the next match
- **WHEN** the current match is the second of three and the
  user presses `n`
- **THEN** the cursor moves to the third match's
  `(cursor, charPos)`, `preferred` is reset to the hit's
  `charPos`, the viewport scrolls to keep the cursor visible,
  and the current-match index advances by one

#### Scenario: `n` wraps from last to first
- **WHEN** the current match is the third (last) of three and
  the user presses `n`
- **THEN** the cursor moves to the first match's
  `(cursor, charPos)`, `preferred` is reset, and the
  current-match index becomes `0`

#### Scenario: `N` cycles to the previous match
- **WHEN** the current match is the second of three and the
  user presses `N`
- **THEN** the cursor moves to the first match's
  `(cursor, charPos)`, `preferred` is reset, and the
  current-match index retreats by one

#### Scenario: `N` wraps from first to last
- **WHEN** the current match is the first of three and the
  user presses `N`
- **THEN** the cursor moves to the third match's
  `(cursor, charPos)`, `preferred` is reset, and the
  current-match index becomes `2`

#### Scenario: `n` is a no-op when no search is active
- **WHEN** the user presses `n` and no search is currently
  active and `m.fileSearch.hits` is empty
- **THEN** no state changes and no key action fires

#### Scenario: `j`/`k` reset the current-match index
- **WHEN** the user has cycled to the second of three matches
  with `n` and then presses `j`
- **THEN** the cursor moves down one line with `preferred`
  preserved (existing `j` semantics), the current-match
  index becomes `-1`, the dim highlight is preserved on the
  non-current matches, and the next `n` advances from the new
  cursor position to the first hit at or after that byte

#### Scenario: `n` resets `preferred` even after `j`/`k`
- **WHEN** the user has typed a query, pressed `Enter` to
  commit (which set `preferred` to the hit's `charPos`),
  then moved with `j` (which preserves `preferred`), then
  pressed `n` to advance to the next match
- **THEN** `preferred` is reset to the new hit's `charPos`
  (not preserved from the post-`j` value)

### Requirement: System SHALL highlight non-current matches with a background colour in stateFileView

While a search is active OR `m.fileSearch.hits` is non-empty, the system SHALL render every non-current match in the visible viewport with a dim background colour (`\x1b[48;5;58m`, dark
teal). The current match SHALL be rendered through the existing
file-viewer cursor's inverted-block style.

The dim background splice SHALL run after `applySelection` and
before `ApplyCursor` in the per-line render. The cursor's
inverted-block splice SHALL run after the dim splice, so on the
current match the cursor's `\x1b[7m` paints over the dim
background. The yellow `▍` comment gutter is unaffected by the
dim splice (the gutter is column 0; hits start at `charPos ≥ 0`
on the body).

The dim background colour SHALL match the message viewer's
(`\x1b[48;5;58m`). The file viewer does not use glamour, so
parent-background restoration is a no-op (the only ANSI in
`renderFileContent` is the cyan selection and the cursor's
inverted block, both of which sit at the cell level and do not
extend beyond the splice).

#### Scenario: dim background on a non-current match
- **WHEN** the query produces three matches in the file and
  the cursor sits on the first match
- **THEN** the second and third matches are painted with
  `\x1b[48;5;58m` backgrounds on every cell in their byte
  range; the first match is painted with the cursor's
  inverted-block style on the first byte of its range

#### Scenario: dim background on a single line with multiple matches
- **WHEN** the query produces two matches on the same line and
  the cursor is on neither
- **THEN** both matches are dim-highlighted; the cells between
  them are not highlighted; the current-match index is `-1`

#### Scenario: dim background survives the visual selection
- **WHEN** the user has a visual selection active and types a
  query that hits a byte inside the selection
- **THEN** the dim background paints over the selection's
  cyan highlight at the hit bytes (last-write-wins on the
  cell); the selection still covers the rest of its range

#### Scenario: highlight cleared on `Esc`
- **WHEN** the user has typed a query and pressed `Enter` to
  commit, then pressed `Esc`
- **THEN** the prompt is closed, the query is empty, no
  `\x1b[48;5;58m` background appears anywhere in the rendered
  text, and the viewport re-renders to the no-highlight state

#### Scenario: highlight cleared on file change
- **WHEN** the user navigates from one file to another
  (via the dir navigator's `Enter` on a different file) while
  a search is active
- **THEN** `m.fileSearch.hits` is set to nil, the
  current-match index is `-1`, the prompt is closed if open,
  and the new file renders with no search highlight

#### Scenario: highlight cleared on file viewer exit
- **WHEN** the user presses `Esc` (or `q`) to leave the file
  viewer while a search is active
- **THEN** the file viewer's hits and prompt are cleared
  before the state transitions; re-entering the file viewer
  does not restore them
