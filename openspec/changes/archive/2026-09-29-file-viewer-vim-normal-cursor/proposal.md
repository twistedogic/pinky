# Proposal

## Why

The file viewer's cursor model diverged from the message viewer's
anchor+cursor shape, and its visual mode is line-only with column-clamping
on `j`/`k` and no word-aware LSP. The result is that LSP queries
(`d`/`R`/`K`) require pixel-precise cursor positioning and the user has
no way to make a multi-line character selection — both of which make
code review clunky. Rewriting the file viewer's navigation on the
message viewer's vim-style model fixes both in one pass.

## What Changes

- **BREAKING** Drop the whole-file comment path: `c` with no visual
  selection now comments the **current line** instead of lines 1..N.
  Whole-file feedback still reaches the agent via compose.
- Refit `fileViewer` on the message viewer's anchor+cursor model:
  - `cursor (line, charPos)` is the single nav pointer, the LSP
    target, and the visual selection's tail.
  - `visual { Active, Mode='c', LineA, CharA }` stores only the
    anchor; the moving end is the cursor.
  - New `preferred` column tracker so `j`/`k` preserve the intended
    column instead of clamping to the new line's byte length.
- Rework the `stateFileView` key surface:
  - `h`/`j`/`k`/`l` always move the cursor (preferred column
    survives `j`/`k`).
  - `v` enters char visual at the cursor; `Esc` exits.
  - `c` opens the comment composer with the current visual range
    (or the current line when no visual is active).
  - `d`/`R`/`K` query LSP at the **word under the cursor** instead
    of at the exact `(line, charPos)`.
- Drop the implicit line-mode snap (`CharA=0`, `CharC=lineLen`) that
  `j`/`k` previously applied inside visual mode.
- Replace the green `▍` left-gutter cursor marker with an inline
  block cursor at `(cursor, charPos)` rendered with an inverted
  background. The yellow `▍` comment gutter remains; the cursor is
  the single inline indicator of the nav position.

## Capabilities

### New Capabilities

None. The new behavior lives entirely inside the existing
`workspace-files` capability.

### Modified Capabilities

- `workspace-files`: the `File viewer has a column cursor` and
  `File viewer keys` requirements are rewritten to match the
  vim-normal-cursor model above. The `LSP interactions are
  read-only` requirement gains the "word under cursor" semantics
  for `d`/`R`/`K`.

## Impact

- `internal/render/file_render.go` (and any file-viewer rendering
  helpers): selection highlight now derives from a single anchor
  rather than `(LineA,CharA,LineC,CharC)`.
- `model.go`:
  - `fileViewer` struct loses `LineC`/`CharC` fields, gains
    `preferred`.
  - `fileSelection` loses `LineC`/`CharC`, gains `Mode`.
  - `fileViewMoveLine` / `fileViewMoveRune` rewritten to update
    `preferred` and never snap `CharA`/`CharC` on `j`/`k`.
  - New `wordAtCursor` helper for `d`/`R`/`K`.
  - `openFileComment` chooses anchors based on `visual.Active` and
    line count of the selection (single-line → `file-inline`;
    multi-line → `file` line range, char ends dropped).
- Spec scenarios in `openspec/specs/workspace-files/spec.md` for
  `j`/`k` column behavior, visual mode `j`/`k` line-snap, `c`
  with no visual, and LSP position all need rewrites (handled in
  the delta spec).
- README "File viewer" key table and "File viewer scrolls" prose
  need updates to match the new surface (drop the line-range
  description on `j`/`k` in visual mode; describe `c` defaults
  to current line; describe `d`/`R`/`K` as word-under-cursor).
- Tests: `file_test.go`, `lspfileview_test.go`, and any test that
  drives the file viewer keys / comments / LSP will need updates
  in lockstep with the model change.
