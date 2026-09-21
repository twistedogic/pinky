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

### Requirement: Render assistant text as markdown
The system SHALL render the assistant message text as markdown
using `github.com/charmbracelet/glamour`, supporting headings,
bold and italic emphasis, inline code, fenced code blocks,
ordered and unordered lists, and blockquotes. Rendered output
SHALL be wrapped to the current viewport width. The glamour
`Margin` SHALL be `1` (one leading cell) so the document's left
indent combined with the focus gutter (see `### Requirement: Block
focus indicator`) totals two cells of left margin — the same visual
width the prior 2-cell-margin + border layout used.

#### Scenario: Headings render with weight-only heading style
- **WHEN** the assistant message contains a markdown heading
  (e.g., `## Step 1`)
- **THEN** the heading is rendered with the heading style
  described in `### Requirement: Heading weight without markdown
  markers` (cyan, bold, and for `h1` also underlined) and the
  rendered text does not begin with `#` characters

#### Scenario: Fenced code block renders as a block
- **WHEN** the assistant message contains a fenced code block
  (e.g., ` ```go ... ``` `)
- **THEN** the code block is rendered with a code-block style
  distinct from prose

#### Scenario: Long line wraps to viewport width
- **WHEN** the assistant message contains a line longer than the
  viewport width
- **THEN** the rendered output wraps the line at the viewport
  boundary rather than overflowing horizontally

### Requirement: Block focus indicator

The TUI SHALL indicate the currently-focused block by drawing a
left-margin gutter character `▍` in cyan (foreground colour `51`)
on every rendered line that falls inside that block's
`StartLine..EndLine` range. Lines outside any focused block SHALL
have a single space in that left-margin column. No horizontal
border lines (above or below the block) and no left vertical bar
across the block's content SHALL be drawn.

#### Scenario: Focused block carries cyan gutter
- **WHEN** the user navigates so block N becomes the focused
  block
- **THEN** every rendered line at indices `blocks[N].StartLine`
  through `blocks[N].EndLine` has a cyan `▍` in its leftmost
  column

#### Scenario: Non-focused lines have an empty gutter column
- **WHEN** the user is focused on block N and the viewport also
  shows lines that belong to block N-1 or block N+1
- **THEN** the lines belonging to N-1 and N+1 have a space in
  their leftmost column

#### Scenario: Gap lines between blocks have an empty gutter column
- **WHEN** the rendered output contains a blank line between two
  blocks (no block's `StartLine..EndLine` covers it)
- **THEN** that gap line has a space in its leftmost column

#### Scenario: Visual mode routes the gutter to the cursor's block
- **WHEN** the user is in visual line mode and the visual cursor
  is in block M while `viewport.YOffset` is in block N (M ≠ N)
- **THEN** the cyan `▍` follows block M (the visual cursor's
  block), not block N

### Requirement: Heading weight without markdown markers

The TUI SHALL render markdown headings using colour and weight
rather than the literal `#` characters from the markdown source.
The `Prefix` configuration passed to glamour for `h1`, `h2`, and
`h3` SHALL be the empty string. `h1` SHALL be rendered in cyan
(foreground colour `51`) with bold and underline modifiers; `h2`
SHALL be rendered in cyan (foreground `51`) with bold; `h3` SHALL
be rendered in a softer cyan (foreground colour `87`) with bold.
The rendered heading text SHALL NOT begin with one or more `#`
characters.

#### Scenario: h1 renders as cyan bold underlined with no `#` prefix
- **WHEN** the assistant message contains `# Step 1`
- **THEN** the rendered line shows the text `Step 1` (no `#`
  prefix) styled in cyan bold underlined

#### Scenario: h2 renders as cyan bold with no `##` prefix
- **WHEN** the assistant message contains `## Subtask`
- **THEN** the rendered line shows the text `Subtask` (no `##`
  prefix) styled in cyan bold

#### Scenario: h3 renders as soft-cyan bold with no `###` prefix
- **WHEN** the assistant message contains `### Detail`
- **THEN** the rendered line shows the text `Detail` (no `###`
  prefix) styled in a softer cyan bold

### Requirement: Build markdown block index
The system SHALL parse the assistant message text into a markdown AST and build an index of top-level non-empty blocks. Each indexed block SHALL record its kind (`heading`, `paragraph`, `code`, `list-item`, or `blockquote`) and its absolute start and end line indices in the rendered output. Empty blocks, thematic breaks, and HTML blocks SHALL be excluded from the index.

#### Scenario: Multi-block message produces one index entry per block
- **WHEN** the assistant message contains a heading followed by a paragraph followed by a code block
- **THEN** the block index contains three entries: one heading, one paragraph, one code block, in source order

#### Scenario: List items are indexed individually
- **WHEN** the assistant message contains an unordered list with five items
- **THEN** the block index contains five list-item entries, one per item

#### Scenario: Empty paragraph is excluded
- **WHEN** the assistant message contains an empty paragraph between two blocks
- **THEN** the empty paragraph is not in the block index

### Requirement: Vim-style navigation
The system SHALL provide vim-style navigation keys in the idle state. `j` SHALL move down one line, `k` SHALL move up one line, `}` SHALL jump to the first line of the next block, `{` SHALL jump to the first line of the previous block, `]]` SHALL jump to the first line of the next heading, `[[` SHALL jump to the first line of the previous heading, `gg` SHALL jump to the top of the message, and `G` SHALL jump to the bottom of the message. The two-key sequences `gg`, `]]`, and `[[` SHALL be detected via a state field with a 500ms timeout.

#### Scenario: j moves down one line
- **WHEN** the user presses `j` in idle state
- **THEN** the viewport scrolls down by one line

#### Scenario: k moves up one line
- **WHEN** the user presses `k` in idle state
- **THEN** the viewport scrolls up by one line

#### Scenario: } jumps to next block
- **WHEN** the user presses `}` in idle state and the current block is not the last
- **THEN** the viewport scrolls so the first line of the next block is at the top of the viewport

#### Scenario: { jumps to previous block
- **WHEN** the user presses `{` in idle state and the current block is not the first
- **THEN** the viewport scrolls so the first line of the previous block is at the top of the viewport

#### Scenario: ]] jumps to next heading
- **WHEN** the user presses `]]` in idle state and there is a heading after the current one
- **THEN** the viewport scrolls so the first line of the next heading block is at the top of the viewport

#### Scenario: [[ jumps to previous heading
- **WHEN** the user presses `[[` in idle state and there is a heading before the current one
- **THEN** the viewport scrolls so the first line of the previous heading block is at the top of the viewport

#### Scenario: gg jumps to top
- **WHEN** the user presses `g` then `g` within 500ms in idle state
- **THEN** the viewport scrolls to the top of the message

#### Scenario: G jumps to bottom
- **WHEN** the user presses `G` in idle state
- **THEN** the viewport scrolls to the bottom of the message

#### Scenario: Two-key timeout clears state
- **WHEN** the user presses `g` and more than 500ms elapse before another key
- **THEN** the two-key state is cleared

#### Scenario: Non-matching second key clears state
- **WHEN** the user presses `g` followed by any key other than `g` within 500ms
- **THEN** the two-key state is cleared and the second key is processed normally

### Requirement: Arrow and page keys alias vim keys
The system SHALL keep arrow keys and `PgUp`/`PgDn` working as aliases for the vim-style keys so existing muscle memory does not break. `↑` SHALL behave as `k`, `↓` SHALL behave as `j`, `PgUp` SHALL jump to the top of the current message, and `PgDn` SHALL behave as `}` (jump to next block).

#### Scenario: Arrow down moves one line
- **WHEN** the user presses `↓` in idle state
- **THEN** the viewport scrolls down by one line

#### Scenario: Arrow up moves one line
- **WHEN** the user presses `↑` in idle state
- **THEN** the viewport scrolls up by one line

#### Scenario: PgUp jumps to top of message
- **WHEN** the user presses `PgUp` in idle state
- **THEN** the viewport scrolls to the top of the current message

#### Scenario: PgDn jumps to next block
- **WHEN** the user presses `PgDn` in idle state
- **THEN** the viewport scrolls to the first line of the next block

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
- A background color tint covering the rendered line range of each
  commented block.

The annotation overlay SHALL NOT modify the assistant message text
itself; the markdown rendering of the assistant message is
unchanged. The overlay is composed after markdown rendering as a
post-processing pass on the rendered line ranges, using the block
index from "Build markdown block index" as the line-range source.

#### Scenario: Comented block shows footnote below
- **WHEN** a saved block-level comment exists for a block in the
  rendered message
- **THEN** the main view shows the rendered block, immediately
  followed by a footnote line containing the marker and the comment
  text, before the next block's first line

#### Scenario: Annotation overlay survives width reflow
- **WHEN** the terminal width changes and the rendered message is
  re-flowed
- **THEN** the comment annotations (footnotes, background tints,
  and the gutter flag carried on `Block.HasComment`) are re-applied
  to the new line ranges without loss; no annotation references a
  stale line index from the previous width

### Requirement: Always-visible help footer
The latest-message view SHALL render a one-line keymap footer at
the bottom of every state's view (`s` picker / idle / compose /
comment-composer / error). Pressing `?` SHALL expand the footer
into a multi-column full-help view; pressing `?` again SHALL
collapse it back. The footer SHALL be sourced from the bubbles
`help.Model` package and SHALL satisfy the `help.KeyMap` interface
with state-aware `ShortHelp()` and `FullHelp()` methods. The
viewport SHALL be shortened by the footer's height (1 line for
short, N lines for the largest full-help group) so the footer
never overlaps the message content.

#### Scenario: Idle view shows short help
- **WHEN** the TUI is in idle state
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
  height (1 line for short, N lines for the largest full-help
  group) so the footer never overlaps the message content

#### Scenario: Compose and error states also show help
- **WHEN** the TUI is in compose / comment-composer / error /
  picker state
- **THEN** the help footer is still rendered at the bottom of the
  view with state-appropriate bindings
