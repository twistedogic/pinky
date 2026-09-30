# Tasks

## 1. Cursor model rewrite

- [x] 1.1 Reshape `render.NavCursor` to `(LineIdx int, CharPos int, Preferred int)` and `render.NavSelection` to `(ByteA int, ByteC int)`; verify `go build ./...` succeeds
- [x] 1.2 Add a `lineStartOffsets []int` table on the model mapping each `lineIdx` to its starting byte offset in `m.latest.Text`; verify `byteOffset(lineIdx, charPos)` returns the correct offset in a unit test for empty / single-line / multi-line messages
- [x] 1.3 Rewrite `render.NavHandle` for source-line motion with the preferred-column rule (`charPos = min(preferred, len(newLine))` on `j`/`k`, `preferred = max(preferred, charPos)` on `l`, `preferred` unchanged on `h`); verify `internal/render/nav_test.go` passes

## 2. Comment anchor reshape

- [x] 2.1 Reshape `render.Comment` to drop `BlockIdx`, `CharStart`, `CharEnd` and add `ByteA`, `ByteC` used by both `CommentMessage` and `CommentFile` kinds; rename `CommentBlock` constant to `CommentMessage`; verify `go build ./...` succeeds
- [x] 2.2 Update `model.buildCommentAnchor` and the `model.commentAnchor` struct to compute the anchor as `(byteA, byteC)` over `m.latest.Text` for message-kind (whole current source line when no visual, visual byte range when in visual) and to copy `(byteA, byteC, path, lineStart, lineEnd)` for file-kind; verify the existing `buildCommentAnchor` unit tests pass with the new fields

## 3. Block-derived code removal

- [x] 3.1 Remove `m.blocks`, the `Block` / `BlockKind` types, and the markdown AST parser call from `model.go` and `internal/render/render.go`; verify `go build ./...` succeeds and no `m.blocks` reference remains in the codebase
- [x] 3.2 Remove `render.RenderMessageWithComments` and `render.InjectGutterWrapped`; verify no callers remain (`rg "InjectGutterWrapped|RenderMessageWithComments"` returns nothing in non-test code)

## 4. Yellow line-direct gutter

- [x] 4.1 Implement `render.LineGutter(text string, comments []Comment, wrapWidth int) (string, []int)` that walks the source lines (split on `\n`), checks each source line `[byteStart, byteEnd)` for overlap with any saved comment's `ByteA < byteEnd && ByteC > byteStart`, emits a yellow `▍` (228) gutter per wrapped sub-line on overlap (or a space otherwise), and returns the gutter-prefixed body plus a `wrappedToSrc` slice; verify a unit test for a no-comment message (all space gutters), a single-line comment (one line yellow), a multi-line comment (every touched line yellow), and a non-overlapping comment (no change)
- [x] 4.2 Wire `LineGutter` into the message view's render path in `model.View` and `model.refreshViewport`, replacing the old block-derived gutter injection; verify in an integration test that a comment on line 3 of a 5-line message produces a yellow `▍` on line 3 and spaces on lines 1, 2, 4, 5

## 5. Inline cursor rendering

- [x] 5.1 Lift `model.applyCursor` and `model.spliceInvert` to the `render` package (new `internal/render/cursor.go` exposes `ApplyCursor`, `SpliceInvert`, plus the wrap-aware helpers `wrapLineWithRanges` and `ApplyInlineCursor`); verify the file viewer's cursor still renders correctly (existing `file_test.go` / `lspfileview_test.go` tests pass unchanged)
- [x] 5.2 Apply the lifted `applyCursor` in the message view at the cursor's `(lineIdx, charPos)` on the rendered line that contains `lineIdx`; verify in an integration test that the byte at `charPos` is painted with the inverted-background style and that this takes precedence over an active visual selection's cyan highlight at the same cell — wired in `model.refreshViewport` after `LineGutter`; the wrap-aware mapping (`wrapLineWithRanges`) finds the wrapped sub-line containing `charPos` and applies the cursor to its body

## 6. Scroll keys

- [x] 6.1 Bind `↑`, `↓`, `PageUp`, `PageDown`, `Home`, `End` in `stateNav` to viewport-only movement (`viewport.LineUp`, `viewport.LineDown`, `viewport.HalfViewUp`, `viewport.HalfViewDown`, `viewport.GotoTop`, `viewport.GotoBottom`) without touching the cursor's `(LineIdx, CharPos, Preferred)`; update `keymap.go` `stateNavKeyMap` to include the scroll bindings and the help text; verify in a unit test that pressing `↓` advances `YOffset` by one and leaves the cursor unchanged, that pressing `Home` sets `YOffset = 0` and leaves the cursor unchanged, and that `j` after a scroll-key re-engages viewport-follows-cursor (the viewport scrolls to keep the cursor visible on the next motion key) — implemented via the existing "forward unhandled special keys to viewport" branch in `handleNavKey`; the keys already route to the viewport without mutating the cursor

## 7. Appendix format

- [x] 7.1 Rewrite `render.FormatCommentsAppendix(comments []Comment) string` to drop the count line and split into two sections (`Comments on the message:` then `Comments on files:`); emit message-kind entries as `- comment on "<excerpt>": <text>` and file-kind entries as `- file "<path>" (lines X-Y): <text>` or `- file-inline "<excerpt>" (line Z): <text>`; emit a single blank line between sections; collapse empty sections (two empty sections → single blank line); verify `internal/render/comment_appendix_test.go` passes for: empty slice → empty string, single message-kind entry, single file-kind entry, mixed-kind slice, all-message slice (no files section), all-file slice (no message section) — `TestUnifiedFlush_SendsAllKinds` in `file_test.go` updated to check for the new section labels instead of the old `2 comments:` count line
- [x] 7.2 Implement the excerpt rules for message-kind entries: truncate verbatim `text[ByteA:ByteC]` to 40 characters with `…` appended if longer, replace internal `\n` with single space, collapse runs of whitespace to single space, trim leading / trailing whitespace; verify in `comment_appendix_test.go` scenarios for: under-40-char excerpt (no truncation), over-40-char excerpt (truncates with `…`), excerpt with embedded newlines (newlines → space), excerpt with leading / trailing whitespace (trimmed)

## 8. Tests and docs

- [x] 8.1 Update `internal/render/nav_test.go` for the new cursor / selection / line-direct model; verify `go test ./internal/render/...` passes
- [x] 8.2 Update `model_test.go`, `view_test.go`, `file_test.go`, `lspfileview_test.go`, `file_motion_test.go`, `help_test.go`, `picker_test.go` for the new cursor, keymap, gutter, and appendix; verify `go test ./...` passes
- [x] 8.3 Update `README.md` keymap tables (nav) and the comments-appendix example block to show the new `Comments on the message:` / `Comments on files:` split with the `comment on "<excerpt>"` format and the new scroll-key rows; verify the README example matches the output of `FormatCommentsAppendix` for a representative comment set
- [x] 8.4 Run `task` (Taskfile.yml); verify build + lint + tests all pass