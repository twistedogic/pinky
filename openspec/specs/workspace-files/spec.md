# workspace-files Specification

## Purpose
Lets the user browse the agent pane's working directory (with
`.gitignore` respected), open a file, and stage comments anchored
to the file's path + line range or path + char range. A `Tab`
keypress toggles the message view and the file review tab; both
tabs share a unified `s`-flush path for comments. The file viewer
also wires in a read-only Language Server Protocol (LSP) client
for definition (`d`), references (`R`), and hover (`K`) on the
currently-open file.

## Requirements

### Requirement: Tab toggle between message and file views

The system SHALL provide a `Tab` keybinding that cycles between
the message view (`stateNav`), the file review tab (one of
`stateFileNav` / `stateFileView`), and the todo tab
(`stateTodoList`). The cycle order SHALL be Message → Files →
Todos → Message. `Tab` SHALL be active in every state except the
comment composer, the picker, and the error screen. Cycling into
a tab SHALL enter that tab's default sub-state (message:
`stateNav`, files: `stateFileNav`, todos: `stateTodoList`).
Cycling back to a previously-visited tab SHALL restore the
sub-state the user left in that tab.

#### Scenario: Tab from message view opens dir navigator
- **WHEN** the user presses `Tab` in `stateNav`
- **THEN** the TUI enters `stateFileNav` and the dir navigator
  is the active view

#### Scenario: Tab from file view returns to message view
- **WHEN** the user presses `Tab` in `stateFileNav` or
  `stateFileView`
- **THEN** the TUI advances one tab in the cycle (enters
  `stateTodoList`); two more `Tab` presses complete the cycle
  back to the message view

#### Scenario: Tab from todo tab returns to message tab
- **WHEN** the user presses `Tab` in `stateTodoList` or
  `stateTodoEdit`
- **THEN** the TUI returns to the message-view state the user
  toggled away from (nav or compose)

#### Scenario: Tab in composer is a no-op
- **WHEN** the user presses `Tab` in `stateCommentComposer`,
  `statePicking`, or `stateError`
- **THEN** the state is unchanged

### Requirement: Gitignore-aware workspace walk

When the file review tab is entered, the system SHALL walk the
agent pane's `pane_current_path` recursively and produce a flat
list of entries (files and directories) honouring every
`.gitignore` file in the tree (including nested `.gitignore`s).
The walker SHALL NOT follow symlinks. The walker SHALL skip the
`.git` directory regardless of `.gitignore` content. The entry
slice SHALL be sorted by `(Depth, Path)` so the navigator
renders deterministically.

#### Scenario: Basic gitignore excludes matched paths

- **WHEN** the workspace root contains a `.gitignore` listing
  `node_modules`
- **THEN** the walker does not include any path under
  `node_modules/` in the entry slice

#### Scenario: Nested gitignore applies within its subtree

- **WHEN** a directory `foo/` contains its own `.gitignore`
  listing `*.gen.go`
- **THEN** the walker excludes `foo/x.gen.go` and
  `foo/bar/y.gen.go` but still includes `bar/x.gen.go` (which
  lives outside `foo/`)

#### Scenario: Negation pattern re-includes a path

- **WHEN** a `.gitignore` contains `*.log` and `!keep.log`
- **THEN** the walker excludes `*.log` but includes `keep.log`

#### Scenario: Symlinks are not followed

- **WHEN** the workspace contains a symlink loop
  (`a/link → b/`, `b/link → a/`)
- **THEN** the walker terminates without infinite recursion and
  each link appears at most once

### Requirement: Dir navigator renders a tree

The dir navigator SHALL render the entry slice as an indented
tree: directories prefixed with `▾` (expanded) or `▸` (collapsed)
and files with no prefix. `Depth` controls indentation (one
space per level, capped at the rendered width). The currently
focused entry SHALL carry a `▶` marker and a brighter style;
other entries SHALL render in a dim style.

#### Scenario: Cursor highlights the focused entry

- **WHEN** the cursor is on entry index `N`
- **THEN** the rendered tree shows `▶` next to entry `N` and a
  dim style for all other entries

#### Scenario: Collapsed directory hides its descendants

- **WHEN** a directory is collapsed
- **THEN** its descendant entries do not appear in the rendered
  tree; only the directory itself is shown

#### Scenario: Expanded directory shows its immediate children

- **WHEN** a directory is expanded and has no further collapse
  flag
- **THEN** the rendered tree shows the directory and its
  immediate children (one level deeper)

### Requirement: Dir navigator keys

In `stateFileNav` the system SHALL provide the following keys:

| key       | action |
|-----------|--------|
| `j` / `↓` | move cursor down |
| `k` / `↑` | move cursor up |
| `h`       | collapse current directory, or jump to parent entry |
| `l`       | expand current directory, or jump to first child |
| `Enter`   | on a file → enter `stateFileView`; on a directory → toggle collapse |
| `c`       | on a file → open comment composer with whole-file anchor |
| `s`       | submit all accumulated comments |
| `Esc`     | return to message view |
| `Tab`     | advance to the next tab in the cycle |
| `q`       | quit pinky |
| `?`       | toggle short / full help overlay |

#### Scenario: `j` advances the cursor

- **WHEN** the user presses `j` in `stateFileNav` and there is
  a next entry
- **THEN** the cursor advances by one

#### Scenario: `h` on a collapsed directory is a no-op

- **WHEN** the cursor is on an already-collapsed directory and
  the user presses `h`
- **THEN** the collapse state is unchanged and the cursor does
  not move

#### Scenario: `l` on a file is a no-op

- **WHEN** the cursor is on a file (no children) and the user
  presses `l`
- **THEN** the cursor does not move and the entry's collapse
  state is unchanged

#### Scenario: Enter on a file opens the viewer

- **WHEN** the cursor is on a file and the user presses `Enter`
- **THEN** the TUI enters `stateFileView` and the file's
  content is loaded into `m.fileViewer`

#### Scenario: Enter on a directory toggles collapse

- **WHEN** the cursor is on a directory and the user presses
  `Enter`
- **THEN** the directory's collapse flag is flipped; if it is
  now collapsed, its descendants disappear from the tree; if
  expanded, they reappear

#### Scenario: `c` on a file opens the composer

- **WHEN** the cursor is on a file and the user presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<relative path>, LineStart=1, LineEnd=<line count>)`

#### Scenario: `c` on a directory is a no-op

- **WHEN** the cursor is on a directory and the user presses
  `c`
- **THEN** the TUI remains in `stateFileNav` and no composer
  is opened

#### Scenario: Tab from dir navigator advances to todo tab
- **WHEN** the user presses `Tab` in `stateFileNav`
- **THEN** the TUI enters `stateTodoList`

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
| `Tab`     | advance to the next tab in the cycle |
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

#### Scenario: `w` advances to the next word within the file

- **WHEN** the user presses `w` in `stateFileView` and the cursor
  is in a word, on a separator, or mid-line
- **THEN** the cursor advances to the start of the next word:
  first across the rest of the current word run (if any), then
  past any separators (whitespace and any blank lines), landing
  on the byte offset of the first rune of the next word. A word
  is a maximal run of `[A-Za-z0-9_]`, or a punctuation run
  (vim's `iskeyword` model). `preferred` becomes
  `max(preferred, charPos)`.

#### Scenario: `b` retreats to the previous word within the file

- **WHEN** the user presses `b` in `stateFileView`
- **THEN** the cursor retreats to the start of the previous
  word, crossing `\n` and skipping blank lines as separators.
  `preferred` is unchanged.

#### Scenario: `w` / `b` no-op at file boundaries

- **WHEN** the cursor is on the last word of the last line and
  the user presses `w`, OR the cursor is on the first word of the
  first line and the user presses `b`
- **THEN** the cursor's `(cursor, charPos, preferred)` is
  unchanged.

#### Scenario: `vw` extends a visual selection across words

- **WHEN** the user is in `stateFileView` visual mode and presses
  `w`
- **THEN** the cursor advances to the next word's start, the
  visual selection's anchor stays put at the original
  `(LineA, CharA)`, and the highlight tracks the byte range
  between the anchor and the new cursor position.

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
- **THEN** the TUI advances one tab in the cycle (enters
  `stateTodoList`); two more `Tab` presses complete the cycle
  back to the message view

#### Scenario: `Tab` preserves visual across the round-trip

- **WHEN** the user is in `stateFileView` with visual active
  (anchor at `(5, 4)`, cursor at `(7, 9)`) and presses `Tab` to
  enter `stateTodoList`, then presses `Tab` twice more (Todos
  → Message → Files) to return to the file viewer
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

### Requirement: Shared location picker

The system SHALL provide a `stateLSPPicker` that shows a list of
locations. The picker SHALL be shared between `d` (when N>1) and
`R` (when N≥1). Each entry in the picker SHALL display:

```
<relpath>:<line>:<col>  <snippet>
```

where `<relpath>` is the location's URI path relative to the pane
cwd, `<line>` and `<col>` are 1-based, and `<snippet>` is the
single-line source at that location (or a single-line excerpt if
the line is too long).

The picker's keys SHALL be:

| key       | action |
|-----------|--------|
| `j` / `↓` | move cursor down |
| `k` / `↑` | move cursor up |
| `Enter`   | select the focused location |
| `Esc`     | dismiss the picker and return to `stateFileView` |
| `q`       | quit pinky |

Selecting a location SHALL execute the same jump semantics as
single-result `d`: same-file → cursor move + scroll; cross-file →
close current viewer + open new file at `(line, char)`.

#### Scenario: Picking a same-file location jumps the cursor

- **WHEN** the picker is showing N locations, the focused entry
  is in the current file, and the user presses Enter
- **THEN** the picker is dismissed, the cursor moves to the
  entry's `(line, char)`, the viewport scrolls so the cursor is
  visible, and the TUI returns to `stateFileView`

#### Scenario: Picking a cross-file location opens the new file

- **WHEN** the picker is showing N locations, the focused entry
  is in a different file, and the user presses Enter
- **THEN** the current file viewer is closed, a new file viewer
  opens at the entry's path with the cursor at the entry's
  `(line, char)`, and the TUI returns to `stateFileView`

#### Scenario: `Esc` dismisses the picker without selecting

- **WHEN** the picker is open and the user presses `Esc`
- **THEN** the picker is dismissed and the TUI returns to
  `stateFileView` with the cursor at its previous position

### Requirement: Hover footer

When a hover response is rendered, the system SHALL display a
single line of text immediately below the file body and above the
help line. The hover footer SHALL occupy the height that the help
line would have occupied (i.e. it does not reduce the file viewer's
visible row count by more than one line). The hover footer SHALL be
cleared when:

- the user presses any key that is not `K`,
- the user presses `K` again (a new hover query replaces the
  footer once it resolves),
- the file viewer transitions to a different state.

#### Scenario: Hover footer renders below the file body

- **WHEN** `K` returns a non-empty hover and the user has not
  pressed any other key since
- **THEN** the file viewer shows: file body rows, then one row of
  hover content, then the help line

#### Scenario: `j` clears the hover footer

- **WHEN** the hover footer is visible and the user presses `j`
- **THEN** the cursor moves down one line and the hover footer
  is no longer rendered

#### Scenario: Multi-line hover content is truncated

- **WHEN** the hover response's `Contents` contains newlines
- **THEN** the hover footer shows the substring before the first
  newline, followed by `…`

### Requirement: File viewer scrolls with a viewport
The file viewer in `stateFileView` SHALL render the opened file through a
`bubbles/viewport.Model` whose size matches the message viewport's size
(same width, same reserved-height formula in `reflow()`). The viewer SHALL
open with `YOffset = 0` (top of file). The header line SHALL show
`lines <topVisible+1>-<topVisible+Height> of <total>` whenever the file has
more lines than fit in the viewport, so the user can see their position;
for files that fit entirely the indicator SHALL be omitted.

#### Scenario: Long file is scrollable
- **WHEN** the opened file has more lines than the viewport height
- **THEN** the user can scroll the viewport using the keys listed in the
  next requirement and the rendered window updates accordingly

#### Scenario: Short file shows no indicator
- **WHEN** the opened file fits entirely within the viewport height
- **THEN** the header line does not include a `lines N-M of K` indicator

#### Scenario: File opens at top
- **WHEN** a file is opened into `stateFileView`
- **THEN** the viewport's `YOffset` is `0` so the first line is visible

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

### Requirement: Server lifecycle is lazy and forgiving

The system SHALL spawn Language Server Protocol (LSP) servers
lazily on the first file-view query for a given language. The
system SHALL use the server registry provided by
`github.com/charmbracelet/x/powernap` (gopls,
typescript-language-server, rust-analyzer, clangd,
jedi-language-server by default).

If a server's binary is not found on `PATH`, the system SHALL:

1. Show a one-line footer in the file viewer that names the
   server and the install command (e.g.
   `gopls not found — install with: go install golang.org/x/tools/gopls@latest`).
2. Mark the server as `unavailable` for 30 seconds.
3. Stay quiet for those 30 seconds; subsequent key presses during
   the unavailable window do not re-show the hint.
4. After 30 seconds, allow the next query to retry the lookup.

The server binary's discovery SHALL use the standard `PATH`. The
system SHALL NOT consult environment variables, configuration
files, or per-language overrides.

#### Scenario: First `d` triggers a server spawn

- **WHEN** the user opens a `.go` file and presses `d` for the
  first time in the session
- **THEN** the system spawns `gopls`, sends `didOpen`, and sends
  `textDocument/definition`; the response is rendered once it
  arrives

#### Scenario: Missing gopls shows the install hint

- **WHEN** the user opens a `.go` file and `gopls` is not on `PATH`
- **THEN** the file viewer shows a one-line footer with the
  install hint, and the system marks gopls as unavailable for 30
  seconds

#### Scenario: Re-pressing `d` during the unavailable window is quiet

- **WHEN** the user presses `d` again within 30 seconds of a
  missing-server hint
- **THEN** the hint is not re-shown and no new process is spawned

### Requirement: LSP interactions are read-only

The system SHALL NOT send any LSP message that mutates a file, a
buffer, or the workspace. Specifically, the system SHALL NOT
invoke:

- `textDocument/rename`
- `textDocument/completion`
- `textDocument/codeAction`
- `workspace/applyEdit`
- `workspace/executeCommand`
- `workspace/workspaceEdit`

When the server sends a `workspace/applyEdit` request to the
client, the system SHALL reply with:

```json
{
  "applied": false,
  "failureReason": "pinky is read-only"
}
```

The system SHALL accept and ignore unsolicited
`textDocument/publishDiagnostics` notifications in v0.

#### Scenario: Server-initiated `applyEdit` is refused

- **WHEN** the LSP server sends a `workspace/applyEdit` request
- **THEN** the system replies with `applied: false` and the file
  on disk is unchanged

### Requirement: Per-file LSP lifecycle

When `openFileViewer(path)` runs, the system SHALL send a
`textDocument/didOpen` notification to the appropriate LSP server
for the file at `path`. When the file viewer closes (Esc to dir
nav, Tab to message view, `q` to quit) the system SHALL send a
`textDocument/didClose` notification for the same path.

The system SHALL NOT send `textDocument/didChange` notifications
(pinky is read-only and never edits the file).

#### Scenario: Opening a file sends `didOpen`

- **WHEN** `openFileViewer(path)` is called
- **THEN** the LSP client sends `textDocument/didOpen` for `path`
  before any query is sent

#### Scenario: Esc from the file viewer sends `didClose`

- **WHEN** the user presses `Esc` in `stateFileView`
- **THEN** the LSP client sends `textDocument/didClose` for the
  currently-open path before transitioning to `stateFileNav`
