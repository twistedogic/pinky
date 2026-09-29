# Design

## Context

The message viewer (`internal/render/nav.go`) already runs on a
vim-style anchor+cursor model: a single `NavCursor` (block + byte
offset) drives nav and is the visual tail, and `NavSelection`
stores only the anchor (charA). The file viewer (`model.go`)
diverged: it has `fileViewer.visual.LineC/CharC` mirroring the
cursor, and `j`/`k` in visual mode hardcodes a "line-shape" snap
(`CharA=0`, `CharC=lineLen`). The result is that the user cannot
make a multi-line char selection, `j`/`k` clamps `charPos` to the
new line length with no preferred column, and LSP (`d`/`R`/`K`)
fires at the exact `(line, charPos)` so the user has to land
precisely on a token to query it. See `proposal.md` for the
motivation.

## Goals / Non-Goals

**Goals:**

- Unify the file viewer's model on the message viewer's
  anchor+cursor shape.
- Let the user make a multi-line char selection with `v` and
  `j`/`k`/`h`/`l`.
- Let `j`/`k` preserve the intended column across lines
  (`preferred` tracker).
- Make `d`/`R`/`K` work without pixel-precise cursor positioning
  by firing at the word under the cursor.

**Non-Goals:**

- New motions (`w`/`b`/`e`/`0`/`$`/`^`/`gg`/`G`/`f`/`t`) — the
  user's review-only surface is `h`/`j`/`k`/`l` plus the word
  semantics of `d`/`R`/`K`. Defer.
- A separate `V` line visual mode — `c` with no visual defaults
  to whole line, and `v`+`j`+`c` covers multi-line line-range
  comments. Defer.
- A whole-file comment path from inside the viewer — explicitly
  dropped per the proposal.

## Decisions

### 1. `preferred` lives next to `cursor` and `charPos`

Add `preferred int` to `fileViewer`. Initialised to `0` on file
open. Updated by `fileViewMoveRune` only:

```go
m.fileViewer.preferred = max(m.fileViewer.preferred, m.fileViewer.charPos)
```

`fileViewMoveLine` reads `preferred` to compute the new column:

```go
lineLen := len(m.fileViewer.lines[m.fileViewer.cursor-1])
m.fileViewer.charPos = min(m.fileViewer.preferred, lineLen)
```

`preferred` is held unchanged by `j`/`k`, retreat by `h`, and
never reset except on file open. Rationale: matches vim's
preferred-column rule, which is what users coming from vim
expect. The alternative (reset on every motion) loses the
"remember where I was going" intent.

### 2. `fileSelection` shrinks to `(Active, Mode, LineA, CharA)`

Drop `LineC` and `CharC` from `fileSelection`. The cursor is the
visual selection's tail; `LineC`/`CharC` are derived as
`(cursor, charPos)`. Add `Mode byte` so future visual modes
(line / block) can extend without another shape change; for v0
`Mode` is always `'c'`.

This makes the visual selection rendering trivial: `LineA..LineC`
is `[min(anchor.line, cursor.line), max(...)]`, and on each line
the byte range is `[min(anchor.char, cursor.char), max(...)]`
(with end-of-line clamping when the line is past the cursor's
char endpoint).

### 3. `v` enters char visual, `v` again exits, `Esc` exits

Char visual is the only mode for now. Pressing `v` when inactive
seeds the anchor at the current cursor. Pressing `v` when active
exits and clears. `Esc` follows the existing rule: exit visual
if active, otherwise return to `stateFileNav`.

No `V` keybinding in this pass. A multi-line line-range comment
is reached by `v` + `j`/`k` + `c` (the resulting char selection
collapses to a line-range anchor in the composer open path).

### 4. `c` chooses the anchor from the visual shape

```go
if !visual.Active {
    // whole current line
    anchor = file-line-range(cursor.line, cursor.line)
} else {
    aLine, aChar := visual.LineA, visual.CharA
    cLine, cChar := cursor, charPos
    if aLine == cLine {
        lo, hi := min(aChar, cChar), max(aChar, cChar)
        anchor = file-inline(aLine, lo, hi, excerpt)
    } else {
        lo, hi := min(aLine, cLine), max(aLine, cLine)
        anchor = file-line-range(lo, hi) // char ends dropped
    }
}
```

Multi-line char selection collapses to a line-range anchor
because the comment format has no multi-line inline kind and
adding one is out of scope.

### 5. Inline block cursor replaces the gutter cursor

The file viewer currently marks the cursor's line with a green
`▍` in the left gutter; the byte position within the line is
not rendered. After this change the gutter carries only the
yellow comment marker — no cursor marker. The cursor's exact
position becomes visible via an inline block cursor:

- One cell, painted over the rune at `charPos`.
- Inverted-background style (foreground and background swapped
  vs the file body).
- When `charPos == len(line)`, render a trailing inverted space
  so the cursor still appears at the end of the line.
- When the cursor overlaps an active visual selection, the
  inverted style wins (paints over the cyan selection
  highlight).

Rationale: the cursor's line is already obvious from the
inverted block; a second gutter marker on top is noise. The
inverted-background block cursor is the standard text-editor
visual — vim, less, nano, every code editor — so existing
mental model applies.

The rendering change lives entirely in `renderFileContent`:
drop the `ln == m.fileViewer.cursor` branch from the gutter
switch, and add a `applyCursor(content, line, charPos)` helper
that injects the inverted-style ANSI codes at the byte
position.

### 6. Rune-boundary clamp lives in one helper

Every motion (`h`/`l`/`j`/`k`) routes through
`clampToRuneBoundary(line string, pos int) int`. The helper:

- returns `pos` unchanged when `pos == 0`, `pos == len(line)`,
  or `line[pos]` starts a rune (high bits `0xxxxxxx` or
  `11xxxxxx`);
- when `pos` falls inside a rune (high bits `10xxxxxx`), walks
  backwards to the start of that rune and returns it.

It does not modify the cursor directly — it just returns the
clamped offset. `fileViewMoveLine` calls it after
`min(preferred, len(newLine))`; `fileViewMoveRune` calls it
after each byte-step. Centralising the math means the renderer
and LSP request builder never have to repeat the check, and
the helper has a tight unit-test surface
(`TestClampToRuneBoundary_*`).

Rationale: UTF-8 correctness belongs at the motion layer so
nothing downstream has to think about it. Alternatives
(defensive clamp in `applyCursor` / `applySelection` /
`wordAtCursor`) scatter the same logic and let future callers
miss it.

### 7. `d`/`R`/`K` fire at the word under the cursor

New helper `wordAtCursor() (line, char int)`:

```go
line := m.fileViewer.lines[m.fileViewer.cursor-1]
c := clamp(m.fileViewer.charPos, 0, len(line))
start := c
for start > 0 && isWordByte(line[start-1]) {
    start--
}
return m.fileViewer.cursor, start
```

`isWordByte` is `[A-Za-z0-9_]`. If the cursor is on whitespace
or punctuation the helper returns the cursor's `charPos`
unchanged — this preserves the LSP "0 results" path for things
like `d` on `(`. Rationale: matches what every code LSP wants;
pinky isn't a prose editor, so Unicode identifiers are out.

`requestLSP` becomes:

```go
line, char := m.wordAtCursor()
return issue(ctx, path, line, char)
```

No change to the picker or jump semantics — those already work
on the LSP server's returned `(line, char)`.

## Risks / Trade-offs

- **Spec drift between message view and file view.** The message
  view keeps its `(blockIdx, charPos)` byte-offset model; the
  file view keeps its `(line, charPos)` 2-coord model with a
  `preferred` column. Both are anchor+cursor; the difference is
  the unit of vertical motion (block vs line). Unifying on a
  byte-offset model across both views is cleaner on paper but
  touches every render and jump target — out of scope here.
  → Mitigation: keep both models anchor+cursor-shaped; document
  the difference in `design.md` so a future refactor can converge
  them.

- **`preferred` ignores vertical motion entirely.** After
  scrolling the cursor with `j`/`k`, a `h` reduces `charPos` but
  not `preferred`, so the next `j` jumps back to the previous
  high column. This matches vim but can feel sticky in narrow
  files where the user is bouncing between short and long lines.
  → Mitigation: ship the vim rule; revisit only if users complain.

- **`c` no longer comments the whole file.** Users who relied on
  the one-keystroke whole-file comment lose it. Whole-file
  feedback still reaches the agent via compose (which appends
  comments by default).
  → Mitigation: surface this in the help / README; future
  improvement could add a `cc` double-tap or a per-file "comment
  header" via the dir navigator.

- **Multi-line char selection collapses to line-range on `c`.**
  Users selecting a function body across lines get a line-range
  comment, not an inline excerpt. This is the same trade-off the
  previous spec already made for line-snap visual mode.
  → Mitigation: keep the format change minimal; the line-range
  format is already a first-class anchor kind.

- **Word boundaries are ASCII-only.** Non-ASCII identifiers
  (`Ω`, `λ`) won't be picked up; `d`/`R`/`K` will fire at the
  exact charPos and likely return 0 results.
  → Mitigation: documented limitation; matches Go's default
  identifier set; can extend `isWordByte` to Unicode categories
  later without breaking the spec (the spec says "word under
  the cursor" and the rule can be a coding decision).

## Migration Plan

The change is internal — no schema, no protocol, no public API.
Deploy is the next commit after `tasks.md` is approved:

1. Rewrite `fileViewer` and `fileSelection` shapes in `model.go`
   and any helpers in `internal/render/`.
2. Update `fileViewMoveLine` / `fileViewMoveRune` for the new
   column and visual rules.
3. Add `wordAtCursor` and route `requestLSP` through it.
4. Update `openFileComment` to branch on visual shape.
5. Update README keymap prose.
6. Update tests; the existing `file_test.go` /
   `lspfileview_test.go` are the regression net.

Rollback = revert the commit; no data migration, no client
coordination. LSP `didOpen`/`didClose` are unchanged so existing
language servers keep working.

## Open Questions

None. The two implementation defaults flagged in the explore
session (preferred-column rule and ASCII word set) are the
choices documented under Decisions 1 and 5.
