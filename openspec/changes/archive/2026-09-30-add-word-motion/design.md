## Context

`stateNav` already has rune-level (`h`/`l`) and line-level (`j`/`k`)
motion. Word motion is the missing third primitive. The natural shape
follows the existing `NavHandle` switch — add two `case` arms, two new
`NavAction` constants, two pure functions for boundary search. The
caller (`handleNavKey`) already routes any motion action through the
same `scrollCursorIntoView` + `refreshViewport` path; no new branches
needed there.

The interesting decisions are: what counts as a word, and how the
motion interacts with blank lines and line boundaries. Both are decided
up front and pinned in the spec so implementation has no ambiguity.

## Goals / Non-Goals

**Goals:**

- `w` moves cursor to the start of the next word; `b` moves cursor to
  the start of the previous word.
- Word = run of `[A-Za-z0-9_]`. Matches vim's default `iskeyword` —
  punctuation runs (`(`, `.`, `*`, backticks in markdown) are also
  words, so stepping through `foo.bar` lands on each piece.
- Motion crosses `\n`; blank lines are separators (vim semantics).
- `w` updates `preferred` like `l`; `b` leaves it like `h`. So
  `j`/`k` after word motion return to the rightmost column reached.
- Visual mode parity: `vw` / `vb` extend the selection's `byteC`.
- `stateNav` scope only.

**Non-Goals:**

- `e` / `ge` / `W` / `B` (word-end motion, whitespace-only-word motion).
  Same problem class; add later if a user asks.
- File viewer (`stateFileView`) word motion. Different cursor model,
  separate change.
- Compose textarea word motion. Already provided by bubbles' textarea
  keymap (`ctrl+←`/`ctrl+→`); no change needed.
- Capitalised `W` / `B` variants.

## Decisions

### Word class is `[A-Za-z0-9_]`

Two options were considered:
- **`iskeyword`** (`[A-Za-z0-9_]`): `foo.bar.baz` is three words
  separated by `.`. Predictable for code; matches vim default.
- **Whitespace-only**: `foo.bar.baz` is one word. Easier to reason
  about in prose but feels wrong to anyone used to vim on code.

Chosen: `iskeyword`. The message view renders markdown, which is full of
identifier-like content (`variableName`, `pkg.Func`), and the file
viewer also benefits from the same model even though that's out of
scope here. `iskeyword` is the vim default and what users expect from
"aligned with vim."

### `nextWordStart` / `prevWordStart` return `(lineIdx, charPos)`

The nav cursor is `(LineIdx, CharPos, Preferred)`, not a global byte
offset. Returning the same shape from the boundary helpers keeps the
caller's assignment to `cur.LineIdx` / `cur.CharPos` symmetric with
`j` / `k`. Internal search can use `lineStartOffsets` to project
between line-relative and global-byte positions when stepping across `\n`
boundaries, but the public return is always `(lineIdx, charPos)`.

### Blank-line rule mirrors vim

From a non-blank line, `w` skips any blank lines between the current
position and the next non-blank line, then lands on the first word of
that non-blank line. From a blank line, `w` jumps to the first word of
the next non-blank line. `b` mirrors. This is vim's behaviour and is
the expected feel for markdown messages with blank-line separators
between code blocks and paragraphs.

### Reuse existing motion dispatch path

`handleNavKey` switches on the returned `NavAction` and treats every
motion action (`ActionBlockDown`, `ActionBlockUp`, `ActionRuneLeft`,
`ActionRuneRight`) the same way: `scrollCursorIntoView()` +
`refreshViewport()`. Adding `ActionWordRight` / `ActionWordLeft` to
that same branch keeps the diff tiny — no new code path in `model.go`.

### Pure-function seam in `internal/render/word.go`

The boundary helpers take `(lines, lineStartOffsets, lineIdx, charPos)`
and return `(lineIdx, charPos)`. No cursor state, no model — the same
shape as `snapLeft`, `runeStart`, `runeAdvance` already in `nav.go`.
Test seam is direct: feed fixture lines, assert returned position.
No bubbletea test harness required for the core logic.

## Risks / Trade-offs

- **[Risk]** `b` on a word mid-line in visual mode could shrink a
  multi-line selection to a partial range. → **Mitigation:** This is
  expected vim behaviour; the spec scenario covers it explicitly
  (`vb` shrinks `byteC`, `byteA` unchanged until cursor crosses the
  anchor).
- **[Risk]** Punctuation-dense markdown (`#`, `*`, backticks) creates
  many "single-rune words" that `w` steps through one at a time, which
  feels noisy. → **Mitigation:** Same as vim; users who want bigger
  steps use `W` (whitespace-only words), which is a follow-up change.
- **[Trade-off]** `iskeyword` excludes `-`. Markdown headings like
  `# Section-name` treat `-` as punctuation, so `w` over `Section-name`
  pauses on every `-`. Acceptable — vim does the same and `Section`,
  `-`, `name` are distinct identifier-style chunks.
- **[Trade-off]** No `2w` / `5b` count prefixes. Adding them would mean
  a small parser in `NavHandle` for the `digit` cases. Out of scope.