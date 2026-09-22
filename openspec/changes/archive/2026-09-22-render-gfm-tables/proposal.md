# Proposal: render-gfm-tables

## Why

Markdown tables in agent messages are silently dropped from the
latest-message view. The goldmark GFM extension that recognises
tables is enabled in `internal/render/render.go` (line 104), so the
parser produces `*ast.Table` nodes — but the block-extraction function
`extract()` has no `case *ast.Table` in its type switch. The whole
table falls through to `return nil` and vanishes from the rendered
view with no error and no warning. I verified this end-to-end with a
temp test against the real `renderBlocks`: the table source
(`| col1 | col2 | ...`) is never seen by glamour, and the cell text
(`col1`, `a`, `b`, `c`, `d`) appears nowhere in the output.

The same root cause silently drops GFM definition lists
(`extension.DefinitionList` is enabled but `*ast.DefinitionList` is
unhandled). Definition lists are less common in agent output but
fixing them in the same change costs ~6 lines.

This is a pure bug fix: the spec already claims pinky renders GFM
markdown; tables are a documented GFM construct; the implementation
just forgot to handle the AST node.

## What Changes

- Add `BlockTable BlockKind = "table"` to the `BlockKind` constants
  in `internal/render/render.go`.
- Add `case *ast.Table:` to `extract()` so `*ast.Table` AST nodes
  produce a block whose `Source` is the table's raw markdown (header
  + alignment row + body rows). Glamour's renderer re-parses this
  source with its own goldmark+GFM setup, so it renders correctly.
- Add `BlockDefList BlockKind = "definition-list"` and a
  `case *ast.DefinitionList:` in the same `switch`, for the
  same-shape fix on definition lists. Single block per whole list
  (matches glamour's render expectations, same reasoning as tables).
- Regression tests proving:
  - Table cell text appears in the rendered output.
  - Table block sits cleanly between an adjacent heading and
    paragraph (block-index boundaries stay contiguous).
  - `extract()` returns one block per `*ast.Table`, with
    `Kind == BlockTable`.
  - (Optional) same trio for definition lists.
- Spec updates to `latest-message-view`:
  - "Render assistant text as markdown" — add `tables` to the list
    of GFM constructs pinky renders.
  - "Build markdown block index" — add `table` to the `kind` enum
    and add a `Scenario: Markdown table is indexed as one block`
    requirement.

No new top-level capabilities — this closes a gap in an existing
one.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `latest-message-view`: requirements "Render assistant text as
  markdown" and "Build markdown block index" updated to cover
  GFM tables (and definition lists, which the GFM+DefinitionList
  extensions already parse).

## Impact

- `internal/render/render.go` — 1 new constant, 1 new switch case
  (~6 lines). Plus a second case for definition lists if scope
  includes them.
- `internal/render/render_test.go` — 3–4 new test cases.
- `openspec/specs/latest-message-view/spec.md` — 2 requirement
  edits, 1 new scenario.
- No changes to the model layer, the nav state machine, the
  comment rendering, or the inject pipeline. All of those
  consume `Block.Kind` opaquely (or not at all) — verified by
  searching the codebase for `BlockKind` consumers. Tables
  behave like any other block as far as navigation, commenting,
  and gutter rendering are concerned.
- No new dependencies; no API surface change.
- Backwards compatible: existing markdown without tables
  renders identically. Existing tests continue to pass.