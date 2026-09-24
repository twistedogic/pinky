## ADDED Requirements

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
