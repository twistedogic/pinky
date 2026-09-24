# Design

## Context

pinky already extracts the agent pane's `pane_current_path` from
tmux (`internal/session/session.go: paneCwd`) for session
discovery. We have a cwd at attach time; we don't currently
expose it to the user. The comment system (`m.comments`,
`render.FormatCommentsAppendix`, the comment composer in
`stateCommentComposer`) is anchored entirely to message blocks.

We want to:

1. Surface the pane cwd as a browsable tree to the user.
2. Let the user anchor comments to file paths + line ranges.
3. Keep a single flush path (`s`) and a single comment slice.

## Goals / Non-Goals

**Goals:**

- `Tab` toggles between message view and file review tab; the
  toggle works from every state except the composer and the
  picker.
- File review starts in a dir navigator; the user opens a file
  to view its content, can stage comments there, and `Esc`
  returns to the navigator.
- One unified `m.comments` slice with a `Kind` discriminator;
  the appendix format and `s` flush carry over unchanged.
- Gitignore-aware walker; no `git` binary on PATH required.
- Existing tests still pass; new tests cover the new
  behaviours.

**Non-Goals:**

- Editing, diffing, syntax highlighting.
- Watching for file changes (line drift is acceptable — the
  appendix reports the line range the user originally
  selected).
- Persisting comments across detaches.
- Commenting on directories.
- Cross-cutting workspace (multiple roots, symlink following).

## Decisions

### D1. New states: `stateFileNav`, `stateFileView`

The state machine grows two new values:

```
statePicking
stateNav                       (existing — message view)
stateCompose                   (existing)
stateCommentComposer           (existing)
stateFileNav                   (new — dir navigator)
stateFileView                  (new — file viewer)
stateError                     (existing)
```

`Esc` from `stateFileNav` returns to the last message-view
state (nav or compose). `Esc` from `stateFileView` returns to
`stateFileNav`. `Tab` from either new state returns to the
message view (and re-presses Tab from the message view return
to whichever file-review state the user came from).

### D2. Tab toggle remembers the file-review sub-state

`m.tab` is an enum (`tabMessage`, `tabFiles`). When `Tab`
toggles, we record the sub-state we left (`stateFileNav` or
`stateFileView`) on a small `m.fileReturn` slot so toggling
back returns the user to where they were. This keeps Tab
round-trips cheap.

### D3. Workspace walker: pure Go, gitignore via dep

New package `internal/workspace/`:

```
type Entry struct {
    Path  string // relative to root
    IsDir bool
    Depth int
}

func Walk(root string, gi *gitignore.GitIgnore) ([]Entry, error)
```

Implementation:

1. `os.ReadDir(root)` recursive, but skip anything `gi.Match`
   says is ignored.
2. Honour nested `.gitignore` files: when walking into a
   directory, parse its `.gitignore` if present and use the
   combined matcher from then on.
3. Skip `.git` and the workspace root's `.gitignore`-ignored
   paths.
4. Do not follow symlinks (use `os.Lstat`); this prevents
   loops in repos with weird `node_modules`-style content.
5. Sort entries by `(Depth, Path)` so the tree renders
   predictably.
6. Return a flat slice; the renderer (in `model.go`) folds it
   into a tree using `Depth` and `IsDir`.

The gitignore dep is `github.com/sabhiram/go-gitignore`
(a pure-Go parser, MIT, no transitive deps of note; widely
used). `ponytail: switch to an inline parser if the dep ever
feels heavy; gitignore is a small spec.`

### D4. `Comment` grows a `Kind` discriminator

```go
type CommentKind int

const (
    CommentBlock CommentKind = iota // existing
    CommentFile                      // new
)

type Comment struct {
    Kind       CommentKind
    BlockIdx   int       // block-kind only
    CharStart  int       // block-kind byte offset; block-kind inline OR file-kind inline
    CharEnd    int       // block-kind byte offset; block-kind inline OR file-kind inline
    Path       string    // file-kind only, relative to pane cwd
    LineStart  int       // file-kind only, 1-based, inclusive
    LineEnd    int       // file-kind only, 1-based, inclusive
    Source     string    // verbatim slice for inline excerpts (both kinds)
    Text       string
    CreatedAt  time.Time
}
```

The block-kind byte offsets live in `block.Source[charStart:charEnd]`;
the file-kind byte offsets live in the file's raw content
`content[charStart:charEnd]`. Block-kind comments populate the
existing fields and leave `Kind = CommentBlock` (the zero
value, so old comments serialise identically). File-kind
comments set `Kind = CommentFile`, `Path`, `LineStart`,
`LineEnd`, and — when the user made a visual char-range
selection — `CharStart` / `CharEnd`. A line-range file comment
that came from `c` with no selection uses `CharStart ==
CharEnd == -1` as the "no inline selection" sentinel (mirrors
block-kind's whole-block sentinel).

### D5. Unified slice, single flush

`m.comments` stays a single `[]render.Comment`. `s` from any
state flushes the whole slice. `submitAllComments` is
unchanged — `FormatCommentsAppendix` grows a file-kind
branch and handles both shapes.

### D6. Appendix format: file-kind entries

`FormatCommentsAppendix` adds branches in the per-entry loop:

```go
switch c.Kind {
case CommentBlock:
    if inline (c.CharStart >= 0) {
        marker := "inline"        // single line, byte range
    } else {
        marker := "block"         // whole-block, line range
    }
    // excerpt from block.Source or blocks[c.BlockIdx].Source
case CommentFile:
    if inline (c.CharStart >= 0) {
        marker := "file-inline"   // byte range inside file
    } else {
        marker := "file"          // line range inside file
    }
    excerpt := c.Path            // for file/whole-line
    if inline { excerpt = fileContent[c.CharStart:c.CharEnd] }
    // (lines X-Y) from c.LineStart..c.LineEnd
}
b.WriteString("- ")
b.WriteString(marker)
b.WriteString(" \"")
b.WriteString(excerpt)
b.WriteString("\"")
// (lines X-Y) from c.LineStart..c.LineEnd when meaningful
b.WriteString(": ")
b.WriteString(c.Text)
```

A typical mixed appendix looks like:

```
3 comments:
- block "func foo() returns a frob." (lines 12-14): rename to bar
- file "internal/workspace/walker.go" (lines 22-31): handle symlink loops
- file-inline "return err" "internal/foo.go" (line 17): wrap with context
```

### D7. File viewer: raw text + line numbers, gutter-only

The viewer renders the file as raw text prefixed with a
right-aligned 1-based line number column. `RenderFile` lives
in `internal/render/file_render.go`:

```
   1  package workspace
   2  │
   3  import (
   4      "io/fs"
   5      "path/filepath"
   6  )
   7  │
   8  func Walk(root string, gi *gitignore.GitIgnore) []Entry {
```

A yellow `▍` gutter replaces the line-number gutter on lines
that carry a comment (mirroring the message view's yellow
gutter for commented blocks). No footnote lines, no body
markers — the appendix already groups file comments by path.

### D8. Comment composer reused; anchor from caller

The existing `stateCommentComposer` is reused. The caller
fills `m.commentAnchor` with whatever shape the calling state
needs:

- `stateNav` (existing): `commentAnchor{ blockIdx, charA,
  charC }`.
- `stateFileNav`: `commentAnchor{ file: <path>, lineStart:
  1, lineEnd: <fileLines> }` (whole-file comment).
- `stateFileView`: `commentAnchor{ file: <path>, lineStart,
  lineEnd }` for a line-range comment, or
  `{ file, lineStart, lineEnd, charA, charC }` when visual
  mode produced an inline char selection. A whole-file comment
  (no selection) uses `lineStart=1, lineEnd=<fileLines>`.

`saveComment` grows a branch on the anchor shape and writes
the appropriate `Comment` entry. Inline char selections get
`CharStart` / `CharEnd` from the file's raw byte stream; the
verbatim excerpt is stashed in `Comment.Source` so the
appendix can quote it without re-reading the file.

### D9. `c` in dir nav → whole-file comment

Pressing `c` in `stateFileNav` when the cursor is on a file
entry opens the comment composer with a whole-file anchor
(`LineStart = 1`, `LineEnd = fileLines`). On a directory
entry, `c` is a no-op (matches the "directories can't be
commented" non-goal).

### D10. Status bar surfaces tab + comment count

The status bar grows a small `tabLabel` chip ("msg" / "files")
so the user always knows which tab is active. The comment
count from the message view stays as is; when in the file
tab the status line shows the total across both kinds.

### D11. Help footer reflects per-state bindings

`ShortHelp` and `FullHelp` extend the file-nav and file-view
groups with the new bindings. The `Tab` binding is shown in
both the message view and the file tab so the user can see
how to get back.

## Risks / Trade-offs

- **Workspace size**: a `node_modules` or a large monorepo
  could make the walker slow. The gitignore dep handles the
  common case (most repos `.gitignore` their deps), but a
  user with a non-ignored giant tree will see a slow first
  walk. Mitigation: cap walker output at N entries; show a
  truncation notice. (ponytail: ship without the cap; add it
  if anyone reports a slow walk.)
- **Line drift**: if the file changes between commenting and
  flushing, the agent receives a stale line range. This is
  the same trade-off every review tool without a Git backend
  makes; not addressing it in this change.
- **Two comment kinds, one slice**: a future "filter to one
  kind" feature would need a discriminator walk. Trivial, not
  anticipated soon.
- **Bigger help footer**: the file-nav and file-view bindings
  are 4-5 keys each; the expanded help view grows by a row
  each. Acceptable; the short view stays one line per state.
