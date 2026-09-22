# Design: render-gfm-tables

## Context

Pinky renders markdown agent messages via `internal/render/render.go`.
`renderBlocks(md, width)` is the single rendering entry point: it
parses the message with goldmark, walks top-level AST nodes, and
for each one calls `extract(node, src)` to produce a `[]Block` index
plus the rendered string.

The parser is configured with three extensions:

```go
extension.GFM,           // tables, strikethrough, task lists, autolinks
extension.Linkify,       // auto-detect URLs
extension.DefinitionList // GFM-style definition lists
```

GFM tables produce `*ast.Table` AST nodes; definition lists produce
`*ast.DefinitionList`. Both are silently discarded by the current
`extract()` switch, which only handles `*ast.Heading`,
`*ast.Paragraph`, `*ast.FencedCodeBlock` / `*ast.CodeBlock`,
`*ast.Bblockquote`, and `*ast.List`.

End-to-end repro (verified with a temp test against the real
`renderBlocks`):

```markdown
# heading

| col1 | col2 |
|------|------|
| a    | b    |
| c    | d    |

trailing paragraph
```

produces 2 blocks (`heading`, `paragraph`); the table is gone.
None of `col1`, `a`, `b`, `c`, `d` appear in the rendered output.

The fix is local to `extract()`. No downstream consumer of
`Block.Kind` (nav, comments, gutter, dispatch) reads the kind
opaquely — confirmed by `rg "BlockKind|\\.Kind "`.

## Goals / Non-Goals

**Goals:**

- GFM tables in agent messages render as one block per table, with
  cell text visible and column alignment preserved.
- GFM definition lists render as one block per list, with the
  term-description pairs visible.
- Both produce a `BlockKind` so navigation (`j`/`k`), commenting,
  gutter highlighting, and the footnote-rendering overlay work
  without further changes.
- Failing test first, then fix, per AGENTS.md.
- Spec updates so the change is documented at the contract level.

**Non-Goals:**

- Per-row block indexing for tables (would break glamour's column
  alignment; tables aren't navigable as rows in any reasonable UX).
- Per-term block indexing for definition lists (same; glamour
  expects the whole list).
- Inline navigation within a table cell. Cursor moves
  block-by-block, like every other block.
- New markdown features (footnotes, math, etc.).
- Style changes — pinky's `pinkyStyle()` already covers tables
  via GFM defaults; visual quality is whatever glamour produces.
  If it looks bad in production we revisit.

## Decisions

### D1: One block per table (not one per row)

Glamour renders the whole table as one styled unit with column
alignment. Passing it a single row source breaks alignment — glamour
emits a one-row "header-less" table with no body. A table row is
not meaningful without column context.

`Block.StartLine..EndLine` covers the whole table's rendered
lines. `Comment.Marker()` uses the `CharStart < 0` sentinel for
block-level comments, which works regardless of `Kind`. No changes
needed downstream.

### D2: Use `blockText(n, src)` to extract source

`blockText` already exists and is the same helper used for
Heading, Paragraph, CodeBlock, Blockquote. It calls `node.Lines()`
and joins segments. For `*ast.Table` and `*ast.DefinitionList` it
returns the raw markdown source verbatim (header + alignment row +
body rows for tables; terms + descriptions for def lists).

That source string is what glamour's renderer expects — glamour
runs its own goldmark+GFM parse internally. Passing the original
markdown through is the same pattern used for every other block.

### D3: Add `BlockTable` and `BlockDefList` constants

The `BlockKind` const is the public type other packages
(intentionally only `model`) switch on. Adding values is
non-breaking — existing switches (none found in the codebase
beyond `extract` itself and tests) would hit the default case.

### D4: Definition lists are in scope

The root cause is identical to tables; the fix is identical
(one new constant + one new switch case). Leaving definition
lists broken would be a known bug with a one-line fix. Including
them in this change avoids a second PR for the same root cause.

If the user wants to scope down, drop the def list changes —
proposal and tasks remain coherent. Spec update touches both
tables and definition lists either way (they're listed together
in the parser extensions block).

### D5: No changes to downstream code

Verified by searching the codebase:
- `rg "BlockKind"` — only declaration + tests
- `rg "\\.Kind "` — only Block.Kind assignment in tests
- `Comment.Kind` was deleted in the recent cleanup
- `NavHandle` doesn't switch on Kind
- `InjectGutter` doesn't switch on Kind
- `footnoteLines`, `applyHighlights` don't switch on Kind
- `FormatCommentsAppendix` uses `c.CharStart` only

So the change is mechanical:
1. Add constant.
2. Add switch case.
3. Test.

### D6: Spec changes are minimal

Two requirements in `latest-message-view` need editing:
- "Render assistant text as markdown" — append `tables` and
  `definition lists` to the feature list.
- "Build markdown block index" — append `table` and
  `definition-list` to the kind enum. Add one scenario for
  tables.

`message-comments`, `single-cursor-nav`, `agent-redirect` are
not touched — they describe behavior orthogonal to block-kind.

## Risks / Trade-offs

[Risk] Glamour's table output looks bad with pinky's StyleConfig
→ Mitigation: visual smoke test as part of tasks; if unacceptable,
add `Table` block to `pinkyStyle()` (extends an existing StyleConfig,
not a new architecture).

[Risk] Wide tables overflow the viewport horizontally
→ Mitigation: glamour's `WordWrap(width+1)` already handles this;
the 1-line wrap drift in `RenderedLineRange` is an accepted
ponytail debt that affects all blocks equally.

[Risk] Yellow gutter + background tint renders oddly inside table cells
→ Mitigation: the gutter is a one-char prepend; the tint is a
one-line wrap. Both are line-scoped, not block-scoped. If it
looks bad we adjust, but the contract doesn't change.

[Risk] Definition lists without a leading blank line parse as paragraphs
in goldmark
→ Mitigation: goldmark's DefinitionList extension requires the
standard `Term\n: def` shape; pinky doesn't need to teach users
markdown syntax. If a def list parses as a paragraph, the user
sees a paragraph (existing behavior), not a crash.

[Risk] The "Build markdown block index" requirement says "Empty
blocks, thematic breaks, and HTML blocks SHALL be excluded." —
amending it to include tables could be misread as removing
that exclusion.
→ Mitigation: add the new `table` kind to the list, keep the
existing exclusion language verbatim.

## Migration Plan

None. This is a bug fix in the rendering path; no API, schema,
or config surface changes. Deploy = `go install` or rebuild.

Rollback = revert the switch case + constant. The spec change
can stay (it documents the intended behavior — reverting would
re-introduce the gap).

## Open Questions

- **None blocking.** The fix is mechanical. If glamour's table
  output is visually unacceptable, that's a follow-up style
  change, not a blocker for this change.

  If during implementation we discover that pinkyStyle's
  StyleConfig doesn't include a Table style, glamour falls back
  to its default table style (works, may look off-brand).
  Decision: accept the default, log a follow-up if it looks bad.