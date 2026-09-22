# Tasks: render-gfm-tables

## 1. Failing regression tests first (per AGENTS.md)

- [x] 1.1 Add `TestRender_TableCellsAppearInOutput` to
      `internal/render/render_test.go`. Build a markdown string
      with a heading, a GFM table (header + alignment + 2 body
      rows), and a trailing paragraph. Assert `stripANSI(rendered)`
      contains every cell text and at least one pipe-shaped or
      alignment character. Verify the test FAILS against the
      current code (table silently dropped).
- [x] 1.2 Add `TestExtract_TableRecognized` to
      `internal/render/render_test.go`. Parse a markdown string
      containing only a GFM table and assert
      `len(blocks) == 1 && blocks[0].Kind == BlockTable` once
      `BlockTable` is added (the test guards against future
      regressions). This test may be a single combined
      `extract_test.go` for the new cases.
- [x] 1.3 Add `TestRender_TableBetweenBlocks` to
      `internal/render/render_test.go`. Heading + table +
      paragraph; assert three blocks, contiguity holds, and the
      table block is in source position 1. Guards the "table sits
      cleanly between blocks" spec scenario.
- [x] 1.4 (Optional, if def lists are in scope) Add the same
      trio of tests for definition lists. Block kind constant:
      `BlockDefList`.

## 2. Implementation

- [x] 2.1 Add `BlockTable BlockKind = "table"` (and optionally
      `BlockDefList BlockKind = "definition-list"`) to the
      `BlockKind` const block in `internal/render/render.go`.
- [x] 2.2 Add `case *ast.Table:` to the `extract()` type switch
      in `internal/render/render.go`. Use `blockText(n, src)` to
      obtain source (same pattern as Heading/Paragraph/etc.),
      short-circuit on whitespace-only source, return
      `[]extracted{{kind: BlockTable, source: s}}`.
- [x] 2.3 (Optional) Add `case *ast.DefinitionList:` to the same
      switch, returning one block per whole list.
- [x] 2.4 Run the regression tests from §1 — confirm they pass
      against the patched code. Run the full suite
      (`task test`) — confirm nothing else regressed.

## 3. Spec updates (closes the contract gap)

- [x] 3.1 Update `openspec/specs/latest-message-view/spec.md`,
      requirement "Render assistant text as markdown" — append
      `GFM tables` and `GFM definition lists` to the feature
      list. The MODIFIED block in this change's
      `specs/latest-message-view/spec.md` is the source of truth;
      when archived, merge it into the main spec.
- [x] 3.2 Update the same spec file, requirement "Build markdown
      block index" — add `table` and `definition-list` to the
      kind enum, and append the two new scenarios (`GFM table is
      indexed as one block`, `GFM definition list is indexed as
      one block`, `Table sits cleanly between adjacent blocks`).
- [x] 3.3 (Optional) Run `openspec validate
      render-gfm-tables` — confirm the delta spec parses
      cleanly.

## 4. Verification

- [x] 4.1 Run `task check` (vet + tests). Green required.
- [x] 4.2 Visual smoke test: rendered a markdown document with
      a heading + 2x2 GFM table + trailing paragraph through
      glamour. Output shows readable table with `│` column
      separators and `──────┼────` alignment row. Cells visible,
      alignment preserved. (Standalone glamour render; pinky uses
      the same glamour setup so the result is equivalent on the
      production path.)
- [x] 4.3 Glamour's default table style is acceptable. `pinkyStyle()`
      does not define a `Table` style — glamour falls back to
      its default, which renders cleanly. No follow-up change
      needed unless a maintainer later wants pinky-branded
      table styling.