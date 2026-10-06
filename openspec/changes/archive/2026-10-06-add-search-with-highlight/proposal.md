# Proposal

## Why

The message view and the file viewer both surface long-form text the
user often wants to scan for a specific word or phrase — the
markdown of the latest agent message and the contents of a file
under review. Today there is no in-view search: the user either
eyes-balls the text, scrolls with `j`/`k`/`PageDown`, or jumps via
`d`/`R`/`K` (file viewer only) and lands somewhere unrelated. The
file navigator already has a fuzzy filename filter on `/`, so the
keystroke is a known affordance — but it operates on directory
entries, not on visible content. Adding `/` to the two content
views closes that gap and brings pinky's review surface to parity
with vim, less, and every modern TUI editor.

## What Changes

- Add a `/` prompt to `stateNav` (last message) and `stateFileView`
  (file content). The prompt collects a query; on `Enter`, the
  cursor jumps to the first match and the prompt closes; `n`/`N`
  cycle through subsequent matches while the prompt stays closed;
  `Esc` cancels and clears the highlights.
- Match using the same subsequence matcher the file navigator
  already uses (`fuzzyMatch` in `model.go`) — every rune of the
  query appears in the source in order, case-insensitive. All
  non-overlapping left-to-right matches on each line are returned
  as `(lineIdx, byteA, byteC)`.
- Highlight every match on screen with a background colour
  (`\x1b[48;5;58m`, dark teal). The current match is rendered
  through the cursor's existing inverted-block style — the cursor
  moves to the current match on commit and on each `n`/`N`, so
  the "current match" is just "where the cursor is". Non-current
  matches are dim.
- The background splice walks the existing ANSI escape stack to
  the left of each match and restores the parent background after
  the splice closes. This preserves glamour's code-block
  background (228-on-236) on the cells surrounding a hit inside
  a fenced code block — option (b) in the design discussion.
- New `Hit` type and `FindHits` helper in `internal/render`
  alongside the existing `SpliceInvert`. New `SpliceStyle` and
  `SpliceStyleAcrossWrap` helpers do the ANSI-aware splice.
- `n`/`N` are bound in both `stateNav` and `stateFileView`. In
  `stateNav`, `n` previously meant "enter compose mode"; the new
  binding is conditional on `search.active` (compose still fires
  when no search is active). In `stateFileView`, `n` is unused
  today.
- `n`/`N` reset the file viewer's `preferred` column tracker
  because the cursor lands on an arbitrary byte offset, not the
  user's last-intended column.
- The existing file-navigator `/` is unchanged.

## Capabilities

### New Capabilities

None. The behaviour is a per-view addition that fits inside the
two existing capabilities below.

### Modified Capabilities

- `latest-message-view`: adds a "Fuzzy search and highlight" set
  of requirements covering `/`, `Enter`, `Esc`, `n`/`N`, the
  dim-background highlight, the cursor-as-current-match
  behaviour, and the parent-background preservation when a hit
  falls inside a glamour-styled region.
- `workspace-files`: adds the matching requirements for the
  file viewer, with one variance — the file viewer's hit
  positions are in `(line, charPos)` rather than the message
  viewer's `(lineIdx, charPos)`, and `n`/`N` reset the
  `preferred` column tracker.

## Impact

- `internal/render/cursor.go` (or new `search.go` in the same
  package):
  - `Hit` struct: `LineIdx, ByteA, ByteC int`.
  - `FindHits(query string, lines []string) []Hit`.
  - `SpliceStyle(line, on string, start, end int) string` —
    ANSI-aware, restores parent background on close.
  - `SpliceStyleAcrossWrap(rendered string, lines []string, h
    Hit, on string, wrapWidth int) string` — wrap-aware
    variant for the message view.
- `model.go`:
  - New `searchState` struct (`active, query, hits, cur`) in
    the model, two instances (`navSearch`, `fileSearch`).
  - `handleNavKey` routes `/` to `enterNavSearch`, the prompt
    to `handleNavSearchKey`, and `n`/`N` to a new
    `cycleSearchHit(+1 / -1)` helper when `navSearch.active ||
    len(navSearch.hits) > 0`. `n` still means "enter compose"
    when no search is active.
  - `handleFileViewKey` gets the same `/`, prompt, and `n`/`N`
    additions.
  - `refreshViewport` calls `SpliceStyleAcrossWrap` for every
    non-current hit after `LineGutter` and before
    `ApplyInlineCursor`.
  - `renderFileContent` calls `SpliceStyle` for every
    non-current hit on the current line, after `applySelection`
    and before `ApplyCursor`.
  - New `enterNavSearch`, `enterFileSearch`,
    `handleNavSearchKey`, `handleFileSearchKey`,
    `commitNavSearch`, `commitFileSearch`, `cycleSearchHit`.
  - `SpliceInvert`'s `Cursor`/`Inline` callers are unchanged —
    the new splice runs first, the cursor runs after, so the
    cursor's `\x1b[7m` paints over the dim background.
- `keymap.go`:
  - `NavSearch`, `NavSearchNext`, `NavSearchPrev`,
    `FileViewSearch`, `FileViewSearchNext`,
    `FileViewSearchPrev` bindings.
  - Help overlay rows for the new bindings in both states.
- `README.md`: add `/` and `n`/`N` to the message-view key
  table; add `/` and `n`/`N` to the file-viewer key table; add
  a one-paragraph note on the dim-highlight behaviour and the
  parent-background preservation.
- Tests: new `search_test.go` covering `FindHits`,
  `SpliceStyle` (with ANSI preservation), the wrap-aware
  splice (with a long line that wraps past the hit),
  `enterSearch`, `cycleSearchHit` (with wrap-around),
  `commitSearch` (jumps cursor to first hit), `Esc` clears,
  `j`/`k` reset the current-hit index, and the file-viewer
  `n`/`N` reset `preferred`.
- No new dependencies. No API or protocol change. The session
  source, tmux bridge, LSP bridge, history, and inject paths
  are all untouched.
