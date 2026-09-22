## MODIFIED Requirements

### Requirement: Render assistant text as markdown
The system SHALL render the assistant message text as markdown
using `github.com/charmbracelet/glamour`, supporting headings,
bold and italic emphasis, inline code, fenced code blocks,
ordered and unordered lists, blockquotes, GFM tables, and GFM
definition lists. Rendered output SHALL be wrapped to the
current viewport width. The glamour `Margin` SHALL be `1` (one
leading cell) so the document's left indent combined with the
focus gutter (see `### Requirement: Block focus indicator`)
totals two cells of left margin — the same visual width the
prior 2-cell-margin + border layout used.

#### Scenario: Headings render with weight-only heading style
- **WHEN** the assistant message contains a markdown heading
  (e.g., `## Step 1`)
- **THEN** the heading is rendered with the heading style
  described in `### Requirement: Heading weight without markdown
  markers` (cyan, bold, and for `h1` also underlined) and the
  rendered text does not begin with `#` characters

#### Scenario: Fenced code block renders as a block
- **WHEN** the assistant message contains a fenced code block
  (e.g., ``` ```go ... ``` ```)
- **THEN** the code block is rendered with a code-block style
  distinct from prose

#### Scenario: Long line wraps to viewport width
- **WHEN** the assistant message contains a line longer than the
  viewport width
- **THEN** the rendered output wraps the line at the viewport
  boundary rather than overflowing horizontally

#### Scenario: GFM table renders as a table
- **WHEN** the assistant message contains a GFM-flavoured
  markdown table (a header row followed by an alignment row
  of `---` separators, followed by one or more body rows)
- **THEN** the table renders in the main view as a single block
  with column alignment preserved and every cell's text visible
  in the output

#### Scenario: GFM definition list renders as a block
- **WHEN** the assistant message contains a GFM definition list
  (one or more `term\n:   description` pairs)
- **THEN** the definition list renders in the main view as a
  single block with every term and description visible in the
  output

### Requirement: Build markdown block index
The system SHALL parse the assistant message text into a markdown AST and build an index of top-level non-empty blocks. Each indexed block SHALL record its kind (`heading`, `paragraph`, `code`, `list-item`, `blockquote`, `table`, or `definition-list`) and its absolute start and end line indices in the rendered output. Empty blocks, thematic breaks, and HTML blocks SHALL be excluded from the index. A GFM table SHALL be indexed as a single block (not one block per row) so that glamour's column alignment is preserved in the rendered view. A GFM definition list SHALL be indexed as a single block.

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