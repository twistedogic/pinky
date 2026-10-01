# Spec Delta

## MODIFIED Requirements

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
ranges between the line end and the corresponding anchor/cursor
char offset.

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
  enter `stateTodoList`, then presses `Tab` twice more (Todos →
  Message → Files) to return to the file viewer
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

## REMOVED Requirements
<!-- None -->