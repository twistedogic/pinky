# Tasks

## 1. Render primitives

- [x] 1.1 Add `Hit` struct (`LineIdx, ByteA, ByteC int`) and
  `FindHits(query string, lines []string) []Hit` to
  `internal/render`. `FindHits` SHALL walk each line, accumulate
  `runStart` when `query[0]` hits, emit a hit when the full
  query consumes, reset, and return all non-overlapping
  matches in left-to-right order. Empty query returns nil.
  Case-fold both query and line before matching. **Verify:**
  `TestFindHits_Subsequence` (e.g. `the` in `the cat the dog`
  yields two hits), `TestFindHits_NoMatch` (`zzz` in
  `hello world` returns nil), `TestFindHits_EmptyQuery` (no
  matches), `TestFindHits_CaseInsensitive` (`THE` matches
  `the`); all pass.

- [x] 1.2 Replace the file navigator's bool `fuzzyMatch(query,
  name string) bool` with a call into `FindHits` —
  `len(render.FindHits(q, []string{name})) > 0`. Update every
  call site (one location in `model.go:925`) to use the new
  form. **Verify:** existing `file_test.go` tests pass; the
  file navigator's `/` still filters the same entries.

- [x] 1.3 Add `SpliceStyle(line, on string, start, end int)
  string` to `internal/render/cursor.go`. The function SHALL
  walk `line`; when it sees `\x1b`, read to the next
  `[0x40-0x7e]` byte and pass through verbatim; on every
  SGR escape, parse the params and update a local
  `currentBG` (reset on `\x1b[0m` / `\x1b[m`; set to the
  `48;5;N` or `48;2;R;G;B` form when seen). When the byte
  counter reaches `start`, write `on`. When the counter
  reaches `end`, write `currentBG` if non-empty, else write
  `\x1b[49m`. If the end is reached inside an escape sequence
  the splice SHALL be deferred to the next non-escape byte
  (test case: hit ends inside an escape).
  **Verify:** `TestSpliceStyle_NoParentBG` (splice on a plain
  line closes with `\x1b[49m`),
  `TestSpliceStyle_ParentBGPreserved` (splice on a
  glamour-styled line closes with the parent's `48;5;236m`),
  `TestSpliceStyle_ParentResetClears` (splice after a
  `\x1b[0m` closes with `\x1b[49m`),
  `TestSpliceStyle_CombinedSGR` (`38;5;228;48;5;236m` parsed
  correctly), `TestSpliceStyle_TrueColorBG` (`48;2;10;20;30`
  remembered and restored); all pass.

- [x] 1.4 Add `SpliceStyleAcrossWrap(rendered string, lines
  []string, h Hit, on string, wrapWidth int) string` to
  `internal/render`. The function SHALL call
  `wrapLineWithRanges(lines[h.LineIdx], wrapWidth)`, find each
  sub-range that overlaps `[h.ByteA, h.ByteC)`, locate the
  corresponding rendered row via
  `sourceToFirst[h.LineIdx] + subIdx`, and call `SpliceStyle`
  on that row's `[subByteA, subByteC)` slice. **Verify:**
  `TestSpliceStyleAcrossWrap_HitOnShortLine` (sub-line == the
  source line, single splice call),
  `TestSpliceStyleAcrossWrap_HitCrossesWrapBoundary` (hit
  split across two visible rows, each row gets the correct
  sub-range), `TestSpliceStyleAcrossWrap_HitInsideCodeBlock`
  (parent bg 236 preserved on both rows); all pass.

## 2. searchState and prompt handlers

- [x] 2.1 Add `searchState` struct (`active bool; query []rune;
  hits []render.Hit; cur int`) and two model fields
  `navSearch`, `fileSearch` of that type in `model.go`. Add
  `enterNavSearch`, `enterFileSearch` (set `active=true`,
  `query=nil`, `hits=nil`, `cur=-1`). **Verify:** `go build
  ./...` succeeds; new fields compile.

- [x] 2.2 Add `handleNavSearchKey` and `handleFileSearchKey`
  in `model.go`. Each routes printable runes to `query` (via
  `append` on `[]rune`), `Backspace` trims the last rune, `Esc`
  sets `active=false, query=nil, hits=nil, cur=-1` and
  returns, `Enter` calls `commitNavSearch` /
  `commitFileSearch`. Arrow keys fall through to the viewport
  (no `if isEsc` or `if KeyEnter` for those). Other keys
  return without state change.
  **Verify:** `TestNavSearch_TypingAppendsRunes`,
  `TestNavSearch_BackspaceTrims`, `TestNavSearch_EscClears`,
  `TestFileSearch_TypingAppendsRunes`,
  `TestFileSearch_BackspaceTrims`, `TestFileSearch_EscClears`;
  all pass.

- [x] 2.3 Add `commitNavSearch` and `commitFileSearch`. Each
  SHALL set `active=false`, recompute `hits` from the current
  `query`, find the first hit at or after the current
  cursor's `(lineIdx, charPos)` / `(cursor, charPos)`, set
  `cur` to that index (or `-1` if no hits), and move the
  cursor to that hit's `(lineIdx, byteA)` / `(cursor, byteA)`.
  In `stateFileView`, `commitFileSearch` SHALL also reset
  `m.fileViewer.preferred` to the hit's `byteA`. After the
  cursor move, call the existing `scrollCursorIntoView` (nav)
  or the existing `refreshFileView` (file view) so the new
  position is rendered.
  **Verify:** `TestNavSearch_EnterJumpsToFirstHit`,
  `TestNavSearch_EnterWithNoMatchesNoOp`,
  `TestFileSearch_EnterJumpsToFirstHitAndResetsPreferred`,
  `TestFileSearch_EnterWithNoMatchesNoOp`; all pass.

- [x] 2.4 Add `cycleSearchHit(delta int, hits []render.Hit,
  cur *int)` helper (or two thin wrappers around it) that
  sets `*cur = ((*cur + delta) + len(hits)) % len(hits)`. If
  `len(hits) == 0` the helper is a no-op. **Verify:**
  `TestCycleSearchHit_WrapsForward`, `TestCycleSearchHit_WrapsBack`,
  `TestCycleSearchHit_EmptyHitsNoOp`; all pass.

## 3. Wire into stateNav

- [x] 3.1 Route `/` in `handleNavKey` to `enterNavSearch` when
  the cursor is not in visual mode. Add the conditional `n`
  handling: when `m.navSearch.active || len(m.navSearch.hits)
  > 0`, `n` calls `cycleSearchHit(+1, m.navSearch.hits,
  &m.navSearch.cur)`, moves the cursor to that hit's
  `(lineIdx, byteA)`, calls `scrollCursorIntoView` and
  `refreshViewport`, and returns. The unconditional `n` →
  compose path remains when no search is active. Add `N` for
  the reverse cycle. **Verify:**
  `TestNavSearch_NCyclesFromActiveSearch`,
  `TestNavSearch_NCyclesAfterCommit`,
  `TestNavSearch_NStillEntersComposeWhenNoSearchActive`,
  `TestNavSearch_NCyclesWrapsForward`,
  `TestNavSearch_NCyclesWrapsBack`; all pass.

- [x] 3.2 In `refreshViewport`, after `LineGutter` and before
  `ApplyInlineCursor`, walk `m.navSearch.hits`; for every
  index `i` where `i != m.navSearch.cur`, call
  `render.SpliceStyleAcrossWrap(guttered, m.lines, hit,
  "\x1b[48;5;58m", wrapWidth)`. The cursor's
  `ApplyInlineCursor` call runs unchanged after this pass, so
  its `\x1b[7m` paints over the dim background on the current
  hit. **Verify:**
  `TestNavSearch_RefreshAppliesDimToNonCurrentHits`,
  `TestNavSearch_RefreshPreservesCodeBlockBackground`,
  `TestNavSearch_RefreshClearsDimOnEsc`; all pass.

- [x] 3.3 In the message view's `View()`, when
  `m.navSearch.active` is true, render the prompt as
  `/<query>▏` on a single line above the help footer. The
  half-block cursor `▏` is dim-styled. When the prompt is
  closed, no prompt row is rendered.
  **Verify:** `TestNavSearch_PromptRendersWhileActive`,
  `TestNavSearch_PromptHiddenWhenInactive`; both pass.

- [x] 3.4 In `handleSessionMsg` (or whichever handler resets
  state on a new message), when `m.latest.Text` changes,
  clear `m.navSearch` (set `active=false, query=nil, hits=nil,
  cur=-1`). **Verify:**
  `TestNavSearch_NewMessageClearsSearch`; passes.

## 4. Wire into stateFileView

- [x] 4.1 Route `/` in `handleFileViewKey` to
  `enterFileSearch` (unconditional — file viewer has no
  visual-mode collision). Add `n` and `N`: when
  `m.fileSearch.active || len(m.fileSearch.hits) > 0`, `n`
  calls `cycleSearchHit(+1, m.fileSearch.hits,
  &m.fileSearch.cur)`, moves the cursor to that hit's
  `(cursor, byteA)`, resets `m.fileViewer.preferred = byteA`,
  and calls `refreshFileView`. `N` is the same with `-1`.
  **Verify:** `TestFileSearch_NCyclesAndResetsPreferred`,
  `TestFileSearch_NCyclesWrapsForward`,
  `TestFileSearch_NCyclesWrapsBack`; all pass.

- [x] 4.2 In `renderFileContent`, inside the per-line loop,
  after `applySelection` and before `ApplyCursor`, look up the
  first hit on the current line whose `LineIdx` is not
  `m.fileSearch.cur`; if present, call
  `render.SpliceStyle(rendered, "\x1b[48;5;58m", hit.ByteA,
  hit.ByteC)` on the line's body. The cursor's `ApplyCursor`
  call runs unchanged after this pass, so its `\x1b[7m`
  paints over the dim background on the current hit.
  **Verify:** `TestFileSearch_DimOnNonCurrentHits`,
  `TestFileSearch_MultipleHitsOnOneLine`,
  `TestFileSearch_DimOverlapsSelection`,
  `TestFileSearch_DimClearsOnEsc`; all pass.

- [x] 4.3 In the file viewer's `View()`, when
  `m.fileSearch.active` is true, render the prompt as
  `/<query>▏` on a single line above the help footer (same
  shape as the message-view prompt).
  **Verify:** `TestFileSearch_PromptRendersWhileActive`,
  `TestFileSearch_PromptHiddenWhenInactive`; both pass.

- [x] 4.4 In `exitFileViewer` and in the file-open path of
  the dir navigator, clear `m.fileSearch` (set
  `active=false, query=nil, hits=nil, cur=-1`).
  **Verify:** `TestFileSearch_ExitClearsSearch`,
  `TestFileSearch_NewFileClearsSearch`; both pass.

## 5. Keymap and help overlay

- [x] 5.1 Add `NavSearch` (`/`), `NavSearchNext` (`n`),
  `NavSearchPrev` (`N`) bindings to `keyMap` in `keymap.go`.
  Add the same three under `FileViewSearch`,
  `FileViewSearchNext`, `FileViewSearchPrev`. Update the help
  overlay rendering to add one row per new binding per state.
  **Verify:** existing help-overlay tests pass with the
  expanded rows; `TestHelp_IncludesSearchKeys` (new) checks
  the new bindings are present in the rendered help.

## 6. Docs and full integration

- [x] 6.1 Update `README.md`: add `/`, `n`, `N` to the
  message-view key table; add `/`, `n`, `N` to the
  file-viewer key table; add a one-paragraph note on the
  dim-highlight behaviour and the parent-background
  preservation. **Verify:** render the README locally and
  confirm no line contradicts the spec scenarios.

- [x] 6.2 Run `task check` (vet + tests) and resolve any
  failures. **Verify:** `task check` exits 0.

- [x] 6.3 Manual smoke: open pinky, search a phrase in a long
  message; confirm highlights render, `n`/`N` cycle, and
  `Esc` clears; switch to the file tab, open a `.go` file
  with a fenced code block in another tab, search a function
  name inside the block, confirm the code-block background
  is preserved on the cells surrounding the hit; switch back
  to the message view and confirm the search state there is
  independent. **Verify:** the four flows above behave as
  described in the spec scenarios. *(Manual smoke deferred
  if no `pi` session is available in the apply environment
  — the model-level tests above cover the same flows via
  `m.Update`.)*
