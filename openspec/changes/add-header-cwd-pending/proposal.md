# Proposal

## Why

Pinky currently has no persistent surface for two pieces of state the user
cares about: which directory the watched agent is working in, and how many
comments have accumulated but not yet been flushed to the agent. The agent's
cwd is known (`m.fileRoot`, set at attach from `session.PaneCwd`) but never
displayed. The unflushed comment count is only shown while in `stateCompose`;
outside compose, accumulated comments are invisible until the user remembers
they exist. A top header line that always shows both pieces closes this gap
without disrupting the existing status-line rhythm.

## What Changes

- Add a new one-row top header above the viewport in all attached states
  (`stateNav`, `stateCompose`, `stateCommentComposer`, `stateFileNav`,
  `stateFileView`). Not rendered in `statePicking` or `stateError`.
- Header left field: abbreviated agent cwd (`~/`-prefixed, tail-shortened to
  fit). Header right field: unflushed comment count, always rendered, yellow
  when zero, bold yellow when `> 0`.
- Header wraps to two rows when the natural single-row width exceeds the
  terminal width. Wrap strategy: abbreviate cwd to `~/last-2-segments` first;
  if still too wide, drop a segment; last resort, split on `/`.
- Header height is dynamic (1 or 2 rows). `tea.WindowSizeMsg` distributes the
  remaining rows to the existing viewport / textarea / file viewer.
- Slim the existing bottom `statusLine()`: drop `m.pane` and
  `len(m.comments)` (now in the header). Keep streaming dot, `[I] include N
  — ON/OFF` (compose only), `VISUAL` chip (visual mode only), and the tab
  chip.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None.

This change adds a presentation surface for state already governed by the
`message-comments` capability (comment accumulation / one-shot flush) and the
agent-attach flow (cwd). No requirement-level behavior changes — accumulating
comments, flushing them via `s`, and `PaneCwd` discovery all keep their existing
contracts. The header is a UI refactor of how existing state is surfaced.

## Impact

- `model.go` — new `m.headerView(width int) string` method; modification of
  `View()` to slot the header into the 5 attached-state branches; slim-down
  of `m.statusLine()`.
- Window-size handling — `tea.WindowSizeMsg` path needs to subtract header
  height from the viewport / textarea / file-viewer allocation.
- No new dependencies. No API changes. No changes to `internal/`, to
  history, to session discovery, or to the keymap.
- Visual budget: pinky gains 1 row of UI chrome in the wide case, 2 rows in
  the wrapped case. Viewport / file viewer / textarea all lose 1-2 rows of
  inner content. Acceptable; all three already have headroom.