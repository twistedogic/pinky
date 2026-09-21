## 1. Comment data model

- [ ] 1.1 Add `Comment` type and constants in `internal/render/render.go`
      (`Kind BlockKind`, `BlockIdx int`, `CharStart int`, `CharEnd int`,
      `Source string`, `Text string`, `CreatedAt time.Time`); add a
      `Marker()` helper that returns `▸` for block-level (CharStart < 0)
      and `•` for inline.
- [ ] 1.2 Add `comments []render.Comment`, `visual visualState`,
      `msgHash string`, `includeComments bool` fields to `model` in
      `model.go`.
- [ ] 1.3 Add `stateCommentComposer` to the `state` enum in `model.go`.
- [ ] 1.4 Add a `visualState` type to `model.go` (`mode SelectionMode`,
      `anchor int`, `cursor int`, `charA int`, `charC int`) and
      `SelectionMode` constants (`selNone`, `selLine`).
- [ ] 1.5 Add a `commentHash(text string) string` helper (short FNV or
      `crypto/sha1` first-8-hex) on the model; reset `comments` when
      `msgHash` flips.

## 2. Anchor math and projection

- [ ] 2.1 Add `BlockByteRange(node ast.Node, src []byte, blockIdx int)
      (start, end int)` helper in `internal/render/anchor.go` that
      returns the byte range of the block in the original markdown,
      using the goldmark AST node's `Lines()`.
- [ ] 2.2 Add `LineByteOffset(block Block, node ast.Node, src []byte,
      renderedLineIdx int) int` that maps a rendered line index
      within a block to a byte offset in the source. Accept 1-line
      drift at word-wrap boundaries.
- [ ] 2.3 Add `RenderedLineRange(blocks []Block, blockIdx int, charA,
      charC int) (startLine, endLine int)` that projects a
      `(CharStart, CharEnd)` byte range onto the block's rendered
      line range.
- [ ] 2.4 Write `internal/render/anchor_test.go` covering: byte-range
      of a heading/paragraph/code/list-item; projection of an
      inline byte range onto rendered lines; 1-line drift on a
      wrapping paragraph; off-the-end byte offset returning the
      block's last line.

## 3. Visual selection state machine

- [ ] 3.1 Add `visualState.Handle(key rune, now time.Time) Action` in
      `internal/render/visual.go` (or extend `vim.go` with a visual
      flag) that processes `V`/`j`/`k`/`}`/`{`/`Esc`/`c` and returns
      an action enum (`VimEnterVisual`, `VimExtendCursor`,
      `VimExitVisual`, `VimComposer`).
- [ ] 3.2 Add a `cursorOnScreen()` check on the model: returns true
      when `viewport.YOffset <= visual.cursor <= viewport.YOffset +
      viewport.Height`. Wire into the idle handler so `j`/`k`/`}`/`{`
      in visual mode scroll the viewport when the cursor leaves the
      visible range.
- [ ] 3.3 Write `internal/render/visual_test.go` (or extend
      `vim_test.go`) covering: `V` enters mode and parks cursor;
      `j`/`k` extend cursor; `}`/`{` jump by block; `Esc` exits and
      clears state; `c` returns the composer action with the
      selection range.

## 4. Comment composer state

- [ ] 4.1 Add a `commentComposer` textarea field on the model (or
      reuse the existing `textarea.Model`); initialize on `m` /
      `c` entry.
- [ ] 4.2 Add `handleCommentComposerKey` to `model.go`: `Ctrl+S`
      saves the comment (block-level or inline anchor based on
      `visual.mode`), clears the composer, returns to idle;
      `Esc` cancels.
- [ ] 4.3 Add `enterCommentComposer(anchor commentAnchor)` helper
      that seeds the composer with the right placeholder text and
      focus state.
- [ ] 4.4 Add `viewComposer()` (or inline in `View()`) that renders
      the viewport + textarea + status line in compose-like layout.
- [ ] 4.5 Wire the composer into `handleKey` so `stateCommentComposer`
      is dispatched before `stateIdle`/visual.

## 5. Comment rendering

- [ ] 5.1 Add `RenderMessageWithComments(md string, width int,
      comments []Comment) (string, []Block)` in `internal/render/render.go`
      that calls `renderBlocks` (existing) and then post-processes
      the rendered string to add footnote lines, background tints,
      and gutter markers.
- [ ] 5.2 Add `footnoteLines(blocks []Block, comments []Comment)
      []footnote` helper that returns one footnote per comment with
      `(lineIdx, marker, text)`; footnotes for inline comments
      include the stored source excerpt.
- [ ] 5.3 Add `applyHighlights(rendered string, blocks []Block,
      comments []Comment, width int) string` that rewrites the
      rendered string line-by-line, prepending the gutter marker to
      the first line of each commented block and applying a
      lipgloss background color to lines inside each commented
      block's `StartLine..EndLine` range.
- [ ] 5.4 Add `injectFootnotes(rendered string, footnotes []footnote)
      string` that inserts footnote lines immediately after each
      commented block's `EndLine`, before the next block begins.
- [ ] 5.5 Extend `model.injectBorder` to call `applyHighlights`
      first and `injectFootnotes` second, so borders sit on top of
      highlights and footnote lines.
- [ ] 5.6 Update `model.refreshViewport` to use
      `RenderMessageWithComments` instead of `RenderMessage`.
- [ ] 5.7 Write `internal/render/comment_render_test.go` covering:
      footnote appears below commented block; inline footnote
      includes excerpt; gutter marker on first line; background
      tint applied to all lines in range; border lines themselves
      are not tinted; multiple comments on the same block produce
      multiple footnote lines.

## 6. Block-local edit / delete / navigate

- [ ] 6.1 Add `handleIdleKey` cases for `m`, `V`, `e`, `d`, `n`, `N`:
      `m` enters the composer pre-anchored to the current block;
      `V` enters visual line mode; `e` re-opens the composer with
      the most recent comment's text; `d` removes the most recent
      comment on the current block; `n`/`N` jump to the
      next/previous commented block.
- [ ] 6.2 Add `currentBlockComments() []render.Comment` helper on
      the model that returns the comments for the currently-focused
      block (empty if none).
- [ ] 6.3 Add `nextCommentedBlock(delta int) int` helper that
      returns the index of the next/previous commented block
      relative to the currently-focused block, wrapping around at
      the ends.
- [ ] 6.4 Add `mostRecentComment(blockIdx int) (int, bool)` helper
      that returns the index in `model.comments` of the most recent
      comment for a given block, or `(-1, false)` if none.
- [ ] 6.5 Write `model_test.go` (or extend existing) covering:
      `m` enters composer; `V` enters visual; `e`/`d` on
      commented vs uncommented block; `n`/`N` wrap-around; visual
      cursor off-screen triggers viewport scroll.

## 7. Compose integration (Ctrl+I include flag)

- [ ] 7.1 Add `includeComments bool` field to `model` (already in
      1.2; here we wire it up).
- [ ] 7.2 Add a `handleComposeKey` case for `Ctrl+I` that toggles
      `includeComments` and refreshes the status line.
- [ ] 7.3 Update the compose status line to show
      `[I] include N comments — ON/OFF` when there are any
      comments, or hide the hint when there are zero.
- [ ] 7.4 Update the `handleComposeKey` `Ctrl+S` send path: when
      `includeComments` is true and `len(comments) > 0`, append
      the appendix (`\n\n---\nN comments:\n- ...`) to the redirect
      text before calling `inject.Send`.
- [ ] 7.5 Add `formatCommentsAppendix(comments []render.Comment,
      blocks []Block) string` helper that builds the appendix
      string per the design (chronological order, ~40-char
      excerpt, block or inline format).
- [ ] 7.6 Write tests for `formatCommentsAppendix`: empty list
      returns empty string; single block comment; single inline
      comment; multiple comments in chronological order; excerpt
      truncation with ellipsis.

## 8. Keymap and help overlay

- [ ] 8.1 Add `Mark`, `Visual`, `Composer`, `EditComment`,
      `DeleteComment`, `NextComment`, `PrevComment`,
      `IncludeComments` bindings to `keyMap` in `keymap.go`.
- [ ] 8.2 Update `keymapGroupsForState` in `keymap_md.go` to
      surface the new bindings in the help overlay: a `mark`
      group in idle, a `visual` group in idle, a `comments`
      group in idle, and an `include` line in compose.
- [ ] 8.3 Verify `showHelpMarkdown` still works (it calls
      `render.RenderMessage`, which is unchanged).
- [ ] 8.4 Write `keymap_md_test.go` (or extend existing) covering
      that the new bindings appear in the right keymap groups for
      each state.

## 9. End-to-end smoke test

- [ ] 9.1 Manual verification: `go build`, `go test ./...`, run
      pinky against a fake pi session, exercise `m`, `V`, `c`, `e`,
      `d`, `n`, `N`, `Ctrl+I`, and confirm: footnote renders
      correctly, gutter marker appears, background tint applied,
      border still draws on top, redirect with appendix sends the
      expected payload, comments cleared on new message arrival.
- [ ] 9.2 Run `openspec validate add-block-and-inline-comments` and
      confirm no validation errors.
- [ ] 9.3 Confirm `go vet ./...` and `go test ./...` are clean.
