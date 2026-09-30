# Make the latest-message comment navigation match the file viewer

## Why

The latest-message view (`stateNav`) and the file viewer
(`stateFileView`) were built to do the same job — let the user move
a cursor over text and stage byte-anchored comments — but their
navigation models diverged. The file viewer has a 1-based line
cursor with a preferred-column tracker: `j`/`k` move one source
line, `h`/`l` move one rune, the column survives shorter lines on
return. The message view has a *block* cursor: `j`/`k` jump to the
next/previous markdown block and `charPos` snaps to 0 on every
jump.

The practical consequence is that picking a phrase inside a long
paragraph for an inline comment requires only `h`/`l` — `j`/`k`
fling the cursor to the next block instead of advancing by one
line. The file viewer already solves this; the message view should
adopt the same model so the two views feel like one tool.

A second motivation: comments on a message are anchored by quoted
text, not by file position, so the appendix format should drop
line numbers for message-kind comments and split the appendix so
the agent can read message-comments and file-comments as
distinct groups.

## What Changes

- **Cursor shape in `stateNav` becomes `(lineIdx, charPos,
  preferred)`, source-line basis.** `j`/`k` move by source line
  (1-based), `charPos` follows the `min(preferred, len(newLine))`
  rule, `preferred` is the rightmost column reached by `l` and
  survives `j`/`k`. Same shape as the file viewer's
  `fileViewer{cursor, charPos, preferred}`. **BREAKING**: the
  `blockIdx` field on `NavCursor` and `NavSelection` is removed.
- **Inline block cursor at `charPos` replaces the cyan `▍`
  focus gutter.** The cursor paints a single inverted-block cell
  at the byte it lands on, exactly as the file viewer does. The
  `latest-message-view` requirement that draws a per-line cyan
  gutter across the focused block is removed.
- **Markdown block index is gone.** `m.blocks`, the markdown AST
  parser, and the `Block`/`BlockKind` types are removed — they
  were load-bearing only for navigation and block-derived
  rendering, both of which are replaced. **BREAKING**: any code
  that reads `m.blocks` or `cursor.BlockIdx` must change.
- **`c` in `stateNav` (no visual or empty visual) anchors the
  new comment to the whole current source line** — `(byteStart,
  byteEnd)` covering every byte on that line in `m.latest.Text`.
  `c` in visual mode (non-empty range) anchors to the visual
  byte range. The block-level / inline discriminator for
  *message* comments is removed; message comments have a single
  shape. **BREAKING**: `Comment.BlockIdx` and the
  `CharStart == CharEnd == -1` whole-block sentinel are removed.
- **Saved comments render with a line-direct yellow `▍` gutter,
  keyed off source lines, not blocks.** Every source line whose
  byte range overlaps any saved comment carries the yellow `▍`
  in the left-margin column. The previous per-block yellow
  gutter (and the footnote-below-block render) is removed.
- **Scroll keys `↑`/`↓`/`PageUp`/`PageDown`/`Home`/`End` in
  `stateNav` move the viewport only**, not the cursor. The
  cursor and viewport are now distinct: `j`/`k` move the cursor,
  the viewport follows with a 1-line cushion; `↑`/`↓` move the
  viewport, the cursor stays. Matches the file viewer's
  separation. **BREAKING**: today these keys are unbound in
  `stateNav`; binding them is a UX change for existing muscle
  memory (e.g. `↓` no longer moves the cursor).
- **Comments appendix format changes.** The leading count line
  is dropped. The body is split into two labelled sections:
  `Comments on the message:` followed by message-kind entries,
  a blank line, then `Comments on files:` followed by file-kind
  entries. Each message-kind entry renders as
  `- comment on "<excerpt>": <text>` with no line position. The
  excerpt is the verbatim text of the commented byte range,
  truncated to 40 chars with `…`, with internal newlines
  replaced by single spaces and runs of whitespace collapsed.
  File-kind entries keep today's format (`- file "<path>"
  (lines X-Y): <text>` / `- file-inline "<excerpt>" (line Z):
  <text>`). No leading `---` separator (existing behaviour).
  **BREAKING**: the agent's received payload shape changes.

## Capabilities

### Modified Capabilities

- `latest-message-view`: the markdown-AST block-index requirement
  is removed; the cyan `▍` focus-gutter requirement is removed;
  the focused-block semantics (driven by `cursor.blockIdx`) is
  replaced by inline-block-cursor-at-charPos semantics driven by
  `(lineIdx, charPos)`. The "raw markdown source" rendering rule
  is unchanged.
- `single-cursor-nav`: the `NavCursor` definition changes from
  `(blockIdx, charPos)` to `(lineIdx, charPos, preferred)`;
  `j`/`k` semantics change from "next/previous block" to
  "next/previous source line with preferred-column"; the
  `single source of truth` requirement's "which block is
  focused" clause becomes "which source line the cursor sits
  on" (or is dropped if redundant with the new definition); the
  scroll-key separation between cursor and viewport is added.
- `message-comments`: the block-level comment kind and its
  whole-block sentinel are removed; the yellow `▍` per-block
  gutter is replaced by a line-direct yellow gutter driven by
  byte-range overlap with source lines; the footnote-below-block
  rendering is removed; the appendix format is replaced by the
  two-section `comment on "<excerpt>"` format; the excerpt
  truncation / whitespace rules above are added.

### New Capabilities

- _None._ Behaviour changes are all inside existing
  capabilities; no new capability is introduced.

## Impact

- **Model / nav layer** — `model.go`, `internal/render/nav.go`,
  `internal/render/anchor.go`: the `NavCursor` / `NavSelection`
  types are reshaped; `NavHandle` is rewritten for line-based
  motion with the preferred-column rule; the `commentAnchor`
  struct loses `blockIdx`/`charA`/`charC` in favour of
  `(byteA, byteB)`; the keymap gains the five scroll keys.
- **Markdown / block rendering** — `internal/render/render.go`,
  `internal/render/comment_render.go`: the `Block` / `BlockKind`
  types and the markdown-AST parser are removed;
  `RenderMessageWithComments` is replaced by a line-direct
  gutter that walks saved comments and walks source lines;
  `InjectGutterWrapped`'s block-idx argument drops; the
  `HasComment` flag is replaced by a per-line overlap test.
- **Comments appendix** —
  `internal/render/comment_appendix.go`: `FormatCommentsAppendix`
  is split into two formatters joined by an orchestrator;
  `render.Comment.BlockIdx`, `CharStart`, `CharEnd` are dropped
  for the message kind; the `IsInlineSelection` helper and the
  `CommentBlock` constant are removed (or `CommentBlock` is
  renamed `CommentMessage` for kind-name clarity).
- **Keymap / help** — `keymap.go`: the stateNav keymap gains
  scroll keys; the help page text is updated.
- **Tests** — `internal/render/nav_test.go`,
  `internal/render/render_test.go`,
  `internal/render/comment_render_test.go`,
  `internal/render/gutter_test.go`,
  `internal/render/comment_appendix_test.go`, `model_test.go`,
  `lspfileview_test.go`, `view_test.go`, `file_test.go` (any test
  that references `blockIdx`, `BlockIdx`, or the block-derived
  rendering needs to be rewritten against the line-based model).
- **User-facing docs** — `README.md`: nav keymap table gains the
  scroll keys; the comment-appendix example block is updated.