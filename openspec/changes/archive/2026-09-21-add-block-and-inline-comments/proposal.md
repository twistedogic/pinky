## Why

When reviewing an AI agent's output in pinky, there's no way to flag a
section for later reference or attach a note to it. Users can already
navigate by block (]]/[[) and compose a redirect, but nothing persists
their review intent inside the message itself. A block-level or inline
visual selector with annotation support lets users mark regions while
they read, and optionally carry those notes into the redirect they send
back to the agent.

## What Changes

- **Block-level annotation.** From the idle view, pressing `m` on the
  current block opens a composer pre-anchored to that block. Saving
  stores a comment with anchor `(BlockIdx, -1, -1)`.
- **Inline annotation (line granularity).** From the idle view, pressing
  `V` enters vim-style visual line mode; `j`/`k` extend the selection
  by rendered line, `}`/`{` extend by block. Pressing `c` while in
  visual mode opens a composer pre-anchored to the selection with
  `(BlockIdx, CharStart, CharEnd)` byte offsets into the original
  markdown. `Esc` exits visual mode without saving.
- **Auto-scroll while in visual mode.** When the visual cursor moves
  off-screen via `j`/`k`, the viewport scrolls to keep it visible
  (matches vim behavior).
- **Inline footnote rendering.** Saved comments are rendered as a
  footnote line directly below their block, prefixed with a comment
  marker so the text is distinguishable from the next block's content.
  Comented regions also receive a background tint and a left-gutter
  marker (▸ for block-level, • for inline) on the rendered lines.
- **Edit and delete comments via block-local keys.** When the current
  block has one or more comments, `e` re-opens the composer pre-filled
  with the most recent comment for editing; `d` deletes the most
  recent comment. `n` and `N` jump to the next/previous commented
  block.
- **Include comments in the redirect.** In compose mode, `Ctrl+I`
  toggles whether the queued redirect will be sent with a comments
  appendix. When enabled, the redirect payload gets a trailing block:

  ```
  <redirect text>

  ---
  N comments:
  - block "..." (lines X-Y): <text>
  - inline "..." (line Z): <text>
  ```

- **In-memory storage only.** Comments live on the `model` struct and
  are cleared when the latest assistant message content changes
  (`msgHash` flip) or when the session detaches. No disk writes, no
  history integration.
- **Border/highlight coexistence.** The existing block-border pass
  (`injectBorder` in `model.go`) and the new comment-highlight pass
  share the same line-rewriting loop. Ordering: comments highlighted
  first, block border applied on top.

## Capabilities

### New Capabilities

- `message-comments`: The full block + inline annotation feature,
  including visual selection, comment composer, in-memory storage,
  inline footnote rendering, edit/delete via block-local keys, and
  the optional redirect-include behavior.

### Modified Capabilities

- `latest-message-view`: The "Render assistant text as markdown"
  requirement gains a sibling requirement for rendering comment
  annotations alongside blocks. This is a requirement change (the
  main view now includes content not in the original assistant
  message), so it lives as a delta spec under this change rather
  than a pure implementation detail.

## Impact

**Code touched**

- `internal/render/render.go` — new `Comment` type,
  `RenderMessageWithComments` (renders blocks and overlays comment
  highlights), `ProjectionLines` helper for byte-offset → rendered
  line projection.
- `internal/render/comment_test.go` — new file: anchor math,
  projection drift, border-coexistence.
- `model.go` — new fields (`comments []render.Comment`, `visual
  visualState`, `msgHash string`), new states (`stateCommentComposer`),
  new handlers (`m`, `V`, `c`, `e`, `d`, `n`, `N`, `Esc` in visual),
  `injectBorder` extended with a comment-highlight pass, visual-mode
  auto-scroll.
- `keymap.go` — new bindings: `Mark`, `Visual`, `CommentComposer`,
  `EditComment`, `DeleteComment`, `NextComment`, `PrevComment`,
  `IncludeComments` (Ctrl+I in compose).
- `keymap_md.go` — new keymap groups: `mark`, `visual`, `comments`.

**APIs / public surface**

- `internal/render` exports a new `Comment` type and a new
  `RenderMessageWithComments` function. `RenderMessage` remains for
  the help overlay (which has no comments).
- No CLI flag changes. No new files outside `internal/render/` and
  the change directory itself.

**Dependencies**

- No new Go module dependencies. All work is over the existing
  goldmark AST and bubbletea viewport.
