# Spec Delta

## MODIFIED Requirements

### Requirement: Display assistant text as raw markdown source
The system SHALL display the assistant message text as raw
markdown source — verbatim, with no styling. Markdown markers
(`#`, `*`, `_`, backticks, ` ``` `, `|`, `>`) SHALL appear in
the output exactly as the agent wrote them. No glamour or other
markdown-to-ANSI renderer is involved; the bytes shown to the
user are the bytes the agent produced, sliced at source-line
boundaries.

#### Scenario: Headings appear with their `#` markers
- **WHEN** the assistant message contains a markdown heading
  (e.g., `## Step 1`)
- **THEN** the heading appears in the view starting with `## `
  (the marker is preserved verbatim)

#### Scenario: Fenced code block appears as raw fenced code
- **WHEN** the assistant message contains a fenced code block
  (e.g., ` ```go ... ``` `)
- **THEN** the opening and closing fence lines are visible in
  the view and the code inside is shown untransformed (no syntax
  highlighting)

#### Scenario: GFM table appears as raw pipe text
- **WHEN** the assistant message contains a GFM-flavoured
  markdown table
- **THEN** the header row, alignment row, and body rows appear
  verbatim — `|` characters included, no column alignment

#### Scenario: GFM definition list appears as raw definition text
- **WHEN** the assistant message contains a GFM definition list
- **THEN** the terms and `:` descriptions appear verbatim with
  no special rendering

#### Scenario: Long lines are not wrapped
- **WHEN** the assistant message contains a line longer than the
  viewport width
- **THEN** the line is shown on one row of the source (the
  viewport scrolls horizontally if needed; pinky does not
  word-wrap)

### Requirement: Single-letter nav surface in stateNav

In `stateNav` (formerly `stateIdle`) the system SHALL provide a
single-letter nav surface. The bindings and actions are:

| key  | action |
|------|--------|
| `j`  | move cursor to next source line; `charPos` becomes `min(preferred, len(newLine))`; `preferred` unchanged |
| `k`  | move cursor to previous source line; same `charPos` rule as `j` |
| `h`  | move cursor one rune left within the current source line |
| `l`  | move cursor one rune right within the current source line; `preferred = max(preferred, charPos)` |
| `v`  | enter visual mode (toggle); re-press while visual exits |
| `Esc`| exit visual mode (no-op when not in visual) |
| `c`  | open comment composer (selection-anchored if visual with a non-empty range, whole-current-line-anchored otherwise) |
| `s`  | send to agent (idle batch when comments exist; compose otherwise) |
| `n`  | enter compose mode |
| `r`  | fetch the tailed session once and re-render the latest message (the only post-attach fetch trigger) |
| `q`  | quit pinky |
| `?`  | toggle short / full help overlay |
| `↓`  | scroll viewport one line down (cursor unchanged) |
| `↑`  | scroll viewport one line up (cursor unchanged) |
| `PageDown` | scroll viewport one page down (cursor unchanged) |
| `PageUp`   | scroll viewport one page up (cursor unchanged) |
| `Home` | scroll viewport to top (cursor unchanged) |
| `End`  | scroll viewport to bottom (cursor unchanged) |

The previous two-key state machine (`gg`, `]]`, `[[`) and the
`Ctrl+C` / `Ctrl+N` / `Ctrl+R` / `Ctrl+S` / `Ctrl+I` bindings
SHALL NOT exist in `stateNav`. Scroll keys SHALL NOT move the
cursor — they move the viewport only. The viewport-follows-cursor
behaviour is unchanged: `j`/`k` still move the cursor and the
viewport scrolls to keep it visible with a 1-line cushion.

#### Scenario: `j` moves to the next block
- **WHEN** the user presses `j` in `stateNav` and the cursor is
  on line `L` of `len(lines) - 1`
- **THEN** the cursor's `lineIdx` becomes `L + 1`, `charPos`
  becomes `min(preferred, len(newLine))`, `preferred` is unchanged,
  and the viewport scrolls to keep the cursor visible

#### Scenario: `k` moves to the previous block
- **WHEN** the user presses `k` in `stateNav` and the cursor is
  on line `L > 0`
- **THEN** the cursor's `lineIdx` becomes `L - 1`, `charPos`
  becomes `min(preferred, len(newLine))`, `preferred` is unchanged,
  and the viewport scrolls to keep the cursor visible

#### Scenario: `h` moves one rune left
- **WHEN** the user presses `h` in `stateNav` and the cursor's
  `charPos` is greater than `0`
- **THEN** the cursor's `charPos` decreases by the byte length
  of the rune preceding it; `preferred` is unchanged

#### Scenario: `l` moves one rune right
- **WHEN** the user presses `l` in `stateNav` and the cursor's
  `charPos` is less than `len(currentLine)`
- **THEN** the cursor's `charPos` increases by the byte length
  of the rune at the current offset and `preferred` becomes
  `max(preferred, charPos)`

#### Scenario: Two-key sequences do not exist
- **WHEN** the user presses `g` followed by `g` in `stateNav`
- **THEN** the first `g` is processed as an unknown rune (no
  state, no timeout); the second `g` is processed as an unknown
  rune. No `gg` action fires.

#### Scenario: `↓` scrolls viewport without moving cursor
- **WHEN** the user presses `↓` in `stateNav`
- **THEN** the viewport's `YOffset` advances by one rendered
  line and the cursor's `(lineIdx, charPos, preferred)` is
  unchanged

#### Scenario: `Home` scrolls viewport to top without moving cursor
- **WHEN** the user presses `Home` in `stateNav`
- **THEN** the viewport's `YOffset` becomes `0` and the cursor's
  `(lineIdx, charPos, preferred)` is unchanged

### Requirement: Render comment annotations as overlay
The latest-message view SHALL be capable of rendering comment
annotations on top of the rendered assistant message. The
annotation overlay consists of:

- A line-direct yellow `▍` (foreground colour `228`) in the
  left-margin column on every source line whose byte range
  overlaps at least one saved comment's byte range (defined in
  `message-comments`).

The annotation overlay SHALL NOT modify the assistant message text
itself; the assistant message is shown verbatim and the overlay
sits on top of it. There SHALL be no footnote line below any
commented range; the only on-source comment signal is the yellow
line gutter.

#### Scenario: Line touched by a comment shows yellow gutter
- **WHEN** a saved comment's byte range covers at least one
  byte on source line `L`
- **THEN** source line `L` carries a yellow `▍` in its leftmost
  column

#### Scenario: Uncommented source line shows empty gutter column
- **WHEN** no saved comment's byte range covers any byte on
  source line `L`
- **THEN** source line `L` carries a single space in its
  leftmost column

#### Scenario: Commented block shows footnote below
- **WHEN** a saved comment exists with anchor covering one or
  more source lines
- **THEN** no footnote line is rendered below the commented
  range; the only on-source signal is the yellow line-direct
  gutter (see Line-direct gutter for saved comments in
  `message-comments`)

#### Scenario: Annotation overlay survives width reflow
- **WHEN** the terminal width changes and the rendered message
  is re-flowed
- **THEN** the comment annotations (the yellow gutter per line)
  are re-applied without loss; no annotation references a stale
  line index from the previous width

## REMOVED Requirements

### Requirement: Block focus indicator
**Reason**: The cyan `▍` focus gutter is replaced by an inline
block cursor at `charPos` (see Inline block cursor at charPos).
The cursor's byte position is now the focus signal; no per-line
or per-block gutter is drawn for focus.
**Migration**: No user-facing migration. The cyan gutter is
removed; the inline cursor at the byte the cursor points at is
the new focus indicator.

### Requirement: Build markdown block index
**Reason**: The markdown block index is no longer needed for
navigation or rendering. Block-derived gutters and the
footnote-below-block are replaced by line-direct rendering; the
`j`/`k` motion keys move by source line, not by block. The
markdown AST parser is removed.
**Migration**: No user-facing migration.

## ADDED Requirements

### Requirement: Inline block cursor at charPos

The system SHALL render an inline block cursor at the byte
position of the cursor's `charPos` on the rendered line
containing the cursor's `lineIdx`. The cursor SHALL be a single
visible cell — the rune at `charPos` (or a trailing space when
`charPos == len(currentLine)`) — styled with an inverted
background (foreground and background swapped relative to the
message body). The cursor SHALL be the only focus indicator;
no gutter, border, or background tint SHALL be drawn to indicate
which line or block the cursor is on. When the cursor lands on
a byte that is also inside an active visual selection, the
inline cursor style SHALL take precedence (the inverted block
paints over the selection's cyan highlight).

#### Scenario: Cursor renders at charPos on the cursor's line
- **WHEN** the cursor's `(lineIdx, charPos)` is `(L, K)` and the
  cursor is not inside an active visual selection
- **THEN** the rendered view paints the rune at `lines[L][K]`
  (or a trailing space when `K == len(lines[L])`) with an
  inverted-background style

#### Scenario: Cursor paints over active visual selection
- **WHEN** the cursor's `(lineIdx, charPos)` is `(L, K)` and an
  active visual selection covers byte `K` on line `L`
- **THEN** the inverted-background style of the cursor is
  applied on top of the selection's cyan highlight at that cell

#### Scenario: No gutter or border marks the cursor's line
- **WHEN** the cursor is on source line `L`
- **THEN** no `▍`, vertical bar, or background tint is drawn in
  any column to indicate "this line is focused"; the inline
  cursor at `charPos` is the only focus signal

#### Scenario: Empty gutter column on uncommented non-cursor lines
- **WHEN** the cursor is on source line `L` and source line `M`
  is rendered in the visible viewport
- **THEN** source line `M`'s leftmost column carries a single
  space (no cyan `▍`, no yellow `▍`, no border)