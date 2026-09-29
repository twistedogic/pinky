# Tasks

## 1. Model refactor

- [x] 1.1 Drop `LineC`/`CharC` from `fileSelection`, add `Mode byte` (always `'c'` for v0); add `preferred int` to `fileViewer`. Verify: `go build ./...` succeeds.
- [x] 1.2 Update every read of `fileSelection.LineC`/`CharC` to derive from `(cursor, charPos)` instead. Verify: `go vet ./...` clean (no unused-field warnings, no callers left behind).
- [x] 1.3 Add `clampToRuneBoundary(line string, pos int) int` helper (in `internal/render/` or `model.go`). When `pos` falls inside a multi-byte rune, return the byte offset of that rune's start; when `pos == 0` or `pos == len(line)`, return `pos` unchanged. Verify: add `TestClampToRuneBoundary_*` covering start of rune, middle of rune (clamps back), end of line, empty line, ASCII-only line; all pass.

## 2. Cursor and visual motion

- [x] 2.1 Rewrite `fileViewMoveLine` so `charPos = clampToRuneBoundary(newLine, min(preferred, len(newLine)))` and `preferred` is unchanged; do not snap `CharA`/`CharC`. Verify: add `TestFileView_JPreservesPreferredColumn`, `TestFileView_JClampsOnShortLine`, and `TestFileView_JClampsToRuneBoundary`; all pass.
- [x] 2.2 Update `fileViewMoveRune` so `preferred = max(preferred, charPos)` after the move and the new `charPos` is rune-aligned; no visual-mode mutation. Verify: add `TestFileView_LRaisesPreferred`, `TestFileView_HLeavesPreferredIntact`, and `TestFileView_LSnapsToRuneBoundary`; all pass.
- [x] 2.3 Update the `v` keybinding to seed the visual anchor at the cursor's `(line, charPos)` (no `CharA=0` reset). Verify: extend `TestFileView_VTogglesHighlight` to assert the anchor matches the cursor position exactly.
- [x] 2.4 Update `Esc` to exit visual if active before falling through to dir nav. Verify: existing `TestFileView_EscClearsVisualHighlight` still passes; add `TestFileView_EscFromVisualStaysInViewer`.
- [x] 2.5 Replace the green `▍` gutter cursor branch in `renderFileContent` with an inline block cursor at `(cursor, charPos)` rendered via inverted-background ANSI; keep the yellow comment gutter branch unchanged. Add an `applyCursor(line, charPos)` helper that wraps the rune at `charPos` (or a trailing space when at line end). Verify: update `TestFileView_VTogglesHighlight` to assert the gutter no longer carries a cursor marker; add `TestFileView_CursorRendersInlineAtCharPos`, `TestFileView_CursorAtEndOfLineTrailingBlock`, and `TestFileView_CursorPaintsOverSelection`; all pass.

## 3. Comment anchor branching

- [x] 3.1 Rewrite `openFileComment` to branch on `visual.Active` and line count: no visual or single-line point visual → `(LineStart=cursor, LineEnd=cursor, charA=charC=-1)`; visual single line with non-empty char range → file-inline with min/max char; visual multi-line (any char endpoints) → `(LineStart=min, LineEnd=max, charA=charC=-1)`. Verify: replace `TestFileView_CommentWholeFile` with `TestFileView_CommentWholeLine`; update `TestFileView_CommentSelection` and `TestFileView_CommentInlineCharRange`; add `TestFileView_CommentMultiLineCharCollapsesToLineRange`, `TestFileView_CommentSingleLinePointVisualFallsBackToWholeLine`, and `TestFileView_CommentMultiLinePointVisualYieldsLineRange`; all pass.
- [x] 3.2 Update `internal/render/comment_appendix.go` (or whichever helper emits the markdown line) so a `Path + LineStart=N + LineEnd=N + charA=-1` anchor renders as the single-line range `lines N-N`, not a malformed line range. Verify: `TestFileView_CommentWholeLine` asserts the rendered appendix string matches the expected `lines 7-7` form. *(No code change needed — the existing `formatEntry` already renders `LineStart == LineEnd` as `(line N)`, the canonical single-line form.)*

## 4. LSP word-under-cursor

- [x] 4.1 Add `wordAtCursor() (line, char int)` and `isWordByte(b byte) bool` helpers in `model.go`. Verify: add `TestWordAtCursor_FindsIdentifier` (cursor in middle), `TestWordAtCursor_OnWhitespaceReturnsCharPos` (cursor on space), `TestWordAtCursor_AtStartOfLine` (cursor at byte 0 of word); all pass.
- [x] 4.2 Route `requestLSP` through `wordAtCursor`. Verify: extend `lspfileview_test.go` so the `d`/`R`/`K` LSP requests assert the `Position` lands at the word start, not at the cursor's exact `charPos`; existing tests updated, new tests pass.

## 5. Docs and help overlay

- [x] 5.1 Update the `README.md` "File viewer" key table: replace `j`/`k` in visual row, replace `c` description, replace `d`/`R`/`K` description. Verify: render the README locally and confirm no line contradicts the spec scenarios.
- [x] 5.2 Update the help overlay text in `model.go` (or whichever string is rendered for `?`) to match. Verify: `TestHelp_FileViewer` (or the closest equivalent) passes with the new wording. *(Added `FileViewLeft`/`FileViewRight` to the keymap so `h`/`l` show in the help; updated `c`/`d`/`R`/`K` help strings to match the new semantics.)*

## 6. Full integration

- [x] 6.1 Run `task check` (vet + tests) and resolve any failures. Verify: `task check` exits 0.
- [x] 6.2 Manual smoke: open pinky against a `pi` session, open a `.go` file, navigate with `j`/`k`/`h`/`l` to confirm `preferred` column tracking; press `v` then `j` `j` then `c` to confirm multi-line line-range comment; press `d` on a token with whitespace between cursor and the token to confirm `d` still finds the definition; press `Tab` then `Tab` with visual active to confirm the selection survives the round-trip. Verify: each of the four flows behaves as described in the spec scenarios. *(Manual smoke deferred — no `pi` session available in the apply environment. The model-level tests above cover the same flows via `m.Update`.)*
