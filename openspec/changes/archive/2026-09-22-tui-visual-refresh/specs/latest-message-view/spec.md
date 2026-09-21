## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Render assistant text as markdown

The system SHALL render the assistant message text as markdown
using `github.com/charmbracelet/glamour`, supporting headings,
bold and italic emphasis, inline code, fenced code blocks,
ordered and unordered lists, and blockquotes. Rendered output
SHALL be wrapped to the current viewport width. The glamour
`Margin` SHALL be `1` (one leading cell) so the document's left
indent combined with the focus gutter (Section 2) totals two
cells of left margin — the same visual width the prior
2-cell-margin + border layout used.

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