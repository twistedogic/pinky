# Design

## Context

The two-view system (message / files) already exists in
`model.go` as `m.tab` and is toggled by `Tab`. The only
on-screen indicator today is a status-line chip that the user
proposed should become a visible row. The `m.headerHeight`
mechanism already subtracts the cwd row count from the viewport
height in `reflow()`, so adding a row is a matter of including
the new line in the header output — no new resize math is
needed. See `proposal.md` for motivation.

## Goals / Non-Goals

**Goals:**
- One row, two cells (`Message`, `Files`), rendered above the
  cwd header on every attached state, swapped active/inactive
  per `m.tab`.
- Drop the status-line chip; status-line behavior is otherwise
  unchanged.
- Reuse the existing `m.headerHeight` accounting so the viewport
  shrinks by 1 row when the tab row is present.

**Non-Goals:**
- No new state machine, no new key bindings, no new tab-related
  state.
- The picker and error screens do not get a tab row.
- No focus / click / hover behavior on the tabs (passive row).
- No new spec for keymap / help-overlay changes.

## Decisions

### D1 — Tab row is prepended to `headerView()` rather than added as a separate rendered row

The top header is already a single string assembled in
`headerView()`. Adding another line at the top is cheaper and
keeps the row count math (`strings.Count(..., "\n") + 1`) honest
without introducing a second header field.

Alternatives considered: (a) a separate `m.tabHeader string`
field combined at render time — adds a new field for one row
of static output, more code than it saves. (b) returning the
tab row from `View()` alongside `m.headerView()` — splits
related output across two methods.

### D2 — Active = filled background, inactive = dim foreground

The user picked the inverted/box look over underline and bold.
`lipgloss.Background("...")` on the active cell plus
`Foreground("241")` on the inactive cell matches the existing
palette (status-bar foreground is `241`; visual-mode chip uses
bg `51` cyan). No new ANSI colour codes are introduced.

### D3 — Reuse the `m.headerHeight` field, no new constant

`reflow()` already computes
`m.headerHeight = strings.Count(m.headerView(), "\n") + 1`
right after rendering the header. Because the tab row becomes
the first line of `headerView()`'s output, that line shows up
in the string and is counted automatically. No change to the
resize math; the viewport shrinks by exactly 1 extra row when
the tab row is present.

### D4 — `statusLine()` removes the chip block unconditionally

The chip block is a small `switch m.tab` that appends one
styled cell. Removing it leaves the streaming dot, `[I]`
include chip, and `[VISUAL]` chip untouched. The
`tabChipStyle` lipgloss style becomes dead code — deleted in
the same change to avoid the "kept around because it might be
useful" hangup.

### D5 — Cell separator: a single space, no `│`

Two cells separated by one space is the cheapest renderable
separator and reads cleanly enough at any terminal width. Box
characters (`│`, `┃`) were considered; the active/inactive
contrast already does the visual partitioning work, so an
extra glyph would be ink without information.

## Risks / Trade-offs

- **[Risk] At very narrow widths (`< 20 cols`), the two-cell
  row pushes the cwd header onto a third line and shrinks the
  viewport by 3 rows total.**
  Mitigation: the existing `headerView()` already wraps the
  cwd path on narrow terminals; this change adds 1 row on top
  of that. Behaviour is consistent: smallest terminals always
  pay the largest header cost.
- **[Risk] Existing tests pin the status line to
 80-column width and assume no tab chip.** Mitigation: those
 tests already pass; they only need to flip one assertion
 ("status line SHALL NOT contain `msg`/`files` chip").
- **[Trade-off] Removing the chip means there is no longer a
 way to identify the active tab when only the status line is
 visible (e.g. after `?` opens the help overlay).** Mitigation:
 the tab row sits permanently above the viewport and is never
 hidden by `?`; the help overlay replaces the viewport, the
 tab header stays put.

## Migration Plan

- Code lands behind no flag.
- No data migration: `m.tab` already exists; no model schema
  changes.
- `README.md` mentions the `Tab` keypress and the existing
  diagram already shows two stacked panes (representing the two
  tabs implicitly). The implementation phase decides whether a
  one-line addition belongs there.
- Rollback: revert the commit. No persistent state to clean up.

## Open Questions

None. All decisions required for the spec and the tasks were
made during exploration: filled-active style, "Message" /
"Files" labels, drop the chip. The remaining unknowns
(best ANSI palette codes for filled background) are answerable
in one `git blame lipgloss.NewStyle()` call during
implementation.
