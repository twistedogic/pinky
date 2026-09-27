# Design

## Context

Pinky has a one-line bottom `statusLine()` that mixes persistent identity
(`m.pane`) with ephemeral state (streaming dot, visual-mode chip, tab chip,
compose-only `[I]` flag). It is rendered for every attached state. The
`agent`'s working directory is stored on `m.fileRoot` (set at attach time
from `session.PaneCwd`) and used internally by the file review tab, but is
not surfaced to the user. Accumulated comments on `m.comments` are only
counted in `statusLine()` while in `stateCompose`. See `proposal.md` for
motivation.

`reflow()` (`model.go`) already manages vertical layout by subtracting
`statusHeight` (1), `m.helpHeight()`, and additional rows for compose
textarea and file-view header from `m.height`. A new top header slots in
as one more row (or two when wrapped).

## Goals / Non-Goals

**Goals:**

- Add a persistent top header to the 5 attached states showing cwd and
  pending-comment count.
- Slim the bottom statusLine so it carries only ephemeral / state-specific
  chips.
- Preserve current visual rhythm at wide widths; allow graceful wrap at
  narrow widths.
- Single-method extraction (`m.headerView()`) called once per render from
  each attached state.

**Non-Goals:**

- No changes to comment accumulation, flush, or `s`-key behavior (owned by
  `message-comments` spec).
- No new dependencies. No keymap changes. No history / session / workspace
  changes.
- No theming work beyond reusing the existing color palette (yellow for
  the count, dim for the path).

## Decisions

### D1 — `headerView()` extracted as a method on `*model`, returns `string`

One method, no sub-methods. Returns a fully-padded multi-line string ready
to drop into `JoinVertical`. Width is derived from `m.width`; on the
zero-width pre-`WindowSizeMsg` case it returns the same fallback the
bottom statusLine uses today (`fillWidth` is a no-op when `m.width == 0`).

**Alternative:** Render via `lipgloss.JoinHorizontal` per row from a small
helper that picks left + right fields. Considered but adds two functions
for a one-row string; `JoinHorizontal` inside `headerView` is enough.

### D2 — Wrap pipeline: abbreviate → shorten → split

```
1. renderText = left(path) + pad + right(count)
2. if ansi.StringWidth(renderText) <= m.width: return renderText
3. path = shortenCwd(path)   // ~/last-2-segments
4. render again; if fits: return
5. path = shortenCwdOneMore(path) // ~/last-segment
6. render again; if fits: return
7. return splitOnSlash(path, count, m.width) // 2-line return
```

`shortenCwd` is a pure helper. `splitOnSlash` walks the path string,
greedy-prefixes segments until the line would overflow, returns two
strings. Never used in practice (steps 3-5 already fit at any reasonable
terminal width) but exists so the header never *clips* silently — a
clipped path is worse than a wrapped one.

**Alternative:** hard truncate with `…`. Rejected — visually quieter but
hides the path. Wrap-on-`/` already falls back hard.

**Alternative:** always render on two rows. Rejected — wastes a row at
wide widths where the data fits comfortably on one.

### D3 — Header height is dynamic, surface via a `m.headerHeight int` field

`reflow()` is called on every `WindowSizeMsg` and every state transition.
It already owns the row budget. Add one line that sets
`m.headerHeight = strings.Count(m.headerView(m.width), "\n") + 1` (or
`m.headerHeight = 1` for the simple case) and subtract from `vpHeight`.

Storing on the model lets the bottom statusLine / help still know how
much vertical real estate is gone without re-measuring. `headerHeight` is
also what `View()` uses to know whether to render 1 or 2 lines (the
return value of `headerView()` is the source of truth).

**Alternative:** measure on every render in `View()`. Rejected — the
header already measures itself; storing avoids re-measuring and keeps the
contract clear: header height is the only state `reflow()` needs.

### D4 — Slimmed `statusLine()` keeps: streaming dot, tab chip, `[I]` chip (compose), `VISUAL` chip (visual)

Drop from statusLine: `m.pane`, `len(m.comments)`. Both move to the header.

**Alternative:** drop the tab chip too (could go in the header). Rejected
— the tab chip is conceptually bound to the bottom row's tab-bar feel, and
moving it creates a second tab indicator on screen.

### D5 — Color: `pendingStyle` yellow always; bold yellow when count > 0

Two pre-declared styles next to `statusBarStyle` / `visualModeStyle` /
`tabChipStyle`:

```
pendingIdleStyle  = yellow, normal weight   // count == 0
pendingArmedStyle = yellow, bold            // count > 0
```

Header left (cwd) uses `dimStyle` (color 241, same as the existing
`statusBarStyle` foreground) so it does not compete with the count for
attention.

**Alternative:** red when > 0. Rejected — pinky uses red for errors; the
pending count is a normal "you have unsent stuff" signal, not an error.

## Risks / Trade-offs

- **Vertical budget shrink** — every attached state loses 1 row (wide) or
  2 rows (narrow). For very short panes the viewport could be squeezed.
  → `reflow()` already clamps `vpHeight` to `>= 1`. Acceptable.
- **Wrap pipeline complexity** — three helper functions for what is mostly
  a one-row string. → Worth it: the only thing worse than a 3-step wrap
  is a clipped or overflowing header.
- **Color cue fatigue** — yellow pending count is always on, including 0.
  If it becomes noise we can switch to `pendingIdleStyle` matching
  `statusBarStyle` foreground so 0 is invisible.
  → Tracked as a follow-up; not blocking.
- **Theming** — the existing `statusBarStyle` and tab chip live at file
  scope; new styles follow the same pattern. Theme overrides (out of
  scope today) would need to learn the new styles.
  → No theming work in this change.

## Migration Plan

Single deploy. No backward-compat concerns — pinky has no persisted
header state. Reverting means `git revert` of the merge commit. The
`m.headerHeight` field is the only new model state and is recomputed on
every reflow, so there is nothing to migrate.

## Open Questions

None. The wrap strategy and styling are pinned. The header height model is
the same shape as the existing `statusHeight` constant. Tasks can be
written without further design input.