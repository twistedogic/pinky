# Add a visible tab header

## Why

Pinky already has two top-level views — the latest-message view
(`tabMessage`) and the file review tab (`tabFiles`) — and `Tab` toggles
between them. The only on-screen signal of which tab is active today is
a tiny `msg` / `files` chip on the bottom-right of the status line,
which is easy to miss and read as decorative. A user who lands in the
TUI mid-task has no at-a-glance way to tell which surface they're
looking at, and no hint that the other surface exists.

A small, persistent tab row at the top of every attached state names
both surfaces, makes the active one unmissable, and turns the
existing toggle into something discoverable instead of a power-user
gesture. Because the row replaces the now-redundant status-line chip,
the diff is mostly additive.

## What Changes

- **New 1-row tab header** sits above the existing cwd header on
  every attached state. Two cells: `Message` and `Files`. The active
  cell renders with a filled background; the inactive cell renders
  dim. The row is a passive indicator — not clickable, not focusable.
- **Drop the status-line tab chip.** The `msg` / `files` chip in
  `statusLine()` is removed; the tab header carries that information
  on its own. The status line keeps every other chip (streaming dot,
  `[I] include N comments — ON/OFF`, `[VISUAL]`).
- **`Tab` keypress behavior is unchanged.** The header is purely
  visual; toggle semantics, round-trip sub-state restoration, and
  the `Tab` binding table in `workspace-files` stay exactly as they
  are.
- **Header height accounting accommodates the new row.** The
  existing `m.headerHeight` field already measures the cwd header's
  row count and `reflow()` subtracts it from the viewport height;
  the tab row folds into the same accounting so a 2-row cwd + a
  1-row tab header = `headerHeight == 3` regardless of which state
  is active.
- **Picker and error screens skip the tab row.** No agent attached
  → no meaningful tab state. Mirrors how the cwd header is already
  skipped on those screens.

## Capabilities

### New Capabilities

- `tab-header`: a single-row tab indicator above the cwd header that
  labels both top-level views and shows which is active; falls
  through picker and error screens.

### Modified Capabilities

- `workspace-files`: requirement #10 ("status line SHALL show a
  small chip 'msg' / 'files'") is removed. The tab header replaces
  its function; no other requirement changes.

## Impact

- `model.go`:
  - New `tabHeader()` method (~6 lines) returns the rendered row.
  - New `activeTabStyle` / `inactiveTabStyle` lipgloss styles.
  - `headerView()` prepends the tab row to its output for attached
    states; `m.headerHeight` accounting extends naturally because
    `strings.Count(m.headerView(), "\n") + 1` already includes the
    added row(s).
  - `statusLine()`: drop the `tabChipStyle` append block.
  - No changes to `handleKey`, `toggleTab`, `m.tab`, or any state
    machine.
- `view_test.go` / `header_test.go`:
  - New fixture asserts the tab row is present in every attached
    state and absent in picker / error states.
  - New fixture asserts the active cell swaps between `Message` and
    `Files` when `m.tab` flips.
  - `TestStatusLine_SlimmedOmitsPaneAndCommentCount` and the cwd
    one-line tests get a single tweak: the status line no longer
    carries the chip (already covered by a new assertion).
- `README.md`: the Tab keymap row mentions `tab UI` /
  `switch to the file review tab`; the new visible header is shown
  implicitly by the existing ASCII diagram, no doc change required
  unless the wording is wrong. Implementation phase decides if a
  short note belongs there.
- No new dependencies. No keymap changes. No help-overlay text
  changes. The `Tab` keypress semantics stay identical.
