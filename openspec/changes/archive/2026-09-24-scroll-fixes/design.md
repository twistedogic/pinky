## Context

Pinky renders two large texts (the latest assistant message, and the file
the user opens in the file tab) inside a bubbletea TUI. Both renderings hit
the terminal's height ceiling; both need scrolling. The current code only
implements scrolling in one place and does it wrong there:

- `stateNav` (latest message) uses `bubbles/viewport.Model` correctly, but
  the `sessionMsg` poll handler at `model.go:376` calls `m.viewport.GotoBottom()`
  after every refresh — including polls that bring no new text. Any user
  attempt to scroll up is reverted within 500 ms.
- `stateFileView` (file content) has no viewport at all. `fileViewView()`
  iterates `m.fileViewer.lines` and writes every line to the output buffer.
  Bubbletea passes the string to lipgloss and out the terminal; content past
  the height is clipped. The fileViewer tracks a 1-based line `cursor`, but
  the cursor has no visual representation anywhere in the renderer — only
  the visual-selection highlight is drawn, and that only on the visible
  window.

`bubbles/viewport` (v1.0.0) is already a direct dependency; it provides
`AtBottom()`, `SetYOffset()`, `GotoBottom()`, and routes arrow / PageUp /
PageDown / mouse-wheel keys through `Update()`. The 1-line cushion math used
by `scrollCursorIntoView` for the message view is the right pattern to
re-use for the file view.

## Goals / Non-Goals

**Goals**

- Make the latest-message viewport readable when scrolled up: stop the
  500 ms poll from yanking the user back to the bottom.
- Make the file viewer scrollable, with a position indicator and the same
  vim-style follow-cursor behaviour as the message view.
- Re-use the existing `j`/`k` cursor-driven selection model — the user has
  already learned that `j` extends the visual selection by one line; do not
  change that semantics.
- Surface file-viewer key changes in `README.md` and the help footer only
  if needed; no new keybindings are introduced.

**Non-Goals**

- Adding a current-line highlight in the file viewer (the cursor remains
  invisible to the eye; selection highlight is still the only visible
  cursor). A future change can add a `▍` for the current line.
- Replacing `bubbles/viewport` with a hand-rolled windowed renderer.
- Changing the visual selection model (still snaps `charA`/`charC` to line
  endpoints on `j`/`k`).
- Adding mouse-wheel scrolling explicitly — the viewport's built-in mouse
  handling already covers it.

## Decisions

### Sticky-bottom on the message view

- Before calling `refreshViewport()` in the `sessionMsg` handler, capture
  `wasAtBottom := m.viewport.AtBottom()`.
- After `refreshViewport()`, only call `m.viewport.GotoBottom()` if
  `wasAtBottom` is true. This is the Slack/Discord/Tailwind UI pattern:
  auto-follow while the user is at the bottom; freeze when they scroll up;
  re-attach when they return.
- Genuinely new messages (`last.Text != m.latest.Text` after the update)
  force-attach: the user wants to see the new content. The cleanest
  expression is to treat the new-message path as "wasAtBottom = true" so
  the GotoBottom still fires, without introducing a special branch.

The change is 3 lines in `model.go` (capture / conditional GotoBottom). The
existing `scrollCursorIntoView` already handles "cursor moved off-screen
via j/k → scroll into view" and is unchanged.

**Alternative considered**: add an explicit `End` / `G` keybinding to
manually re-attach. Rejected — `End` is forwarded to the viewport already
(it's a special key in `handleNavKey`), so the user can already re-attach
by pressing `End`. No new binding needed.

### File viewer uses `bubbles/viewport`

- Add `viewport viewport.Model` to `fileViewer`. The viewport is sized in
  `reflow()` (already exists for the message viewport) — file-view uses
  the same height the message view would have used (`vpHeight` in
  `reflow()`).
- Replace the body of `fileViewView()` (header + per-line loop) with: build
  the full rendered string into a `strings.Builder`, hand it to
  `m.fileViewer.viewport.SetContent(...)`, return `m.fileViewer.viewport.View()`.
- The header line gains `lines <topVisible+1>-<topVisible+Height> of <total>`
  so the user can see position; rendered via the same `headerStyle`.
- `openFile` (the file-open transition at `model.go:945`) seeds the
  viewport with a `viewport.New(w, h)` and `SetContent(...)` of the rendered
  string. Initial YOffset = 0.

### Cursor still drives scroll in the file viewer

- Keep `m.fileViewer.cursor` (1-based line index) and the existing
  `fileViewMoveLine(delta int)` logic — `j`/`k` move the cursor, extend the
  visual selection if active, snap `charA`/`charC`.
- After the cursor moves, call a new `scrollFileCursorIntoView()` that
  mirrors the message-view pattern: if the cursor is outside
  `[YOffset, YOffset+Height)`, scroll so the cursor lands on the last
  visible line (1-line cushion, same math as `scrollCursorIntoView`).
- Arrow keys (`↑`/`↓`) and PageUp/PageDown are forwarded to
  `m.fileViewer.viewport.Update(msg)` from `handleFileViewKey` — these
  scroll without moving `m.fileViewer.cursor`, so the cursor can drift
  off-screen intentionally when the user just wants to peek at another
  part of the file (matches vim's `Ctrl+Y`/`Ctrl+E` mental model).
- `h`/`l` remain rune-granular in visual mode only, unchanged.

### Visual selection still works

- The selection highlight is applied via `applySelection(line, lineNo, sel)`
  inside the rendered string before it goes into the viewport. If the
  selection spans lines outside the visible window, the highlight is
  invisible — same as today. Scrolling reveals it. No change to the
  selection data model.

## Risks / Trade-offs

- **[Risk]** Sticky-bottom means the user can scroll up and the new
  streaming text accumulates off-screen; when they return they see a big
  jump. → **Mitigation**: this is the standard chat-UX trade-off and is
  what the user expects; an on-screen `[N new lines]` chip could be added
  later if it becomes confusing.
- **[Risk]** File viewport introduces a second viewport state to manage
  alongside the message viewport; reflow / setSize must keep both
  consistent. → **Mitigation**: both are sized in the single `reflow()`
  function; same width and same height formula. Add a small test that
  `reflow()` updates both viewports identically.
- **[Risk]** Initial `GotoBottom()` on a new file shows the bottom of the
  file rather than the top. → **Mitigation**: not currently a problem (no
  user has reported it), but easy to revisit — change initial YOffset from
  0 to `maxYOffset` or leave at 0 depending on user preference. Default:
  top (YOffset = 0). Document in the spec scenario.
- **[Trade-off]** No visible "current line" marker in the file viewer —
  the cursor is still invisible. → Acceptable for this change; documented
  as non-goal. Can be a follow-up.

## Open Questions

- File viewer initial YOffset: top (0) or bottom (max)? Defaulted to top
  above; flip if you want file opens to land at the bottom like chat.
- Indicator format in the header: `lines 1-20 of 500` or `[1/500]` or a
  percent? Defaulted to the `N-M of K` form (most informative at a glance).
