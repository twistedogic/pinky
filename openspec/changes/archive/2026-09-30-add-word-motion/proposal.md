## Why

The `stateNav` cursor moves by rune (`h`/`l`) or by source line (`j`/`k`)
but has no word-level motion. Reviewing an agent's markdown response means
jumping between prose tokens ("should the cursor have moved from `cursor`
to `selects` to `the` to `next` to `word`...") one rune at a time, which
is the slowest primitive for the most common motion — moving to the next
or previous word. Vim's `w`/`b` cover this exactly.

## What Changes

- **Two new keybindings in `stateNav`**: `w` moves the cursor to the start
  of the next word; `b` moves it to the start of the previous word. Both
  cross `\n` (blank lines are separators).
- **Word definition**: a run of `[A-Za-z0-9_]` (vim's default
  `iskeyword`). Punctuation runs and whitespace both separate words; each
  punctuation run is also a word on its own.
- **Visual mode parity**: `vw` / `vb` extend the selection's `byteC` to
  the new cursor byte, mirroring `vh` / `vl`.
- **Preferred column rule**: `w` updates `preferred = max(preferred,
  charPos)` (like `l`); `b` leaves `preferred` unchanged (like `h`).
  `j` / `k` after a word-motion detour still return to the rightmost
  column reached so far.
- **Scope is `stateNav` only.** The file viewer keeps its own handler;
  bubbles' compose textarea already provides word motion through its own
  keymap. Adding parity there is a separate conversation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `single-cursor-nav`: add `w` and `b` rows to the `stateNav` key table;
  add a new requirement defining word-boundary motion with scenarios.

## Impact

- `internal/render/word.go` (new) — `isWordRune`, `nextWordStart`,
  `prevWordStart`. Pure functions on `(lines, lineStartOffsets,
  lineIdx, charPos)`.
- `internal/render/word_test.go` (new) — boundary scenarios (ascii,
  multibyte, blank-line, line-crossing, end-of-source).
- `internal/render/nav.go` — two new `NavAction` constants
  (`ActionWordRight`, `ActionWordLeft`); two new `case` arms in
  `NavHandle`. Returns match `ActionRuneLeft`/`ActionRuneRight` so the
  existing `scrollCursorIntoView` / `refreshViewport` path in
  `handleNavKey` works unchanged.
- `internal/render/nav_test.go` — scenarios for the new actions.
- `keymap.go` — two new `key.Binding` rows (`NavWordRight`,
  `NavWordLeft`) for help-overlay rendering; rune dispatch stays in
  `NavHandle`. Help strings: `"w", "word forward"` / `"b", "word back"`.
- `model.go` — `handleNavKey`'s `ActionWordRight` / `ActionWordLeft`
  fall into the existing motion branch (same as `ActionRuneLeft` /
  `ActionRuneRight`): call `scrollCursorIntoView()` then
  `refreshViewport()`. No new branches.
- `openspec/specs/single-cursor-nav/spec.md` — delta: two rows in the
  nav table; one new requirement with scenarios.
- `README.md` — two rows in the Nav key table (`w` / `b`).