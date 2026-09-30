# Design

## Context

The latest-message view (`stateNav`) and the file viewer
(`stateFileView`) share a job — let the user navigate a text body
and stage byte-anchored comments — but use different cursor
models. The file viewer's cursor is `(cursor, charPos, preferred)`
over 1-based source lines with a preferred-column tracker; the
message viewer's cursor is `(blockIdx, charPos)` over a markdown
block index with no column memory. `j`/`k` in the message view
jump block-to-block and snap `charPos` to `0`, which makes
fine-picking an inline byte range inside a long paragraph
impossible with `j`/`k`. This change aligns the message view
with the file viewer's mechanics so both views feel like one
tool. See `proposal.md` for motivation and the user-facing
`specs/` deltas for what the new behaviour looks like.

The current message view carries an entire markdown AST index
(`m.blocks`) that exists only to drive: navigation
(blockIdx cursor), per-block rendering (cyan focus gutter),
per-block rendering (yellow comment gutter), and per-block
rendering (footnote-below-block). All four are replaced. The AST
parser, the `Block` / `BlockKind` types, and the gutter-injection
plumbing around them are removed.

## Goals / Non-Goals

**Goals**

- A single cursor shape shared between message and file views,
  down to the field names: `(lineIdx, charPos, preferred)`.
- A single selection shape between message and file views:
  global byte range into the rendered text.
- A single comment anchor shape between message and file views:
  global byte range + (path, line range) for file-kind only.
- A single inline cursor rendering pattern: lifted from the file
  viewer and applied at message-view charPos.
- A single gutter rendering pattern for saved comments:
  per-source-line yellow `▍`, computed by byte-range overlap.
- A new appendix layout where message-kind entries reference the
  agent's quoted text instead of line positions.

**Non-Goals**

- Markdown rendering beyond the current raw-source display.
  Glamour, chroma, or any markdown→ANSI transformation is out
  of scope (already out per `latest-message-view`).
- LSP, syntax highlighting, code folding — all file-viewer
  features only.
- Compose-mode changes beyond the appendix format.
- Configurable keybindings.
- Block-level commentary in any form. The `c`-no-visual anchor
  is the whole current source line; there is no "whole message"
  or "whole block" kind.

## Decisions

### 1. Source-line basis for the cursor

**Choice:** `lineIdx` is 1-based into
`strings.Split(m.latest.Text, "\n")`. `charPos` is a byte offset
into that source line, snapped to rune boundaries, clamped to
`[0, len(currentLine)]`.

**Why:** Matches the file viewer's `(cursor, charPos)` shape
exactly. The existing `wrappedToSrc` / `sourceToFirst` projection
already maps source-line indices to rendered-line indices, so
viewport-follows-cursor keeps working. `strings.Split` on `\n`
is stdlib, no AST parser needed.

**Alternatives considered:**

- Rendered-line basis (`b`). `charPos` semantics get ugly when a
  rendered line spans multiple source lines (a wrapped
  paragraph). Comment anchors become rendered-line based, which
  diverges from file-kind anchors. Rejected.
- "Logical line" basis (splitting on `\n` AND blank-line groups).
  Adds parsing for no UX gain. Rejected.

### 2. Preferred-column tracker

**Choice:** New `preferred int` field on `NavCursor`. `l` sets
`preferred = max(preferred, charPos)`. `h`, `j`, `k` leave
`preferred` unchanged. New-message-resets `preferred` to `0`.

The file viewer's `fileViewer.preferred` is the precedent; the
new `NavCursor.preferred` follows the same rules. Lifted from
the file viewer's `moveFileCursor` / `fileViewMoveRune`
helpers.

### 3. Selection is a global byte range

**Choice:** `NavSelection{ByteA, ByteC}` over `m.latest.Text`.
`v` sets `ByteA = ByteC = byteOffset(lineIdx, charPos)`.
`h`/`l`/`j`/`k` in visual update `ByteC` from the cursor's new
position. Selection ordering is byte-wise (`min(a, c)` is the
start; `max(a, c)` is the end); the visual highlight covers
every rendered line whose byte range intersects the selection.

**Why:** Matches the file viewer's
`fileSelection{LineA, CharA}` + byte-anchor pattern, where the
anchor side is fixed and the moving side is the cursor. The
file viewer uses per-line char offsets; we use global byte
offsets because the message view has a contiguous text buffer
(`m.latest.Text`) and the conversion cost is one offset lookup.

### 4. Comment anchor: global bytes, single shape

**Choice:** `Comment` struct drops `BlockIdx`, `CharStart`,
`CharEnd`. Adds `ByteA`, `ByteC` used by both kinds:

- **Message-kind:** `ByteA`, `ByteC` are byte offsets into
  `m.latest.Text`.
- **File-kind:** `ByteA`, `ByteC` are byte offsets into the
  file's raw content (or `0` for line-range-only comments).
  `Path`, `LineStart`, `LineEnd` are populated.

The `Kind` discriminator stays but collapses to
`CommentMessage` (zero value) / `CommentFile`. The
`CommentBlock` constant is renamed `CommentMessage` for kind
clarity. The `IsInlineSelection()` helper is removed (no
whole-block sentinel anymore — every comment has a byte range).

**Why:** The file-kind anchor was already global bytes; the
message-kind anchor was per-block bytes. Aligning them on global
bytes makes the appendix format simpler (one code path for byte
excerpts), makes the rendering simpler (one range test per line
regardless of kind), and removes the `CharStart == -1` sentinel
that the block-level kind depended on.

### 5. Block-level comment kind is removed

**Choice:** `c` in `stateNav` (no visual, or empty visual range)
anchors to the **whole current source line**: `ByteA =
lineStart[lineIdx]`, `ByteC = lineStart[lineIdx] +
len(lines[lineIdx]) + 1` (inclusive of the trailing newline).

**Why:** Without a `BlockIdx` concept, there is no "whole
block" to anchor to. The closest analog is "whole current line",
which is the file viewer's `c`-no-visual behaviour.

### 6. Inline cursor rendering is lifted from the file viewer

**Choice:** The file viewer's `applyCursor` /
`spliceInvert` helpers in `model.go` are reused. They operate on
ANSI-styled content (a string with embedded `\x1b[…m` codes);
the message view's rendered markdown source already uses the
same convention. The helper is called on the line that contains
the cursor's `lineIdx`, at `charPos`.

**Why:** Avoids two implementations of the same splice. The
file viewer already has this tested (workspace-files spec
covers the byte-over-selection rule).

### 7. Per-line yellow gutter computed by overlap

**Choice:** Replace `render.InjectGutterWrapped` (which walks
blocks) with a new `render.LineGutter(text, comments, width)`
helper that:

- Splits the text into source lines on `\n`.
- For each source line `[byteStart, byteEnd)`, checks every
  saved comment's `ByteA < byteEnd && ByteC > byteStart`.
- Emits a wrapped version of the line (width = `wrapWidth`)
  with a yellow `▍` (228) gutter on every wrapped sub-line if
  the source line has any overlap, or a space gutter otherwise.

**Why:** The current `InjectGutterWrapped` keys off block
indices that no longer exist. The new shape is a direct port:
walk source lines, check overlap with comment byte ranges,
emit gutter + wrapped line. The output format (one gutter char
per wrapped sub-line) is unchanged, so the message viewer's
external layout doesn't drift.

### 8. Appendix split by kind, no count line

**Choice:** New `render.FormatCommentsAppendix(comments)`
function returns the full appendix body as a single string:

```
Comments on the message:
- comment on "<excerpt>": <text>

Comments on files:
- file "<path>" (lines X-Y): <text>
```

Message-kind excerpt rules:

- Truncate to 40 characters with `…` appended if longer.
- Replace internal `\n` with single space.
- Collapse runs of whitespace to single space.
- Trim leading / trailing whitespace.

Empty sections omit the label and entries. Two empty sections
collapse to a single blank line. No leading `---`, no count
line.

**Why:** The agent's API for "comment on the message" is by
quoted text (the agent will recognise its own words), not by
line number. The split puts message-kind and file-kind
comments in distinct groups so the agent can tell them apart
without parsing the entry prefix. Empty-section collapse keeps
single-kind payloads clean.

### 9. Scroll keys forward to the viewport only

**Choice:** `↑`, `↓`, `PageUp`, `PageDown`, `Home`, `End` in
`stateNav` are bound in the keymap to direct viewport model
calls (`viewport.LineUp`, `viewport.LineDown`,
`viewport.HalfPageDown`, etc.) **without** touching the
cursor. Viewport-follows-cursor is suspended while the user is
scroll-keying; `j` / `k` re-engages it on the next motion key.

**Why:** Matches the file viewer's separation between "move the
cursor" (`j`/`k`/`h`/`l`) and "scroll the viewport" (the named
scroll keys). The suspension rule keeps the user's scroll
position from snapping back when the cursor would otherwise
fall outside the visible range.

### 10. Markdown AST parser is removed

**Choice:** The `render.Parse` (or equivalent) call that
produces `m.blocks` is removed. `m.blocks` becomes a
non-existent field. `m.latest.Text` is rendered by passing it
through the new `LineGutter` + `applyCursor` pipeline. The
existing `wordWrap` helper stays.

**Why:** Every consumer of `m.blocks` (cursor nav, cyan gutter,
yellow gutter, footnote below block, `IsInlineSelection`,
`CurrentBlockIdx`, `NavLineIndex`, `RenderMessageWithComments`)
is replaced by line-direct equivalents. The markdown AST
parser exists solely to feed those consumers; with them gone,
the parser is dead code.

## Risks / Trade-offs

- **[Risk]** The new cursor shape is breaking: every test that
  reads `cursor.BlockIdx` must change. → **Mitigation:** tasks
  are sequenced so the `NavCursor` rewrite comes with a
  compilation break that's fixed in the same task; tests are
  rewritten against the new shape in batches.
- **[Risk]** The new appendix format changes the bytes the
  agent receives. Existing agents / downstream tools that
  parse the appendix text need to know. → **Mitigation:**
  `README.md` is updated to show the new format. The
  `agent-redirect` spec mentions the appendix format by
  example; its scenario text is unchanged because the spec
  describes the dispatch, not the format.
- **[Risk]** `applyCursor` operates on ANSI-styled content; if
  the message viewer's render path produces plain text (no
  ANSI), `applyCursor` is a no-op for the body. → **Mitigation:**
  the message viewer already uses ANSI escapes for the gutter
  (`\x1b[38;5;228m`), so the input to `applyCursor` is styled;
  the function's splice behaviour works on styled strings
  because of the existing `spliceInvert` implementation.
- **[Risk]** Scroll-key separation breaks muscle memory for
  users who relied on `↓` / `↑` to move the cursor.
  → **Mitigation:** the change is opt-in via the new keymap; the
  `↓` / `↑` keys are still visible in `?` help so users can
  discover the new behaviour.
- **[Risk]** `c` in nav with no visual now anchors to a whole
  source line; users who expected "whole block" might
  over-anchor. → **Mitigation:** the line anchor is short
  enough to re-edit by entering visual and re-selecting; the
  new behaviour matches the file viewer's `c`-no-visual.
- **[Trade-off]** Removing `m.blocks` simplifies the message
  viewer but loses the per-block context that the cyan gutter
  used to provide (you no longer see "which paragraph you're
  in" at a glance — only the byte position). The inline cursor
  at charPos gives precise position but not coarse structure.
  This is accepted as part of the alignment with the file
  viewer.

## Migration Plan

The change is a single atomic commit: the `stateNav` cursor,
selection, comment anchor, gutter rendering, keymap, and
appendix format all change together. There is no incremental
state where some parts of the change are live and others are
not — partial deployment would produce a broken model.

- **Code:** `model.go`, `internal/render/nav.go`,
  `internal/render/render.go`, `internal/render/comment_render.go`,
  `internal/render/comment_appendix.go`, `keymap.go`,
  `view_test.go`, `model_test.go`, `file_test.go`,
  `lspfileview_test.go` (and their sibling render tests).
- **Docs:** `README.md` keymap and appendix example.
- **No data migration:** comments are in-memory only
  (`message-comments` spec). The change has no persistent
  state to migrate.
- **Rollback:** revert the commit. No state, no schema, no
  persistent files affected.

## Open Questions

None. All decisions are resolved at design level; remaining
choices (excerpt length 40 chars, separator `\n\n`,
single-blank-line collapse for empty sections) are pinned in
the spec scenarios and tasks below.