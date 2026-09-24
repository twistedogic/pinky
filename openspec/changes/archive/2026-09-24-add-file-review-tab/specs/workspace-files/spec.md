# workspace-files Specification

## Purpose
TBD - created by archiving change add-file-review-tab. Update Purpose after archive.
## ADDED Requirements

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

### Requirement: File viewer keys

In `stateFileView` the system SHALL provide the following keys:

| key       | action |
|-----------|--------|
| `j` / `↓` | move cursor down one line (in visual mode: extend line range to the next line, snapping `charA`/`charC` to the start/end of the destination line) |
| `k` / `↑` | move cursor up one line (in visual mode: same extension rule, reversed) |
| `l`       | in visual mode: advance the cursor one rune within the current line; outside visual mode: no-op |
| `h`       | in visual mode: retreat the cursor one rune within the current line; outside visual mode: no-op |
| `v`       | enter / exit visual selection at the cursor's current `(line, charPos)` |
| `c`       | open comment composer anchored to selection (inline char range if visual mode is active, line range otherwise) |
| `s`       | submit all accumulated comments |
| `Esc`     | return to `stateFileNav` |
| `Tab`     | return to message view |
| `q`       | quit pinky |
| `?`       | toggle short / full help overlay |

Visual mode in the file viewer mirrors the message viewer's
visual mode: `h`/`l` operate at rune granularity within the
current line, `j`/`k` extend the line range with `charA`
snapping to the start of the first line and `charC` snapping
to the end of the last line.

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

### Requirement: Tab indicator in the status bar

The status line SHALL show a small chip ("msg" / "files")
indicating the active tab. The chip SHALL render in dim style
and SHALL update whenever `m.tab` changes.

#### Scenario: Status bar shows "msg" in message view

- **WHEN** the TUI is in `stateNav` or `stateCompose`
- **THEN** the status line contains the `msg` chip

#### Scenario: Status bar shows "files" in file tab

- **WHEN** the TUI is in `stateFileNav` or `stateFileView`
- **THEN** the status line contains the `files` chip
