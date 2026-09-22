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
user are the bytes the agent produced, sliced at block boundaries.

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

### Requirement: Block focus indicator

The TUI SHALL indicate the currently-focused block by drawing a
left-margin gutter character `▍` in cyan (foreground colour `51`)
on every rendered line that falls inside the focused block's
`StartLine..EndLine` range. Lines outside any focused block SHALL
have a single space in that left-margin column. No horizontal
border lines (above or below the block) and no left vertical bar
across the block's content SHALL be drawn.

The focused block is the block at the cursor's `blockIdx` (see
the `single-cursor-nav` capability). The cursor is the single
source of truth — visual mode and idle mode both read the same
cursor.

#### Scenario: Focused block carries cyan gutter
- **WHEN** the cursor's `blockIdx` is `N`
- **THEN** every rendered line at indices `blocks[N].StartLine`
  through `blocks[N].EndLine` has a cyan `▍` in its leftmost
  column

#### Scenario: Non-focused lines have an empty gutter column
- **WHEN** the cursor is on block N and the viewport also shows
  lines that belong to block N-1 or block N+1
- **THEN** the lines belonging to N-1 and N+1 have a space in
  their leftmost column

#### Scenario: Gap lines between blocks have an empty gutter column
- **WHEN** the rendered output contains a blank line between two
  blocks (no block's `StartLine..EndLine` covers it)
- **THEN** that gap line has a space in its leftmost column

#### Scenario: Visual mode routes the gutter to the cursor's block
- **WHEN** the user is in visual line mode and the cursor is in
  block M (cursor is the source of truth in both modes)
- **THEN** the cyan `▍` follows block M, regardless of the
  viewport's `YOffset`

### Requirement: Build markdown block index
The system SHALL parse the assistant message text into a markdown AST and build an index of top-level non-empty blocks. Each indexed block SHALL record its kind (`heading`, `paragraph`, `code`, `list-item`, `blockquote`, `table`, or `definition-list`) and its absolute start and end line indices in the rendered output. Empty blocks, thematic breaks, and HTML blocks SHALL be excluded from the index. A GFM table SHALL be indexed as a single block (not one block per row). A GFM definition list SHALL be indexed as a single block.

#### Scenario: Multi-block message produces one index entry per block
- **WHEN** the assistant message contains a heading followed by a paragraph followed by a code block
- **THEN** the block index contains three entries: one heading, one paragraph, one code block, in source order

#### Scenario: List items are indexed individually
- **WHEN** the assistant message contains an unordered list with five items
- **THEN** the block index contains five list-item entries, one per item

#### Scenario: Empty paragraph is excluded
- **WHEN** the assistant message contains an empty paragraph between two blocks
- **THEN** the empty paragraph is not in the block index

#### Scenario: GFM table is indexed as one block
- **WHEN** the assistant message contains a GFM table with a
  header row, an alignment row, and two body rows
- **THEN** the block index contains exactly one `table` entry
  whose `StartLine..EndLine` covers every rendered line of the
  table (header, alignment, and body rows)

#### Scenario: GFM definition list is indexed as one block
- **WHEN** the assistant message contains a GFM definition list
  with two term-description pairs
- **THEN** the block index contains exactly one
  `definition-list` entry whose `StartLine..EndLine` covers
  every rendered line of the list

#### Scenario: Table sits cleanly between adjacent blocks
- **WHEN** the assistant message contains a heading, then a
  GFM table, then a paragraph
- **THEN** the block index contains three entries (heading,
  table, paragraph) with contiguous `EndLine + 1 == next
  StartLine` line ranges — no gap or overlap

### Requirement: Single-letter nav surface in stateNav

In `stateNav` (formerly `stateIdle`) the system SHALL provide a
single-letter nav surface. The bindings and actions are:

| key  | action |
|------|--------|
| `j`  | move cursor to next block |
| `k`  | move cursor to previous block |
| `h`  | move cursor one rune left within the current block |
| `l`  | move cursor one rune right within the current block |
| `v`  | enter visual mode (toggle); re-press while visual exits |
| `Esc`| exit visual mode (no-op when not in visual) |
| `c`  | open comment composer (selection-anchored if visual, block-anchored otherwise) |
| `s`  | send to agent (idle batch when comments exist; compose otherwise) |
| `n`  | enter compose mode |
| `r`  | refresh the tailed session and re-render the latest message |
| `q`  | quit pinky |
| `?`  | toggle short / full help overlay |

The previous two-key state machine (`gg`, `]]`, `[[`) and the
`Ctrl+C` / `Ctrl+N` / `Ctrl+R` / `Ctrl+S` / `Ctrl+I` bindings
SHALL NOT exist in `stateNav`.

#### Scenario: `j` moves to the next block
- **WHEN** the user presses `j` in `stateNav`
- **THEN** the cursor's `blockIdx` increments by one (clamped at
  the last block) and the viewport scrolls to keep the cursor
  visible

#### Scenario: `k` moves to the previous block
- **WHEN** the user presses `k` in `stateNav`
- **THEN** the cursor's `blockIdx` decrements by one (clamped at
  the first block) and the viewport scrolls to keep the cursor
  visible

#### Scenario: `h` moves one rune left
- **WHEN** the user presses `h` in `stateNav` and the cursor's
  `charPos` is greater than `0`
- **THEN** the cursor's `charPos` decreases by the byte length of
  the rune preceding it

#### Scenario: `l` moves one rune right
- **WHEN** the user presses `l` in `stateNav` and the cursor's
  `charPos` is less than `len(block.Source)`
- **THEN** the cursor's `charPos` increases by the byte length of
  the rune at the current offset

#### Scenario: Two-key sequences do not exist
- **WHEN** the user presses `g` followed by `g` in `stateNav`
- **THEN** the first `g` is processed as an unknown rune (no
  state, no timeout); the second `g` is processed as an unknown
  rune. No `gg` action fires.

### Requirement: Yank to bottom on new content
The system SHALL scroll the viewport to the bottom of the message each time new content is appended to the current message, so the user sees the latest text without manual scrolling.

#### Scenario: Streaming keeps viewport at bottom
- **WHEN** new assistant text is appended to the current message
- **THEN** the viewport scrolls so the last line of the rendered message is at the bottom of the viewport

### Requirement: Render comment annotations as overlay
The latest-message view SHALL be capable of rendering comment
annotations on top of the rendered assistant message. The
annotation overlay consists of:

- A footnote line for each saved comment, rendered immediately
  below the block it annotates and before the next block begins.

The annotation overlay SHALL NOT modify the assistant message text
itself; the assistant message is shown verbatim and the overlay
sits on top of it. The yellow left-gutter indicator on every line
inside a commented block's `StartLine..EndLine` range (defined in
`message-comments`) is the only visual signal distinguishing a
commented block from an uncommented one.

#### Scenario: Commented block shows footnote below
- **WHEN** a saved block-level comment exists for a block in the
  rendered message
- **THEN** the main view shows the rendered block, immediately
  followed by a footnote line containing the marker and the comment
  text, before the next block's first line

#### Scenario: Annotation overlay survives width reflow
- **WHEN** the terminal width changes and the rendered message is
  re-flowed
- **THEN** the comment annotations (footnotes and the gutter flag
  carried on `Block.HasComment`) are re-applied to the new line
  ranges without loss; no annotation references a stale line index
  from the previous width

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
