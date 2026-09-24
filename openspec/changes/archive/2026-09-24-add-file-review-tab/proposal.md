# File review tab with workspace-wide comments

## Why

Today pinky only lets the user comment on the rendered blocks of
the agent's latest message. A user who wants to push back on a
specific function in a specific source file has no way to attach
the feedback to the file — they have to copy the snippet into a
message comment and hope the agent correlates the prose back to
the file.

A workspace-aware view closes that loop: the user can browse the
agent's cwd (with `.gitignore` respected), open a file, mark up a
line range, and ship all comments — message and file alike — in a
single `s` flush.

## What Changes

- **Tab toggle**: pressing `Tab` switches between the existing
  message view (`stateNav`) and a new file review tab. The tab
  state lives on `m.tab` and survives within an attached session.
- **File review tab** has its own sub-state machine: a **dir
  navigator** (`stateFileNav`) and a **file viewer**
  (`stateFileView`). Esc walks back up: file viewer → dir
  navigator → message view.
- **Workspace walker**: a recursive directory walk rooted at
  the agent pane's `pane_current_path`, honouring `.gitignore`
  via `sabhiram/go-gitignore`. Trailing newline and BOM
  tolerated; symlinks not followed (avoid loops).
- **Dir navigator** renders the walker output as a tree:
  `j`/`k` move the cursor, `h`/`l` collapse/expand directories,
  `Enter` on a file opens the file viewer, `c` on a file
  anchors a whole-file comment, `Tab` and `Esc` exit back to
  the message view, `s` flushes all comments.
- **File viewer** renders the file as raw text with line
  numbers and a yellow `▍` gutter on every line that carries a
  comment. `j`/`k` move by line, `v` enters visual selection,
  `h`/`l` extend the selection by one rune on the current line
  (inline char range), `c` anchors a comment to the selection
  (or the whole file when nothing is selected), `Esc` returns
  to the dir navigator, `s` flushes all comments.
- **Unified comment slice**: `m.comments` becomes a single
  slice whose entries carry a `Kind` discriminator:
  `Block` (the existing block-anchored comment) and `File`
  (the new path + line-range comment). `s` from any state
  flushes the whole slice.
- **Comment shape extended**: `render.Comment` gains `Kind`,
  `Path`, `LineStart`, `LineEnd`, `CharStart`, `CharEnd` fields.
  `CharStart` / `CharEnd` are byte offsets into the file's raw
  content (one combined byte stream, not per-line offsets),
  used for inline char selections made via visual mode.
  Block-kind comments leave those new fields zero-valued;
  line-range file comments leave `CharStart == CharEnd == -1`
  as a sentinel for "no inline selection".
- **Appendix format extended**: `FormatCommentsAppendix`
  emits one line per file comment of the form
  `- file "<path>" (lines X-Y): <text>`. Block comments keep
  their existing `block "<excerpt>" (lines X-Y):` shape. The
  body is sorted chronologically (oldest first) regardless of
  kind, so the agent reads a single ordered list.

## Capabilities

### New Capabilities

- `workspace-files`: the file review tab — Tab toggle, dir
  navigator, file viewer, gitignore-aware walker, comment
  composition against a file anchor.

### Modified Capabilities

- `message-comments`: `Comment` gains `Kind`, `Path`,
  `LineStart`, `LineEnd`; `FormatCommentsAppendix` formats
  file-kind comments as `- file "<path>" (lines X-Y):`; the
  unified slice and the existing `s`-flush path carry over to
  file-kind comments unchanged.

## Impact

- `go.mod`: add `github.com/sabhiram/go-gitignore` (gitignore
  parser used by the workspace walker).
- `internal/workspace/` (new package): recursive walker that
  takes a `gitignore.GitIgnore` + a root, returns a sorted slice
  of `WorkspaceEntry { Path string; Dir bool; Depth int }`. No
  I/O outside the root; tests cover dotfiles, nested
  `.gitignore`, negation patterns, symlink loops.
- `internal/render/comment.go` (new): `CommentKind` constants,
  `Kind`, `Path`, `LineStart`, `LineEnd`, `CharStart`, `CharEnd`
  on `Comment`, helpers `IsFile`, `IsBlock`,
  `IsInlineSelection`. `FormatCommentsAppendix` grows a file-
  kind branch and an inline-file branch.
- `internal/render/file_render.go` (new): `RenderFile(path,
  string) string` returning the file content prefixed with
  right-aligned 1-based line numbers, gutter-aware.
- `model.go`: `m.tab`, `m.fileEntries`, `m.fileCursor`, `m.fileCollapsed`,
  `m.fileViewer` (path + lines + cursor), `m.comments` becomes
  `[]render.Comment` (already is, but the `BlockIdx` field is
  now only meaningful for block-kind). New states
  `stateFileNav`, `stateFileView`. `handleKey` routes Tab
  globally when in nav/file-nav/file-view; the comment composer
  state swallows Tab (textarea behaviour unchanged).
- `keymap.go`: `Tab` binding (help: `tab` `switch tab`); per-
  state bindings for `FileNavUp/Down/Collapse/Expand/OpenFile`,
  `FileViewUp/Down/Visual/Comment`, plus the existing `NavSend`
  reused for file flush.
- `view.go`-equivalent (currently `View()` in `model.go`):
  `viewFileNav`, `viewFileView`, and a `tabLabel` shown in the
  status bar so the user always knows which tab is active.
- `README.md`: new tab key, dir navigator keys, file viewer
  keys; updated compose-mode description to mention file
  comments flush alongside message comments.
- Tests: walker unit tests against a fixture tree; `FormatCommentsAppendix`
  mixed-kind test; `model_test.go` exercises the tab toggle,
  dir navigator j/k/h/l/Enter/Esc, file viewer c→Enter→s,
  and the unified flush.

## Non-Goals

- Editing files or showing diffs (this is a review surface, not
  a code surface).
- Files outside `pane_current_path` (no `~/` expansion, no
  absolute paths).
- File watcher / live reload — comments stay valid as long as
  line numbers hold; if the file changes between comment and
  send, that's the user's problem (same drift risk as a code
  review tool with no Git integration).
- Syntax highlighting (matches the existing "raw markdown"
  choice in the message view).
- Comment persistence across detaches (existing in-memory-only
  invariant applies to file comments too).
