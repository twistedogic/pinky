# Design

## Context

The message view (`stateNav`) renders the latest assistant message
as raw markdown (today — the spec captures that, and the design
adds a search affordance without changing the rendering itself).
The file viewer (`stateFileView`) renders the raw file content,
line by line, with a yellow `▍` gutter on comment-touched lines,
a cyan visual-selection highlight, and an inverted-block cursor at
`(cursor, charPos)`. The file navigator (`stateFileNav`) already
implements `/` as a fuzzy filename filter — same matcher, same
key, same prompt-then-Enter-commit pattern, but the matcher
operates on a flat list of `workspace.Entry` paths and there is no
highlight or `n`/`N` cycle. The new feature generalises that
pattern to operate on the visible content of the two text views
and adds highlighting and cycling.

The glamour stylesheet in `internal/render/markdown.go` paints
fenced code blocks with a background colour (`48;5;236`). When a
search hit lands inside a code block, a naive background splice
breaks the row's background continuity on either side of the hit.
The design addresses this by walking the existing ANSI escape
stack to the left of the hit and restoring the parent background
on close. This is the "option (b)" path discussed during explore;
it generalises correctly to every glamour style that sets a
background (today: code blocks; future: blockquotes, list markers,
table headers).

## Goals / Non-Goals

**Goals:**

- Add `/` fuzzy search to the message view (`stateNav`) and the
  file viewer (`stateFileView`), reusing the existing
  subsequence matcher and the existing file-navigator `/` UX.
- Highlight every match on screen with a background colour
  (`\x1b[48;5;58m`).
- Make the current match visible via the cursor's existing
  inverted-block style — `Enter` and `n`/`N` move the cursor to
  the match byte, so "current match" is the cursor's position.
- Preserve the parent background of any glamour-styled region
  (code blocks today) on the cells surrounding a hit.
- Cycle with `n`/`N` (wrap-around). Reset the current-match
  index on `j`/`k`/`h`/`l` so the next `n` continues from the
  new cursor position.
- Reset the file viewer's `preferred` column on every
  search-driven cursor move.

**Non-Goals:**

- Regex search. Subsequence match only.
- fzf-style match list / picker. Single-line prompt like the
  file navigator.
- Replacing or changing the existing file-navigator `/`. That
  surface keeps its own `m.fileSearch` state and its bool matcher
  call.
- A "previous search" / persistent highlight mode. Highlights
  exist only while a search is active; `Esc` clears them.
- Forward slash variant (`?` for back-search). `n`/`N` cycle in
  one direction each, no third key.
- Incremental cursor movement as the user types. The cursor
  stays where it is during the prompt; `Enter` commits and
  jumps. The highlights DO update as the user types (they are
  recomputed on every render) but the cursor does not.
- SGR 24-bit truecolor. The stylesheet today only emits 8-bit
  indexed colours (`38;5;N` and `48;5;N`); the parser only
  needs to handle the indexed form. Future-proofing for `48;2;R;
  G;B` is ~3 extra lines and deferred.
- Tracking foreground (not just background) across a hit. A hit
  that spans an in-line style change will see the local fg
  change. The cost of tracking fg is ~2× the parser work; the
  real-world impact is invisible on any message a human has
  written. Defer.

## Decisions

### 1. `Hit` is a `(lineIdx, byteA, byteC)` triple in source space

```go
type Hit struct {
    LineIdx int
    ByteA   int // start byte offset into lines[LineIdx]
    ByteC   int // end byte offset (exclusive)
}

func FindHits(query string, lines []string) []Hit
```

`FindHits` walks each line, accumulates a `runStart` when
`query[0]` hits, emits a hit when the full query consumes, and
resets. All non-overlapping matches in left-to-right order. Empty
query returns nil. The file navigator's bool `fuzzyMatch` becomes
`len(FindHits(q, []string{name})) > 0` — same matcher, no parallel
implementation.

`Hit` lives in `internal/render` (next to `SpliceInvert`) because
it is purely a function of the source lines and is consumed by
the renderer's splice helpers. The model imports it for state.

### 2. Two `searchState` instances, no shared abstraction yet

```go
type searchState struct {
    active bool    // prompt is open
    query  []rune  // current query while prompt is open
    hits   []render.Hit // computed every render when active || query != ""
    cur    int     // index of "current" hit in hits; -1 if none
}
```

`m.navSearch` and `m.fileSearch` are two instances. They share
zero state. The behaviours are the same (open prompt, type,
Enter commits, n/N cycle, Esc cancels) but the surrounding model
fields and render integration differ, so a single shared
abstraction would just be two structs in a trench coat. Ponytail:
two near-identical pairs, ~30 lines each, copy-paste with
careful naming; refactor to one shared type if a third view ever
needs search.

### 3. Cursor is the current match

`Enter` (commit) and `n`/`N` (cycle) all set the cursor's
`(LineIdx, CharPos)` to `hits[cur].(LineIdx, ByteA)`. The cursor's
existing `\x1b[7m` inverted-block style paints that byte, so the
"current" highlight is free. `m.navSearch.cur` is the source of
truth; the dim splice skips the index at `cur`.

On `j`/`k`/`h`/`l` the cursor moves and `cur` becomes -1 (no
current match). The next `n` finds the first hit at or after the
new cursor position. This is vim's "search anchor resets on
motion" rule and it falls out naturally.

In the file viewer, every search-driven cursor move resets
`preferred` to the hit's `ByteA`. The user's "where I was going"
intent is meaningless when the cursor is teleported by `n`.

### 4. Dim background is `\x1b[48;5;58m` (dark teal)

Picked from the existing palette plus safe extensions. Distinct
from every existing background (228 gutter is foreground; 236
code block is dark grey; 51 cyan is foreground in the help
header). Readable with any foreground the markdown renderer
might emit. Falls back to `\x1b[48;5;240m` (dark grey) if 58
reads too saturated in practice — a one-line change in the splice
call site.

### 5. `SpliceStyle` restores the parent background

```go
// SpliceStyle layers a style (typically \x1b[48;5;58m) around
// line[start:end]. While walking the string, it tracks the most
// recent SGR background set. On close, it restores that
// background (or \x1b[49m to default if no parent bg was set).
// ANSI escapes inside line are passed through verbatim and not
// split.
func SpliceStyle(line, on string, start, end int) string
```

The walker is the same shape as `SpliceInvert` in
`internal/render/cursor.go`: scan forward, when you see `\x1b`,
read to the next `[0x40-0x7e]` byte (the `m` terminator), pass
through verbatim. The new piece is: parse the escape's params and
update a local `currentBG` whenever you see `48;5;N` or `48;2;R;
G; B` (or a reset clears it). On splice close, emit the remembered
`currentBG` instead of a hardcoded `\x1b[49m`.

The SGR parser is ~15 lines: split on `;`, recognise `48`
followed by `5` (8-bit indexed) or `2` (truecolor — parsed but
unused today), update `currentBG`. Glamour's output for the
code-block style is `\x1b[38;5;228;48;5;236m` — the parser must
handle `38;5;X;48;5;Y` in a single escape and pick up `Y` for
the bg. This is the only non-trivial SGR shape; the rest of the
stylesheet is single-purpose escapes that fit the same parser.

### 6. `SpliceStyleAcrossWrap` for the message view

The message view's rendered string is the result of
`LineGutter` + `ApplyInlineCursor`, both of which respect
`wrapWidth`. A hit on a long line may span multiple visible
rows. `SpliceStyleAcrossWrap` walks `wrapLineWithRanges(line,
wrapWidth)` for the hit's source line, finds each sub-range that
overlaps `[ByteA, ByteC)`, and calls `SpliceStyle` on the
corresponding rendered row (mapped via `sourceToFirst[LineIdx] +
subIdx`).

~10 lines. Mirrors the wrap walk in `ApplyInlineCursor`. Used
once per non-current hit per render — total work is
`O(hits × wrappedSubLines)`, typically a handful of operations.

### 7. `n`/`N` are conditional in `stateNav`, unconditional in `stateFileView`

In `stateNav`, `n` currently means "enter compose mode". The new
binding `n` (next hit) fires only when `m.navSearch.active ||
len(m.navSearch.hits) > 0`. When no search is active, `n` keeps
its old meaning. This is the cleanest way to add the new key
without a breaking change to the existing compose key. The help
overlay documents both meanings.

In `stateFileView`, `n` is unused today — it goes straight to
`handleFileViewKey` with no match, the same as an unknown rune.
`n` and `N` are bound unconditionally for search cycling.

### 8. Prompt rendering mirrors the file navigator

The prompt is a one-line footer above the help bar, only visible
while `searchState.active`. Format:

```
/<query>▏       (query is dim, cursor is the half-block)
```

`Esc` clears; backspace trims; printable runes append. The
existing `enterFileSearch` and `handleFileNavSearchKey` are the
reference; the two new prompt handlers (`enterNavSearch` and
`enterFileSearch` / `handleNavSearchKey` and
`handleFileSearchKey`) follow the same shape with the renamed
fields.

### 9. Re-render hooks: `refreshViewport` and `renderFileContent`

The two render functions gain a "dim splice" pass after their
existing styling and before the cursor splice. The cursor splice
runs LAST, so its `\x1b[7m` paints over the dim background on the
current hit. The order is:

```
message view:
  LineGutter
  → SpliceStyleAcrossWrap for each non-current hit
  → ApplyInlineCursor (current hit, if hits exist)

file view:
  per-line:
    applySelection
    → SpliceStyle for any non-current hit on this line
    → ApplyCursor (current hit on this line, if any)
```

The cursor is unaware of search; it operates on its existing
`(LineIdx, CharPos)` model. The dim splice is unaware of the
cursor; it operates on a known hit list. The two never
coordinate except by render order.

## Risks / Trade-offs

- **SGR parser bugs on unusual escapes.** The parser handles
  `48;5;N`, `48;2;R;G;B`, and combined escapes like
  `38;5;X;48;5;Y`. If a future stylesheet produces a
  non-standard escape, the parser falls back to "no remembered
  bg" and the splice closes with `\x1b[49m`, which is the
  option-(a) behaviour for that one cell. → Mitigation:
  exhaustive unit tests on the parser (`TestSGR_*`); document
  the stylesheet's expected SGR surface in the test fixtures.

- **`n` collision in `stateNav` with compose.** The new `n` is
  conditional on search state, so it's a behavioural shift only
  while a search is active. Users who don't search see no
  change. → Mitigation: help overlay documents both meanings;
  alternative would be to bind next-hit to a different key,
  but `n` is the vim answer and the user explicitly asked for
  vim-like behaviour.

- **Hit positions are recomputed on every render.** A
  re-render triggered by viewport scroll (which does happen on
  `j`/`k`/`h`/`l` in the message view) recomputes the hit list
  even when the query is unchanged. Cost is O(L · |query|) per
  re-render, where L is the number of source lines (≤ a few
  hundred for a single message). → Mitigation: the cost is
  trivial; cache only if profiling shows it matters.

- **Wrap-aware hit splice in the message view is the most
  error-prone code path.** A hit that crosses a wrap boundary
  must be split across two visible rows, with each row getting
  the correct byte range within its sub-line. Off-by-one in
  the sub-line range arithmetic will mis-highlight. →
  Mitigation: mandatory test with a long line + a hit that
  crosses the wrap; the test asserts both the hit bytes are
  highlighted and the surrounding code-block background is
  preserved (per risk 1).

- **Multi-line hit interpretation.** `FindHits` is per-line; a
  match that crosses a newline is not produced. This is the
  right behaviour for both views: a query that spans lines
  would highlight two separate lines, and the user is unlikely
  to mean "match across the line break" anyway. If a future
  user wants cross-line matches, the change is to walk
  `strings.Join(lines, "\n")` instead of per-line — out of
  scope here. → Mitigation: documented limitation; revisit if
  asked.

- **Highlights reset on every new message or new file open.**
  The user's search state is ephemeral; if they switch files or
  a new agent message lands, the hits list and the prompt are
  cleared. This matches vim's `:bufdo` semantics. → Mitigation:
  documented; consider a `:keepsearch` later if the workflow
  is common.

- **Background colour is not user-configurable.** The 58
  teal is a hardcoded constant. The user's terminal theme can
  make it look wrong (e.g. on a teal background). → Mitigation:
  fall back to 240 if 58 reads as too saturated; punt config to
  a later change when there's evidence it's wanted.

## Migration Plan

The change is internal — no schema, no protocol, no public API,
no migration of persistent state. Deploy is the next commit after
`tasks.md` is approved:

1. Add `Hit`, `FindHits`, `SpliceStyle`, `SpliceStyleAcrossWrap`
   in `internal/render`. Unit tests for the SGR parser and the
   wrap-aware splice.
2. Add `searchState` and the two model fields; add the prompt
   handlers and the `n`/`N` cycle helper. Wire into
   `handleNavKey` and `handleFileViewKey`.
3. Wire the dim splice into `refreshViewport` and
   `renderFileContent`.
4. Add the keymap bindings and the help rows.
5. Update `README.md`.
6. Run `task check` (vet + tests) and resolve any failures.
7. Manual smoke: open pinky, search a phrase in a long message
   and confirm highlights + cycle; open a `.go` file with a
   fenced code block in another tab, search a function name
   inside the block, confirm the code-block background is
   preserved around the hit; `Esc` and confirm clean restore.

Rollback = revert the commit. No data migration, no client
coordination. The session source, tmux bridge, LSP bridge,
history, and inject paths are all untouched.

## Open Questions

None. The implementation defaults flagged during explore
(subsequence match, `\x1b[48;5;58m`, walk-the-stack SGR
restoration, conditional `n` in `stateNav`, two
near-identical `searchState` instances) are the choices
documented under Decisions 1-9.
