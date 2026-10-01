# Design

## Context

`tabHeader()` currently builds the row as `activeCell + " " + inactiveCell`,
which positions the active cell on the left regardless of which tab is
active. The spec mandates a fixed slot order (Message then Files) and only
the highlight is supposed to move.

## Goals / Non-Goals

**Goals:**
- Cell order is `Message | Files` regardless of `m.tab`.
- Only the highlight (cyan fill vs. dim) moves between the two slots.

**Non-Goals:**
- Changing the spec text — the existing requirement already states the
  invariant; the bug is purely in the implementation.
- Changing the active/inactive styles (`activeTabStyle`, `inactiveTabStyle`)
  or the help-line padding.

## Decisions

- **Build in canonical order, swap styles per slot.** Render
  `inactiveTabStyle.Render("Message")` then `activeTabStyle.Render("Files")`
  (or vice versa, depending on `m.tab`). This keeps the row layout stable
  and moves only the style — the smallest change that fixes the bug.

  Alternative considered: keep the active-first build but reorder which
  label gets the active style. That also fixes the row visually but
  reads worse ("build active then inactive" implies order matters) and
  is one indirection harder to reason about.

- **No test name change for `TestTabHeader_ActiveCellSwapsWithMTab`.**
  Its assertion (`active escape is immediately followed by the active
  label`) is correct and passes both before and after the fix, because
  lipgloss emits the style escape immediately before its text. The bug
  is about row order, which the new `TestTabHeader_FixedCellOrder` test
  covers.

## Risks / Trade-offs

- None material. The fix is a literal swap of which cell gets which
  style on the `tabFiles` branch; the existing test continues to pass,
  the new test fails before the fix and passes after.