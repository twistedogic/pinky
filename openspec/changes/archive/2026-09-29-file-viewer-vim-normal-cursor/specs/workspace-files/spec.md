# Spec Delta: workspace-files

## MODIFIED Requirements

### Requirement: File viewer renders raw text with line numbers

The system SHALL load the file's raw content (UTF-8 bytes) and
render it with right-aligned 1-based line numbers in a
dedicated column plus a one-character left gutter whenever
`stateFileView` is entered for a given path. Lines that carry
at least one file-kind comment SHALL display the yellow `▍`
(foreground colour `228`) in the gutter; all other lines SHALL
display a space. The left gutter SHALL NOT carry any cursor
marker — the cursor's position is shown only inline.

The system SHALL render an inline block cursor at the current
`(cursor, charPos)`: a single visible cell, the rune at
`charPos` (or a trailing space when `charPos == len(currentLine)`),
styled with an inverted background (foreground and background
swapped relative to the file body). When the cursor lands on a
byte that is also inside an active visual selection, the cursor
style SHALL take precedence (the inverted block paints over the
selection's cyan highlight).

#### Scenario: Line numbers appear right-aligned

- **WHEN** the file has 12 lines
- **THEN** the rendered viewer shows `   1` through `  12` as
  the line-number column, each padded to the same width

#### Scenario: Commented line carries yellow gutter

- **WHEN** the file has a file-kind comment anchored to lines
  5-7
- **THEN** lines 5, 6, and 7 in the rendered viewer display a
  yellow `▍` in the left gutter

#### Scenario: Uncommented line carries empty gutter

- **WHEN** the file has no comments anchored to a given line
- **THEN** that line displays a space in the left gutter

#### Scenario: Cursor renders inline at (line, charPos)

- **WHEN** the cursor is at `(line 5, charPos 4)` on the line
  `    foo := 1` and the file body is unstyled
- **THEN** the rendered line shows the byte at `charPos 4` (the
  `f`) with an inverted-background style and every other byte
  unstyled; the left gutter on line 5 shows a space (no cursor
  marker)

#### Scenario: Cursor at end of line renders as a trailing block

- **WHEN** the cursor is at `(line 5, charPos = len(line 5))`
- **THEN** the rendered line shows a trailing cell (one space
  wide) with an inverted-background style at the position
  immediately after the last byte of the line

#### Scenario: Cursor paints over visual selection highlight

- **WHEN** visual is active and the cursor's byte position falls
  inside the selection byte range
- **THEN** that byte is rendered with the inverted cursor style
  rather than the cyan selection style

#### Scenario: Multi-line visual selection fully highlights interior lines

- **WHEN** visual mode is active with the anchor at `(5, 2)`
  and the cursor at `(7, 5)` (the selection spans lines 5
  through 7), and line 6 is the empty string
- **THEN** every byte on line 5 from offset 2 to `len(line 5)`
  is cyan, every byte on line 6 is cyan (including the empty
  line's lone trailing position), and every byte on line 7 from
  offset 0 to 5 is cyan; the cursor renders inverted at
  `(7, 5)`

### Requirement: File viewer has a column cursor

The `fileViewer` SHALL track two positions:

- a **line cursor** (1-based line index),
- a **column cursor** that is a non-negative byte offset into the
  current line, clamped to `len(currentLine)` after every cursor
  move, and snapped to the nearest rune boundary on every motion
  (`h`/`l`/`j`/`k`) — no cursor position SHALL fall in the middle
  of a multi-byte UTF-8 rune.

The column cursor SHALL be initialised to `0` when a file is
opened. The column cursor SHALL be one of two things:

1. a `preferred` column tracker — updated only by `h`/`l` (set to
   `max(preferred, columnCursor)` on every rune move), held
   unchanged by `j`/`k`, and reset to `0` whenever a file is
   opened;
2. the **on-screen column cursor** — when `j`/`k` moves the line
   cursor, the column cursor becomes `min(preferred, len(newLine))`.

`preferred` exists so a user who navigates to a column on one line
keeps that column when they `j`/`k` to a shorter line: the column
cursor clamps to the new line's byte length but `preferred` does
not change, so the next `j`/`k` to a longer line returns the
cursor to the originally intended column.

#### Scenario: Opening a file sets the column to 0

- **WHEN** `openFileViewer(path)` runs
- **THEN** `fileViewer.cursor == 1`, `fileViewer.charPos == 0`,
  and `fileViewer.preferred == 0`; the cursor sits at the start
  of line 1

#### Scenario: `j` preserves the column when possible

- **WHEN** the user is on line `N` with `preferred == K` and the
  user presses `j` to line `N+1`, where `len(line N+1) >= K`
- **THEN** `fileViewer.cursor == N+1` and `fileViewer.charPos ==
  K` (the column is preserved)

#### Scenario: `j` clamps the column when the new line is shorter

- **WHEN** the user is on line `N` with `preferred == K` and the
  user presses `j` to line `N+1`, where `len(line N+1) < K`
- **THEN** `fileViewer.cursor == N+1` and `fileViewer.charPos ==
  len(line N+1)` (clamped); `fileViewer.preferred` remains `K`

#### Scenario: `l` advances the column and raises preferred

- **WHEN** the cursor is at `(line 5, charPos 3)` and the user
  presses `l` to land on `(line 5, charPos 4)`
- **THEN** `fileViewer.charPos == 4` and `fileViewer.preferred`
  becomes `max(3, 4) = 4`

#### Scenario: `h` retreats the column but holds preferred

- **WHEN** `preferred == 7` and the user presses `h` from
  `(line 5, charPos 7)` to `(line 5, charPos 6)`
- **THEN** `fileViewer.charPos == 6` and `fileViewer.preferred`
  remains `7`

#### Scenario: `j` then `k` returns to the same preferred column

- **WHEN** the cursor is at `(line 4, charPos 2)` with
  `preferred == 10`, the user presses `j` to line 5 (byte
  length 4) and then `k` back to line 4
- **THEN** `fileViewer.cursor == 4` and `fileViewer.charPos == 2`
  (the original column on line 4 is restored)

#### Scenario: Rune boundary preservation on `l`

- **WHEN** the cursor is at `(line 5, charPos 4)` and the line
  contains the rune `é` (2 bytes) starting at offset 4, and the
  user presses `l`
- **THEN** `charPos` becomes `6` (the start of the next rune),
  not `5`

#### Scenario: Rune boundary preservation on `j`

- **WHEN** `preferred == K` and the destination line at line `N+1`
  begins with a multi-byte rune that ends at byte offset `M < K`
- **THEN** `charPos` becomes the byte offset of the rune
  containing column `K` (or the line end), never a byte in the
  middle of a rune

### Requirement: File viewer keys

In `stateFileView` the system SHALL provide the following keys.
The line cursor `(cursor, charPos)` is the **single source of
truth** for both navigation and any active visual selection's
moving end; visual mode stores only an anchor.

| key       | action |
|-----------|--------|
| `j` / `↓` | move the line cursor down one line; `charPos` becomes `min(preferred, len(newLine))`; `preferred` unchanged |
| `k` / `↑` | move the line cursor up one line; same column rule as `j` |
| `l`       | advance the column cursor one rune within the current line; `preferred = max(preferred, charPos)` |
| `h`       | retreat the column cursor one rune within the current line; `preferred` unchanged |
| `v`       | enter char visual selection at the cursor's current `(cursor, charPos)` (anchor = cursor); pressing `v` again exits visual |
| `Esc`     | exit char visual selection if active; otherwise return to `stateFileNav` and send `textDocument/didClose` |
| `c`       | open comment composer: anchor is `(Path, charA, charC)` of the visual range when visual is active, otherwise the current line `(Path, LineStart=cursor, LineEnd=cursor)` |
| `d`       | send `textDocument/definition` at the **word under the cursor**; jump on 1 result, picker on N>1 results, silent on 0 |
| `R`       | send `textDocument/references` at the **word under the cursor**; picker on N≥1, silent on 0 |
| `K`       | send `textDocument/hover` at the **word under the cursor**; render footer on 1+, silent on 0 |
| `s`       | submit all accumulated comments |
| `Tab`     | return to message view |
| `q`       | quit pinky |
| `?`       | toggle short / full help overlay |

Visual selection is **char-only**: when visual is active, `j`/`k`
move the line cursor and update `charPos` per the column rules
above; the anchor stays put. The selection byte range is
`[min(anchor, cursor), max(anchor, cursor)]` interpreted as
`(line, charPos)` ordered lexicographically across lines. The
selection spans lines from `min(anchor.line, cursor.line)` to
`max(anchor.line, cursor.line)`: interior lines are fully
highlighted, and the first and last lines show partial byte
ranges between the line endpoint and the corresponding
anchor/cursor char offset.

The LSP keys (`d`, `R`, `K`) SHALL be active only in
`stateFileView`; pressing them in any other state SHALL be a
no-op and SHALL NOT trigger an LSP request. "Word under the
cursor" means the contiguous run of `[A-Za-z0-9_]` bytes that
contains `charPos`; if `charPos` falls on a non-word byte the LSP
request SHALL be sent at `charPos` unchanged.

#### Scenario: `j` advances the cursor line

- **WHEN** the user presses `j` in `stateFileView` and the
  cursor is not on the last line
- **THEN** the cursor advances by one line

#### Scenario: `l` advances the column cursor outside visual mode

- **WHEN** the user is in `stateFileView`, visual mode is
  inactive, and the user presses `l`
- **THEN** the column cursor increases by the byte length of the
  rune at the current offset (clamped at `len(currentLine)`) and
  `preferred = max(preferred, charPos)`

#### Scenario: `h` retreats the column cursor outside visual mode

- **WHEN** the user is in `stateFileView`, visual mode is
  inactive, and the user presses `h`
- **THEN** the column cursor decreases by the byte length of the
  rune preceding it (clamped at `0`); `preferred` unchanged

#### Scenario: `l` extends the visual cursor one rune

- **WHEN** the user is in `stateFileView` visual mode and the
  cursor is mid-line
- **THEN** the cursor's char offset advances by the byte length
  of the rune at the current position (clamped at the line end)
  and `preferred = max(preferred, charPos)`; the visual
  selection's `CharC` follows the cursor's char position

#### Scenario: `h` retreats the visual cursor one rune

- **WHEN** the user is in `stateFileView` visual mode and the
  cursor is mid-line
- **THEN** the cursor's char offset retreats by the byte length
  of the rune preceding the current position (clamped at `0`);
  `preferred` unchanged; the visual selection's `CharC` follows
  the cursor's char position

#### Scenario: `j` in visual mode extends the line range

- **WHEN** the user is in `stateFileView` visual mode on line
  5 and presses `j`
- **THEN** the line cursor advances to line 6 and `charPos`
  becomes `min(preferred, len(line 6))`; the visual selection
  spans from the anchor (line, charPos) through the cursor
  (line 6, charPos) — no whole-line snap is applied

#### Scenario: `c` with no selection comments the whole file

- **WHEN** the user is in `stateFileView` with no active visual
  selection and the cursor on line 7, and presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=7, LineEnd=7, CharStart=-1,
  CharEnd=-1)` — the **current line** (the prior "whole file"
  semantics are dropped; feedback for an entire file now reaches
  the agent through the compose path)

#### Scenario: `c` with a line-range selection

- **WHEN** the user is in `stateFileView` visual mode with the
  anchor at `(5, 4)` and the cursor at `(7, 9)`, and presses
  `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=5, LineEnd=7, CharStart=-1,
  CharEnd=-1)` — a line-range file comment covering the lines
  touched by the char selection, no inline excerpt (the comment
  format has no multi-line inline kind)

#### Scenario: `c` with an inline char selection

- **WHEN** the user is in `stateFileView` visual mode with the
  anchor at `(5, 4)` and the cursor at `(5, 12)`, and presses
  `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=5, LineEnd=5, CharStart=4, CharEnd=12,
  Source=<verbatim excerpt>)`

#### Scenario: `c` with a single-line point visual falls back to whole line

- **WHEN** the user is in `stateFileView` visual mode with the
  anchor and cursor both at `(5, 4)` (single-line empty
  selection), and presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=5, LineEnd=5, CharStart=-1,
  CharEnd=-1)` — an empty single-line visual falls back to the
  whole-line semantics, indistinguishable from no-visual

#### Scenario: `c` with a multi-line point visual yields the line range

- **WHEN** the user is in `stateFileView` visual mode with the
  anchor at `(5, 0)` and the cursor at `(7, 0)` (multi-line
  selection whose byte range covers all of lines 5 and 6 plus the
  empty start of line 7), and presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=5, LineEnd=7, CharStart=-1,
  CharEnd=-1)` — the existing multi-line-char → line-range
  collapse rule applies verbatim, no special case for empty-byte
  multi-line selections

#### Scenario: `Esc` returns to dir navigator

- **WHEN** the user presses `Esc` in `stateFileView` with
  visual inactive
- **THEN** the TUI returns to `stateFileNav` and the file
  viewer's content is unloaded; `textDocument/didClose` is sent

#### Scenario: `Esc` from char visual stays in the file viewer

- **WHEN** the user is in `stateFileView` with char visual
  active and presses `Esc`
- **THEN** visual becomes inactive, the selection is cleared,
  and the TUI remains in `stateFileView`

#### Scenario: `q` in char visual quits pinky immediately

- **WHEN** the user is in `stateFileView` with char visual
  active and presses `q`
- **THEN** pinky exits without exiting visual first; the
  selection state is discarded along with the rest of the
  process state

#### Scenario: `Tab` returns to message view

- **WHEN** the user presses `Tab` in `stateFileView`
- **THEN** the TUI returns to `stateNav`; pressing `Tab` again
  returns the user to `stateFileView` at the same file

#### Scenario: `Tab` preserves visual across the round-trip

- **WHEN** the user is in `stateFileView` with visual active
  (anchor at `(5, 4)`, cursor at `(7, 9)`) and presses `Tab`,
  then presses `Tab` again to return to the file viewer
- **THEN** the TUI is back in `stateFileView` with visual still
  active and the anchor and cursor positions unchanged
  (`(5, 4)` and `(7, 9)` respectively); the cyan selection
  highlight re-renders identically

#### Scenario: `Esc` to dir nav clears visual

- **WHEN** the user is in `stateFileView` with visual active
  and presses `Esc` to return to `stateFileNav`, then re-opens
  the same file
- **THEN** the file viewer opens at `(cursor=1, charPos=0,
  visual.Active=false)` — visual is reset on file open,
  independently of the Tab round-trip path

#### Scenario: `d` on a symbol with one definition jumps

- **WHEN** the cursor is anywhere inside the contiguous
  `[A-Za-z0-9_]` run that forms an identifier and `d` returns
  exactly one location (same file or different file)
- **THEN** the cursor moves to the location's `(line, char)`,
  the viewport scrolls to keep the cursor visible, and the state
  remains `stateFileView`; for cross-file locations the viewer
  is closed and reopened at the new path

#### Scenario: `d` on an overloaded symbol opens the picker

- **WHEN** the cursor is on a word that resolves to N>1
  definitions
- **THEN** the system enters `stateLSPPicker` with the
  locations and a label of "definition"

#### Scenario: `d` on whitespace or punctuation is silent

- **WHEN** the cursor is on a non-word byte and `d` returns
  zero locations
- **THEN** the file viewer is unchanged and no picker appears
  (the request was sent at `charPos` unchanged)

#### Scenario: `d` outside `stateFileView` is a no-op

- **WHEN** the user is in `stateNav` (or any state other than
  `stateFileView`) and presses `d`
- **THEN** no LSP request is sent and the state is unchanged

#### Scenario: `R` on a symbol opens the picker

- **WHEN** the cursor is on a word and `R` returns N≥1
  locations
- **THEN** the system enters `stateLSPPicker` with the
  locations and a label of "references"

#### Scenario: `R` with zero results is silent

- **WHEN** the cursor is on a word and `R` returns zero
  locations
- **THEN** the file viewer is unchanged and no picker appears

#### Scenario: `K` on an identifier renders the hover footer

- **WHEN** the cursor is on a word and `K` returns a non-empty
  `Hover`
- **THEN** the file viewer shows a single-line hover footer
  containing the first line of `Hover.Contents` until any other
  key is pressed

#### Scenario: `K` on whitespace is silent

- **WHEN** the cursor is on a non-word byte and `K` returns a
  null `Hover`
- **THEN** the file viewer is unchanged and no footer appears

### Requirement: File viewer scroll keys

In `stateFileView` the system SHALL provide the following scroll
keys in addition to the cursor-movement and selection bindings:

| key             | action |
|-----------------|--------|
| `↑` / `↓`       | forward to the file-viewer's viewport (scroll one line, cursor does not move) |
| `PageUp` / `PageDown` | forward to the file-viewer's viewport (scroll one page) |
| `Home` / `End`  | forward to the file-viewer's viewport (scroll to top / bottom) |

When `j`/`k` move the line cursor, the viewport SHALL scroll so
the cursor stays visible with a 1-line cushion when the cursor
leaves the visible window. When char visual is active, the
moving end of the selection follows the cursor; the selection
byte range is recomputed as `[min(anchor, cursor),
max(anchor, cursor)]` on every cursor move (no line-range snap
applies).

#### Scenario: `j` scrolls when the cursor leaves the bottom

- **WHEN** the user presses `j` and the resulting cursor line
  index exceeds `YOffset + Height - 1`
- **THEN** the viewport's `YOffset` advances so the cursor sits
  one line inside the visible window

#### Scenario: `k` scrolls when the cursor leaves the top

- **WHEN** the user presses `k` and the resulting cursor line
  index falls below `YOffset`
- **THEN** the viewport's `YOffset` retreats so the cursor sits
  one line inside the visible window

#### Scenario: Arrow keys scroll without moving the cursor

- **WHEN** the user presses `↓`
- **THEN** the viewport scrolls down one line and
  `m.fileViewer.cursor` is unchanged

#### Scenario: PageDown scrolls a page

- **WHEN** the user presses `PageDown`
- **THEN** the viewport scrolls down by `Height - 1` lines
  (matching the viewport package's built-in `HalfPageDown` /
  `PageDown` semantics)

#### Scenario: `End` jumps to the bottom of the file

- **WHEN** the user presses `End`
- **THEN** the viewport's `YOffset` becomes `maxYOffset` so the
  last line is at the bottom of the visible window
