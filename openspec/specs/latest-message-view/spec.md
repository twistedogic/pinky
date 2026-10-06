# latest-message-view Specification

## Purpose
TBD - created by archiving change focus-latest-message. Update Purpose after archive.
## Requirements
### Requirement: Show latest complete agent message
The system SHALL display only the most recent assistant message in the main view. Older assistant messages and prior conversation history SHALL NOT be displayed. When a new assistant message is surfaced, the view SHALL be replaced with the new message.

#### Scenario: First message arrives
- **WHEN** pinky is attached to a session and the agent produces its first assistant message
- **THEN** the message appears in the main view

#### Scenario: New message replaces the previous
- **WHEN** a second assistant message is surfaced while a first is already shown
- **THEN** the main view is replaced with the second message and the first is no longer displayed

#### Scenario: Empty state before any message
- **WHEN** pinky is attached to a session and no assistant message has been surfaced yet
- **THEN** the main view shows a "waiting for agent…" placeholder

#### Scenario: Polling yields no new content
- **WHEN** the agent session is polled and no new assistant text has been surfaced
- **THEN** the main view is unchanged

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

### Requirement: Yank to bottom on new content
The system SHALL keep the viewport pinned to the bottom of the rendered
message while the user is at the bottom, and SHALL release the pin the
moment the user scrolls up. While the pin is released, polling the agent
session and receiving new text SHALL NOT change the viewport's vertical
position — the user stays where they scrolled. The pin SHALL re-attach
automatically as soon as the user scrolls back to the bottom (so the next
poll resumes auto-follow). A new assistant message that replaces the
current one (a different `Text` from the prior `m.latest`) SHALL always
force-attach the pin so the new content is visible.

#### Scenario: Auto-follow while at the bottom
- **WHEN** the viewport's `YOffset` is at the bottom of the rendered
  message and a poll brings appended text to the same message
- **THEN** the viewport scrolls so the new last line sits at the bottom
  of the viewport

#### Scenario: Scrolled-up user is not yanked back
- **WHEN** the user has scrolled the viewport up (away from the bottom)
  and a poll brings appended text to the same message
- **THEN** the viewport's `YOffset` is unchanged; the user remains at
  their scroll position and the new text accumulates off-screen below

#### Scenario: Return to bottom re-attaches the pin
- **WHEN** the user scrolls back to the bottom (e.g. via `End`) and a
  subsequent poll brings appended text to the same message
- **THEN** the viewport scrolls so the new last line sits at the bottom
  of the viewport

#### Scenario: New message always scrolls to bottom
- **WHEN** a poll surfaces an assistant message whose `Text` differs
  from `m.latest.Text`
- **THEN** the viewport scrolls to the bottom of the new message
  regardless of the previous scroll position

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

### Requirement: Always-visible help footer
The latest-message view SHALL render a one-line keymap footer at
the bottom of every state's view (`s` picker / nav / compose /
comment-composer / error). Pressing `?` SHALL expand the footer
into a multi-column full-help view; pressing `?` again SHALL
collapse it back. The footer SHALL be sourced from the bubbles
`help.Model` package and SHALL satisfy the `help.KeyMap` interface
with state-aware `ShortHelp()` and `FullHelp()` methods. The
viewport SHALL be shortened by the footer's height so the footer
never overlaps the message content.

#### Scenario: Nav view shows short help
- **WHEN** the TUI is in nav state
- **THEN** the bottom of the view contains a one-line keymap
  footer listing the most relevant keys for the current state

#### Scenario: ? expands to full help
- **WHEN** the user presses `?` in any state
- **THEN** the footer expands into a multi-column full-help view
  showing every keybinding for the current state, grouped by
  category; pressing `?` again collapses it back to the short
  footer

#### Scenario: Help footer reserves viewport space
- **WHEN** the footer is shown
- **THEN** the message viewport is shortened by the footer's
  height so the footer never overlaps the message content

#### Scenario: Compose and error states also show help
- **WHEN** the TUI is in compose / comment-composer / error /
  picker state
- **THEN** the help footer is still rendered at the bottom of the
  view with state-appropriate bindings

### Requirement: Manual refresh via `r` after attach

After the one-shot `Init()` poll has resolved, the system SHALL NOT
poll the watched agent session on a timer. The only way for the user
to surface new assistant text in the main view after attach SHALL be
by pressing `r` in `stateNav`. Each `r` press SHALL trigger exactly
one fetch from the session source. After that fetch resolves, the
system SHALL return to idle with no further `tea.Tick` scheduled.

The `Init()` poll SHALL fire within 500 ms of attach so the main view
is populated before the user can reasonably press any key. If the
`Init()` poll resolves with no assistant message, the main view SHALL
remain on its "waiting for agent…" placeholder until the user presses
`r` and a subsequent fetch surfaces a message.

A `r` press that resolves with the same assistant text as the
current `m.latest` (i.e. the session source has nothing new) SHALL be
a no-op: `m.latest` is unchanged, `m.comments` is preserved, and the
viewport position is preserved.

A `r` press that resolves with a different assistant text SHALL
replace `m.latest`, clear `m.comments` (so any annotations on the
prior message do not leak into the new one), and re-render the
viewport.

#### Scenario: `Init()` fires one warm-up poll
- **WHEN** pinky enters `stateNav` from attach
- **THEN** the system returns exactly one `pollCmd(m.src)` from
  `Init()` and does not schedule any further tick from the resulting
  `sessionMsg` handler

#### Scenario: `r` press is the only post-attach fetch trigger
- **WHEN** the user is in `stateNav` and the `Init()` poll has
  already resolved
- **THEN** pressing `r` returns exactly one `pollCmd(m.src)` from
  `handleNavKey`'s `ActionRefresh` branch, and the `sessionMsg`
  handler does not schedule a follow-up poll

#### Scenario: `r` with no new content is a true no-op
- **WHEN** the user presses `r` and the session source returns no
  new assistant messages
- **THEN** `m.latest.Text` is unchanged, `m.comments` retains every
  previously-saved annotation, and the viewport's `YOffset` is
  unchanged

#### Scenario: `r` with new content replaces `m.latest` and clears comments
- **WHEN** the user presses `r` and the session source returns a
  assistant message whose `Text` differs from `m.latest.Text`
- **THEN** `m.latest` is replaced with the new message,
  `m.comments` is set to `nil`, the viewport is re-rendered, and the
  viewport is pinned to the top (per "New message always scrolls to
  bottom")

#### Scenario: `r` while a session error is active
- **WHEN** the user presses `r` and the session source returns an
  error
- **THEN** the error is surfaced via the existing error-handling
  path and no follow-up poll is scheduled

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

