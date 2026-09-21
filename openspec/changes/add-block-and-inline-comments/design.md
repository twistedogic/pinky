## Context

`pinky` is a TUI that sits next to a coding agent (pi/codex) in a
tmux split, tails the agent's session output, and renders the
latest assistant message as markdown with block-level navigation
(`}`/`{` to jump blocks, `]]`/`[[` to jump headings, `gg`/`G` for
top/bottom). The current code uses `goldmark` to parse the message
into a goldmark AST and extract a `[]Block` index of top-level
non-empty blocks (heading/paragraph/code/list-item/blockquote),
then feeds each block's reconstructed source to `glamour` for ANSI
rendering. Block boundaries are tracked as `StartLine`/`EndLine`
indices into the concatenated rendered output.

There is no way for the user to mark a region of the rendered
message, attach a note to it, or carry notes into their push-back
to the agent. This change adds that capability: block-level and
inline line-granularity annotations, stored in memory, with
inline footnote rendering and an opt-in to embed them in the
redirect payload.

Stakeholders: a developer reviewing the agent's response who wants
to flag regions for follow-up or capture context to send back.

## Goals / Non-Goals

**Goals:**

- Block-level annotation: `m` opens a composer pre-anchored to the
  currently-focused block; saving stores the comment.
- Inline annotation: `V` enters vim-style visual line mode; `j`/`k`
  extend by line, `}`/`{` extend by block; `c` opens the composer
  pre-anchored to the selection; `Esc` exits visual mode.
- Auto-scroll while in visual mode so off-screen cursor movement
  remains visible.
- Inline footnote rendering: each saved comment produces a footnote
  line directly below its block, a background tint on the block's
  rendered lines, and a left-gutter marker on the first line.
- Block-local lifecycle: `e` edits the most recent comment on the
  current block; `d` deletes it; `n`/`N` jump between commented
  blocks.
- Compose integration: `Ctrl+I` in compose toggles whether the
  redirect payload is sent with a numbered comments appendix.
- In-memory only: comments live on `model`; cleared on message
  change, detach, and quit.
- Border / comment highlight coexist on the currently-focused
  commented block (border on top, tint under).

**Non-Goals:**

- Disk persistence, history integration, or cross-session
  continuity.
- Character-granularity visual mode (h/l moves). Inline is line
  granularity only.
- Threaded comments, replies, status flags (open/resolved), or
  author attribution.
- A whole-message comment (no current block). Comments must
  anchor to a block.
- Multiple comments on a block as a navigable list. The "most
  recent" wins for `e`/`d`; older comments remain stored but are
  not directly addressable in v1.
- Comment text length limits or styling customization.

## Decisions

### D1. Anchor shape: `(BlockIdx, CharStart, CharEnd)`

Block-level comments use `CharStart = CharEnd = -1` as a sentinel
for "no inline range". Inline comments use the byte offsets
returned by the goldmark AST's `Lines().At(i).Start` / `.Stop` for
the anchor line and the cursor line. The original message text is
stored verbatim (`src[CharStart:CharEnd]`) so the redirect appendix
can quote the source without re-parsing.

**Alternative considered:** viewport-line range (`startY`,
`endY`). Rejected because it breaks under width reflow — a
comment anchored at viewport line 6 stays at viewport line 6 even
when the same content moves to viewport line 9 after a width
change. Byte offsets into the source survive reflow.

**Alternative considered:** block index only. Rejected because
inline comments would lose precision — "block N" can't quote a
single sentence in the redirect appendix.

### D2. Visual mode is line-granularity only

The visual mode cursor tracks rendered lines, not characters.
`h`/`l` are not bound in visual mode. This avoids the question of
"what is character 7 of a wrapped line?" — glamour's wrap is
opaque to us, and tracking character offsets through it would
require either intercepting glamour's wrap or post-hoc
projection that drifts under reflow.

**Alternative considered:** character-granularity visual mode
(vim's lowercase `v`). Rejected because glamour's word-wrap
splits one source line into N rendered lines without exposing
the split points, so a character cursor cannot be projected
onto the rendered output reliably.

### D3. Comments stored on `model`, cleared on message change

The model holds `comments []render.Comment` and `msgHash string`
(a short hash of `latest.text`). When `latest.text` changes
during a poll, `msgHash` flips and `comments` is reset. On
`attach()` and `Quit` the slice is reset to nil.

**Alternative considered:** disk persistence. Rejected per user
direction — in-memory only.

### D4. Footnote rendered inline below the block

Each commented block gets one or more footnote lines appended
immediately after the block's `EndLine`, before the next block's
`StartLine`. The footnote line uses a lipgloss style distinct
from the rendered markdown (italic + dim accent), so it doesn't
collide with adjacent prose. The block's rendered lines
themselves receive a background tint; the first line gets a
gutter marker (`▸` for block-level, `•` for inline).

**Alternative considered:** side panel / annotations list.
Rejected per user direction — comments live alongside their
blocks.

### D5. Inline range projection via goldmark `Lines()`

Given a block's AST node and a target rendered line index
within that block, the source byte offset for that line is
`node.Lines().At(renderedLineIdx - blockStartLine).Start`. The
glamour renderer emits one rendered line per source line for
non-wrapping blocks and N rendered lines per source line for
wrapping blocks. The projection accepts a 1-line drift at wrap
boundaries (one rendered line may highlight slightly off the
exact source line) — the source excerpt stored in the comment
remains exact, so the redirect appendix is correct.

**Alternative considered:** post-hoc scan of the rendered
output to detect wrap boundaries. Rejected because glamour's
ANSI output mixes styles and colors with content, making wrap
detection fragile.

### D6. Border pass and comment pass share the line-rewrite loop

`model.go`'s `injectBorder` already splits the rendered string
into lines, iterates, and writes each line with optional border
lines inserted at `StartLine` and `EndLine`. The new
`injectComments` pass iterates the same line slice and applies
background tint + gutter marker for lines inside any commented
block. The two passes run in order: comments first, then
borders, so border lines themselves are not tinted.

**Alternative considered:** single combined pass. Rejected
because the existing `injectBorder` is tested in isolation
(`border_test.go`) and merging would break that contract.

### D7. Auto-scroll rule: cursor off-screen triggers viewport follow

In visual mode, after every `j`/`k`/`}`/`{`, the cursor's
rendered line is checked against `viewport.YOffset..viewport.YOffset+viewport.Height`. If the cursor is outside that
range, the viewport's YOffset is adjusted (clamped to the total
line count) so the cursor sits one line inside the visible
range. The check is O(1) and runs only in visual mode.

**Alternative considered:** always scroll to keep cursor
centered (vim's default `scrolloff=5`). Rejected because it
makes single-line `j`/`k` movements visibly jump the viewport;
a 1-line cushion is less surprising for a TUI.

### D8. Redirect appendix format

```
<redirect text>

---
N comments:
- block "<excerpt>" (lines X-Y): <comment text>
- inline "<excerpt>" (line Z): <comment text>
```

`excerpt` is the first ~40 chars of the block source or inline
range with trailing ellipsis if truncated. `X-Y` is the
block's `StartLine..EndLine`; `Z` is the inline range's
`StartLine` only. Comments are listed in `CreatedAt` order
(chronological). When the include flag is ON but `len(comments)
== 0`, no appendix is appended (the redirect is sent as plain
text).

**Alternative considered:** markdown bullet list with `**type**`
formatting. Rejected for simplicity — the agent gets a
scannable plain-text appendix that doesn't depend on its own
markdown rendering.

### D9. `n`/`N` navigation wraps around the comment list

`n` from the last commented block jumps to the first;
`N` from the first jumps to the last. This matches vim's `n`/`N`
search behavior and avoids dead-end states.

**Alternative considered:** clamp at the ends (no wrap).
Rejected because it produces a no-op keypress at the boundary,
which is more surprising than wrap-around.

## Risks / Trade-offs

- **[Risk] Inline range drift under glamour word-wrap.** Glamour
  may split one source line into multiple rendered lines; the
  projection uses source-line indices, so the highlight may
  cover one extra or one fewer rendered line at the wrap
  boundary. **Mitigation:** the source byte offsets and excerpt
  stored in the comment are exact; the highlight drift only
  affects the visual marker, not the redirect appendix. Test
  covers the projection with a wrapping paragraph.

- **[Risk] Footnote line collision with the next block's first
  line.** If a block's `EndLine` is the last line of the
  viewport, the footnote line gets pushed below the fold.
  **Mitigation:** auto-scroll on comment creation jumps the
  viewport to show the new footnote; status bar shows comment
  count so the user knows footnotes exist off-screen.

- **[Risk] Visual mode cursor drift on very narrow terminals.**
  When the viewport is narrower than 8 columns, line-down
  movements may scroll past content boundaries.
  **Mitigation:** clamp cursor at `[0, totalLines-1]`; tests
  cover narrow-width cases.

- **[Risk] `d` deletes without confirmation.** A misclick deletes
  the most recent comment on the current block. **Mitigation:**
  `d` is a single-press delete, but a follow-up `Ctrl+Z` (undo)
  is out of scope for v1; user-direction trade-off accepted.
  Document in the help overlay.

- **[Trade-off] No character-granularity visual mode.** Inline
  selection is line-granularity only. Users who want to quote
  "just the function name" must select the whole line.
  **Accepted:** glamour's wrap is opaque; tracking characters
  through wrap is fragile and adds significant complexity for a
  feature that's rarely worth it in practice.

- **[Trade-off] `e`/`d` operate on the most recent comment per
  block only.** A block with multiple inline comments exposes
  only the latest for edit/delete via `e`/`d`. Older comments
  are preserved in the slice but not directly addressable.
  **Accepted:** v1 scope; a future change can add a per-block
  comment list if needed.

- **[Trade-off] `d` has no undo.** A deleted comment is gone for
  the session. **Accepted:** in-memory storage; re-creating is
  trivial.

## Migration Plan

This is a pure feature add. No existing behavior changes:

- The help overlay (`keymap_md.go`) gets three new keymap
  groups: `mark`, `visual`, `comments`. Existing groups are
  unchanged.
- `RenderMessage` continues to work as-is for the help overlay
  (which has no comments). A new `RenderMessageWithComments`
  function handles the main view with overlays.
- No CLI flags change. No config file changes. No file format
  changes.
- Rollback: revert the commit. No data is persisted, so no
  migration of existing state is needed.

## Open Questions

None at proposal time. The choices above resolve the design
decisions surfaced during exploration. If implementation
uncovers new questions (e.g., a glamour quirk in word-wrap
projection that the drift risk doesn't capture), the design
doc will be amended as part of that work.
