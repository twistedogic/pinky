# Proposal

## Why

The tab-header spec already mandates "exactly two cells, **in this order:
Message and Files**", but the implementation in `model.tabHeader()` builds
the row by writing the active cell first, so when `m.tab == tabFiles` the
visual order becomes `Files | Message` instead of `Message | Files`. The
fix aligns the code with the existing spec.

## What Changes

- `model.tabHeader()` renders the row in canonical Message-then-Files
  order; the active style is selected per slot.
- `view_test.go` gains a regression test asserting cell order is invariant
  of `m.tab`.

## Capabilities

### New Capabilities
<!-- None — the project gains no new behavior. -->

### Modified Capabilities
<!-- None — the existing `tab-header` spec already states the invariant
     ("exactly two cells, in this order: Message and Files"); the bug is in
     the implementation, not in the spec text. `skip_specs: true` is set in
     `.openspec.yaml`. -->

## Impact

- `model.go`: `tabHeader()` (1 function, ~5 lines)
- `view_test.go`: 1 new test (~20 lines); no existing test changes