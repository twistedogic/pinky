## Why

Pinky's nav surface has drifted away from a single coherent keyboard.
Three sources of duplication and friction:

1. **Two nav dispatchers in one TUI.** `handleIdleKey` and
   `handleVisualKey` (each with its own state machine — `vim.Handle`
   vs `visual.Handle`) consume the same motion keys (`j k { } G`).
   Visual mode claims them first only because the two pipelines
   don't share a dispatcher. The "documentation-only" bindings
   (`VisualDown/Up/NextBlock/PrevBlock/Open/Exit` in `keymap.go`)
   exist because the bindings and the dispatcher don't agree on
   who owns nav. A bug in motion has to be fixed twice.

2. **Two cursors per session.** `m.viewport.YOffset` drives the
   idle "where am I" via `CurrentBlockIdx(blocks, YOffset)`;
   `m.visual.Cursor` (a rendered-line index) and `m.visual.CharC`
   (a byte offset) drive visual mode's "where am I and what am I
   selecting." These are three derived-from-three-sources of
   truth. The viewport `YOffset` and the visual cursor drift
   apart when scrolling.

3. **Non-vim keys in a vim-flavoured TUI.** `Ctrl+R`, `Ctrl+N`,
   `Ctrl+S`, `Ctrl+I`, `Ctrl+C` — five control chords in a tool
   whose first keymap (j/k/}/[/g) is already vim-style. The two
   two-key sequences (`gg`, `]]`, `[[`) add a 500 ms timeout state
   machine for three bindings.

Unify dispatch onto one state machine, one cursor, and one
single-letter nav surface.

## What Changes

- **Single nav state machine.** Replace `internal/render/vim.go`
  and `internal/render/visual.go` with a single `nav.go`. The
  `VimState` two-key tracker is deleted (no `gg`, `]]`, `[[`).
  Visual mode becomes a `Mode` flag plus a selection range; the
  flag does not own dispatch.
- **Single cursor.** `model.cursor {blockIdx int, charPos int}`
  replaces the `viewport.YOffset`-derived cursor and the
  `visual.cursor / visual.charC` cursor. The viewport's
  `YOffset` is derived from the cursor (render the cursor's
  rendered line + 1-line cushion). One source of truth.
- **Single dispatch.** `handleIdleKey` and `handleVisualKey`
  collapse into `handleNavKey`. Mode keys (`v` enter visual,
  `Esc` exit, `c` comment) intercept at the top; everything else
  falls through to the nav state machine.
- **Single-letter nav surface (stateNav).** `j k h l v c s q r n`
  plus `?` and `Esc`. The previous bindings
  (`LineUp/Down/PrevBlock/NextBlock/BottomLine/Refresh/QuitIdle`)
  and the "documentation-only" `VisualDown/Up/NextBlock/PrevBlock/Open/Exit`
  are deleted. **`Ctrl+C`, `Ctrl+N`, `Ctrl+R`, `Ctrl+S`** (compose
  send) are deleted. **`BREAKING`** — users with the old keymap
  muscle-memory will need a one-line note in the help footer.
- **Universal send on `s`.** `s` in any state performs "send to
  agent": idle with accumulated comments → batch-send via
  `submitAllComments`; compose with text → `inject.Send` (+ optional
  appendix when `Ctrl+I` is on); empty / disabled contexts → no-op.
  The previous two entry points (idle `s`, compose `Ctrl+S`) merge
  into one. **`BREAKING`** removal of `Ctrl+S`.
- **`c` covers block-level comments.** With no selection, `c`
  opens the comment composer pre-anchored to the whole block
  (the previous `m` binding is gone). **`BREAKING`** removal of
  `m`.
- **`Ctrl+S` retained inside `stateCommentComposer`** to save the
  comment (the comment body is multi-line so a non-newline send
  key is needed; vim's own `:` is heavier than a single chord).
- **`Ctrl+I` retained in compose** to toggle the comments
  appendix.
- **Character-granularity visual mode.** The selection cursor
  uses byte offsets into the block source, not rendered-line
  indices. Selecting across block boundaries (`v` then `j`)
  extends across block boundaries.

## Capabilities

### New Capabilities

- `single-cursor-nav`: the unified cursor model
  (`m.cursor {blockIdx, charPos}`), the single nav state
  machine (`nav.go`), the universal send via `s`, and the
  per-mode keymap surface (stateNav keys: `j k h l v Esc c s q r n
  ?`).

### Modified Capabilities

- `latest-message-view`: the "Vim-style navigation" requirement is
  rewritten — `j`/`k` now mean block nav (not line nav); the two-key
  state machine and `gg`/`]]`/`[[`/`G` are removed; `h`/`l` are
  added for rune nav; the rendered-line-derives-`blockIdx` rule
  is replaced with a cursor-derives-YOffset rule.
- `message-comments`: "Visual inline line selection" is replaced
  with character-granularity selection; "Visual block-level
  annotation" (the `m` key) is removed (folded into `c`); the
  `(blockIdx, charA, charC)` anchor shape stays unchanged but
  applies to a character range, not a line range.
- `agent-redirect`: compose is entered via `n` (was `Ctrl+N`); the
  compose-mode send key is `s` (was `Ctrl+S`); manual refresh is
  `r` (was `Ctrl+R`).

## Impact

**Code touched**

- `internal/render/` — new `nav.go`; delete `vim.go` and `visual.go`
  contents (or merge into `nav.go`); delete `visualState`,
  `SelectionMode`, `VimState`, `VisualState` types.
- `model.go` — replace `m.viewport.YOffset`-derived cursor with
  `m.cursor`; collapse `handleIdleKey` + `handleVisualKey` into
  `handleNavKey`; replace `m.visual Cursor/Anchor/CharA/CharC/CurBlock`
  with `m.selection {blockIdx, charA, charC}` + `m.visual.Mode` flag;
  drop the `[[`/`]]`/`gg` two-key fallback at the bottom of the
  handler; add `handleSend` that branches on state for `s`; rename
  `stateIdle` → `stateNav` (call-sites in `reflow`, `View`,
  `help`, tests).
- `keymap.go` — collapse `keyMap`: delete `LineUp/Down`,
  `PrevBlock/NextBlock`, `BottomLine`, `Refresh`, `QuitIdle`,
  `Mark`, `EditComment/DeleteComment/NextComment/PrevComment`,
  `VisualDown/Up/NextBlock/PrevBlock/Open/Exit`, `Send`,
  `QuitError` rules; keep `Help`, `Compose` (renamed intent:
  "compose" = enter compose from nav), `Newline`, `Cancel`,
  `IncludeComments`. Add the new nav group
  (`j k h l v Esc c s q r n ?`).
- `keymap_md.go` — rewrite `keymapGroupsForState` to reflect the
  collapsed keymap (idle group → nav group).
- `*_test.go` — update `keymap_test.go`, replace `visual_test.go`
  with `nav_test.go`, replace `vim_test.go` with `nav_test.go`
  cases for the new state machine.

**APIs / public surface**

- `internal/render` no longer exports `VimState`, `VisualState`,
  `VimAction`, `VisualAction`, or `SelectionMode`. New export:
  `nav.Handle(rune, cursor, blocks) (NavAction, cursor, selection)`
  (or a similar signature taking pointers in).
- `model.cursor`, `model.selection`, `model.visual.Mode` are the
  new visible state for tests / external callers.

**Dependencies**

- No new Go module dependencies.

**Behaviour changes visible to users (BREAKING)**

- All bindings prefixed with `Ctrl+` (except `Ctrl+S` and `Ctrl+I`
  inside compose / comment-composer) are gone; replaced with
  single letters. The help overlay (`?`) carries the new surface.
- Visual mode is character-granularity (inline byte offsets) rather
  than line-granularity. Existing inline comments anchored at line
  ranges become character ranges; the projection logic from
  `add-block-and-inline-comments` is reused.
- Pressing `s` once in compose now sends (was `Ctrl+S`); the
  previous `Ctrl+S` is unbound outside comment composer.
- `m` is gone; `c` covers whole-block annotation.

**Files deleted or merged**

- `internal/render/vim.go` → contents migrate to `nav.go`.
- `internal/render/visual.go` → contents migrate to `nav.go`.
- `internal/render/visual_test.go` → migrate / rename to
  `nav_test.go`.
- `internal/render/vim_test.go` → migrate / rename to `nav_test.go`.
