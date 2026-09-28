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

The system SHALL provide a `Tab` keybinding that toggles between
the message view (`stateNav`) and the file review tab (one of
`stateFileNav` / `stateFileView`). `Tab` SHALL be active in every
state except the comment composer, the picker, and the error
screen. Toggling from the message view to the file tab SHALL
enter `stateFileNav` (the dir navigator). Toggling from the
file tab back to the message view SHALL restore the message-view
state the user left (nav or compose).

#### Scenario: Tab from message view opens dir navigator

- **WHEN** the user presses `Tab` in `stateNav`
- **THEN** the TUI enters `stateFileNav` and the dir navigator
  is the active view

#### Scenario: Tab from file view returns to message view

- **WHEN** the user presses `Tab` in `stateFileNav` or
  `stateFileView`
- **THEN** the TUI returns to `stateNav` (or the message-view
  state the user toggled away from)

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
| `Tab`     | return to message view |
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

### Requirement: File viewer renders raw text with line numbers

The system SHALL load the file's raw content (UTF-8 bytes) and
render it with right-aligned 1-based line numbers in a
dedicated column plus a one-character left gutter whenever
`stateFileView` is entered for a given path. Lines that carry
at least one file-kind comment SHALL display the yellow `▍`
(foreground colour `228`) in the gutter; all other lines SHALL
display a space.

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

### Requirement: File viewer has a column cursor

The `fileViewer` SHALL track a column position in addition to its
line position. The column position SHALL be a non-negative byte
offset into the current line, clamped to `len(currentLine)` after
every cursor move. The column position SHALL be initialised to `0`
when a file is opened. The column position SHALL be the byte
position passed to LSP queries (`definition`, `references`,
`hover`).

#### Scenario: Opening a file sets the column to 0

- **WHEN** `openFileViewer(path)` runs
- **THEN** `fileViewer.charPos == 0` and the cursor sits at the
  start of line 1

#### Scenario: `j` preserves the column when possible

- **WHEN** the user presses `j` and the destination line's byte
  length is greater than or equal to the current `charPos`
- **THEN** `charPos` is unchanged after the move

#### Scenario: `j` clamps the column when the new line is shorter

- **WHEN** the user presses `j` and the destination line's byte
  length is less than the current `charPos`
- **THEN** `charPos` is set to the destination line's byte length

### Requirement: File viewer keys

In `stateFileView` the system SHALL provide the following keys:

| key       | action |
|-----------|--------|
| `j` / `↓` | move cursor down one line (in visual mode: extend line range to the next line, snapping `charA`/`charC` to the start/end of the destination line); the column cursor clamps to the new line's byte length |
| `k` / `↑` | move cursor up one line (in visual mode: same extension rule, reversed); the column cursor clamps to the new line's byte length |
| `l`       | advance the column cursor one rune within the current line; in visual mode, also advance the visual `CharC` |
| `h`       | retreat the column cursor one rune within the current line; in visual mode, also retreat the visual `CharC` |
| `v`       | enter / exit visual selection at the cursor's current `(line, charPos)` |
| `c`       | open comment composer anchored to selection (inline char range if visual mode is active, line range otherwise) |
| `d`       | send `textDocument/definition` at the cursor's `(line, charPos)`; jump on 1 result, picker on N>1 results, silent on 0 |
| `R`       | send `textDocument/references` at the cursor's `(line, charPos)`; picker on N≥1, silent on 0 |
| `K`       | send `textDocument/hover` at the cursor's `(line, charPos)`; render footer on 1+, silent on 0 |
| `s`       | submit all accumulated comments |
| `Esc`     | return to `stateFileNav` (also clears hover footer and visual mode); also sends `textDocument/didClose` |
| `Tab`     | return to message view |
| `q`       | quit pinky |
| `?`       | toggle short / full help overlay |

`h` and `l` SHALL always move the column cursor (no longer no-ops
outside visual mode). Visual-mode selection behaviour on `h`/`l`
SHALL be preserved: `CharA` and `CharC` track the cursor's column
exactly as today.

The LSP keys (`d`, `R`, `K`) SHALL be active only in
`stateFileView`; pressing them in any other state SHALL be a
no-op and SHALL NOT trigger an LSP request.

#### Scenario: `j` advances the cursor line

- **WHEN** the user presses `j` in `stateFileView` and the
  cursor is not on the last line
- **THEN** the cursor advances by one line

#### Scenario: `l` extends the visual cursor one rune

- **WHEN** the user is in `stateFileView` visual mode and the
  cursor is mid-line
- **THEN** the cursor's char offset advances by the byte
  length of the rune at the current position (clamped at the
  line end)

#### Scenario: `h` retreats the visual cursor one rune

- **WHEN** the user is in `stateFileView` visual mode and the
  cursor is mid-line
- **THEN** the cursor's char offset retreats by the byte
  length of the rune preceding the current position (clamped
  at `0`)

#### Scenario: `j` in visual mode extends the line range

- **WHEN** the user is in `stateFileView` visual mode on line
  5 and presses `j`
- **THEN** the cursor advances to line 6, `LineEnd` becomes 6,
  and `charC` snaps to the end of line 6

#### Scenario: `l` advances the column cursor outside visual mode

- **WHEN** the user is in `stateFileView`, visual mode is
  inactive, and the user presses `l`
- **THEN** the column cursor increases by the byte length of the
  rune at the current offset (clamped at `len(currentLine)`)

#### Scenario: `h` retreats the column cursor outside visual mode

- **WHEN** the user is in `stateFileView`, visual mode is
  inactive, and the user presses `h`
- **THEN** the column cursor decreases by the byte length of the
  rune preceding it (clamped at `0`)

#### Scenario: `c` with no selection comments the whole file

- **WHEN** the user is in `stateFileView` with no active visual
  selection and presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=1, LineEnd=<fileLines>)`

#### Scenario: `c` with a line-range selection

- **WHEN** the user is in `stateFileView` visual mode with the
  cursor on line 9 (anchor on line 4) and presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=4, LineEnd=9, CharStart=-1,
  CharEnd=-1)` — a line-range file comment, no inline excerpt

#### Scenario: `c` with an inline char selection

- **WHEN** the user is in `stateFileView` visual mode with the
  cursor at char offset 12 on line 5 (anchor at char offset 4
  on line 5) and presses `c`
- **THEN** the TUI enters `stateCommentComposer` with anchor
  `(Path=<path>, LineStart=5, LineEnd=5, CharStart=<4's
  byte offset>, CharEnd=<12's byte offset>, Source=<verbatim
  excerpt>)`

#### Scenario: `Esc` returns to dir navigator

- **WHEN** the user presses `Esc` in `stateFileView`
- **THEN** the TUI returns to `stateFileNav` and the file
  viewer's content is unloaded

#### Scenario: `Tab` returns to message view

- **WHEN** the user presses `Tab` in `stateFileView`
- **THEN** the TUI returns to `stateNav`; pressing `Tab` again
  returns the user to `stateFileView` at the same file

#### Scenario: `d` on a symbol with one definition jumps

- **WHEN** the cursor is on an identifier and `d` returns
  exactly one location (same file or different file)
- **THEN** the cursor moves to the location's `(line, char)`,
  the viewport scrolls to keep the cursor visible, and the
  state remains `stateFileView`; for cross-file locations the
  viewer is closed and reopened at the new path

#### Scenario: `d` on an overloaded symbol opens the picker

- **WHEN** the cursor is on a symbol that resolves to N>1
  definitions
- **THEN** the system enters `stateLSPPicker` with the
  locations and a label of "definition"

#### Scenario: `d` on whitespace or punctuation is silent

- **WHEN** the cursor is on a non-identifier position and `d`
  returns zero locations
- **THEN** the file viewer is unchanged and no picker appears

#### Scenario: `d` outside `stateFileView` is a no-op

- **WHEN** the user is in `stateNav` (or any state other than
  `stateFileView`) and presses `d`
- **THEN** no LSP request is sent and the state is unchanged

#### Scenario: `R` on a symbol opens the picker

- **WHEN** the cursor is on an identifier and `R` returns N≥1
  locations
- **THEN** the system enters `stateLSPPicker` with the
  locations and a label of "references"

#### Scenario: `R` with zero results is silent

- **WHEN** the cursor is on an identifier and `R` returns zero
  locations
- **THEN** the file viewer is unchanged and no picker appears

#### Scenario: `K` on an identifier renders the hover footer

- **WHEN** the cursor is on an identifier and `K` returns a
  non-empty `Hover`
- **THEN** the file viewer shows a single-line hover footer
  containing the first line of `Hover.Contents` until any
  other key is pressed

#### Scenario: `K` on whitespace is silent

- **WHEN** the cursor is on a non-identifier position and `K`
  returns a null `Hover`
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
In `stateFileView` the system SHALL provide the following scroll keys in
addition to the existing `j`/`k`/`h`/`l`/`v`/`c`/`s`/`Esc`/`Tab`/`q`/`?`
bindings:

| key             | action |
|-----------------|--------|
| `j` / `k`       | move the line cursor by one (existing behaviour); scroll the viewport so the cursor stays visible with a 1-line cushion when the cursor leaves the visible window |
| `↑` / `↓`       | forward to the file-viewer's viewport (scroll one line, cursor does not move) |
| `PageUp` / `PageDown` | forward to the file-viewer's viewport (scroll one page) |
| `Home` / `End`  | forward to the file-viewer's viewport (scroll to top / bottom) |

`j`/`k` SHALL continue to extend the visual selection when visual mode is
active; the scroll-follow behaviour SHALL NOT interfere with the
selection's `(LineA, LineC)` range. The line cursor SHALL keep advancing
past the viewport boundaries when `j`/`k` are held — pressing `j` from the
bottom of the file SHALL land the cursor on the last line and the
viewport SHALL scroll to keep it visible.

#### Scenario: `j` scrolls when the cursor leaves the bottom
- **WHEN** the user presses `j` and the resulting cursor line index
  exceeds `YOffset + Height - 1`
- **THEN** the viewport's `YOffset` advances so the cursor sits one line
  inside the visible window

#### Scenario: `k` scrolls when the cursor leaves the top
- **WHEN** the user presses `k` and the resulting cursor line index
  falls below `YOffset`
- **THEN** the viewport's `YOffset` retreats so the cursor sits one line
  inside the visible window

#### Scenario: Arrow keys scroll without moving the cursor
- **WHEN** the user presses `↓`
- **THEN** the viewport scrolls down one line and `m.fileViewer.cursor`
  is unchanged

#### Scenario: PageDown scrolls a page
- **WHEN** the user presses `PageDown`
- **THEN** the viewport scrolls down by `Height - 1` lines (matching the
  viewport package's built-in `HalfPageDown` / `PageDown` semantics)

#### Scenario: `End` jumps to the bottom of the file
- **WHEN** the user presses `End`
- **THEN** the viewport's `YOffset` becomes `maxYOffset` so the last line
  is at the bottom of the visible window

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