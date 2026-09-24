# Tasks

## 1. Add gitignore-aware workspace walker

- [x] 1.1 Add `github.com/sabhiram/go-gitignore` to `go.mod`
      (`go get github.com/sabhiram/go-gitignore`).
- [x] 1.2 Create `internal/workspace/walker.go` with `Entry`,
      `Walk(root, *gitignore.GitIgnore) ([]Entry, error)`. Skip
      `.git`, do not follow symlinks (`os.Lstat`), honour nested
      `.gitignore` files by composing matchers as the walk
      descends.
- [x] 1.3 Create `internal/workspace/walker_test.go`: fixture
      tree with `.gitignore` (basic, negation, nested); assert
      ignored paths are absent and tracked paths appear in
      `(Depth, Path)` order.

## 2. Extend `render.Comment` with kind discriminator

- [x] 2.1 In `internal/render/comment_render.go`: add
      `CommentKind` (`CommentBlock`, `CommentFile`) and the
      `Kind`, `Path`, `LineStart`, `LineEnd` fields on
      `Comment`. Existing block-kind comments use the zero value
      (`CommentBlock`).
- [x] 2.2 Add helper `IsFile(c) bool` and `IsBlock(c) bool`.
- [x] 2.3 Extend `internal/render/comment_appendix.go`'s
      `FormatCommentsAppendix` with a file-kind branch:
      `- file "<path>" (lines X-Y): <text>`. Existing block-kind
      tests must still pass.
- [x] 2.4 Add `TestFormatCommentsAppendix_MixedKinds` in
      `internal/render/comment_appendix_test.go`: one block-kind
      and one file-kind comment, assert the output contains
      both formatted entries and a `2 comments:` header.

## 3. File viewer rendering

- [x] 3.1 Create `internal/render/file_render.go` with
      `RenderFile(path, content string, comments []Comment)
      (string, []LineIndex)` (or equivalent): raw text prefixed
      with right-aligned 1-based line numbers, yellow gutter on
      lines that carry a comment.
- [x] 3.2 Add `TestRenderFile_LineNumbersAndGutter` in
      `internal/render/file_render_test.go`: feed a small
      content string with one commented line, assert the line
      number column and the gutter.

## 4. Tabs in the TUI

- [x] 4.1 In `model.go`: add `m.tab` (`tabMessage`, `tabFiles`)
      and `m.fileReturn` (state to restore on Tab back). Add
      `stateFileNav` and `stateFileView` constants.
- [x] 4.2 Add `m.fileEntries`, `m.fileCursor`,
      `m.fileCollapsed` (map[string]bool for collapsed dirs),
      `m.fileViewer` (path + raw content + cursor).
- [x] 4.3 Implement `m.toggleTab()`: switches `m.tab`, restores
      `m.state` from `m.fileReturn` when going back. Records the
      outgoing state into `m.fileReturn` when leaving.
- [x] 4.4 Add `Tab` to the keymap (`keymap.go`): binding
      `tab` `switch tab`, help text `tab` `switch tab`. Route
      `Tab` in `handleKey` before per-state dispatch; swallow
      Tab in `stateCommentComposer` and `statePicking` and
      `stateError`.
- [x] 4.5 `ShortHelp` and `FullHelp` add the new per-state
      bindings (file nav j/k/h/l/Enter/c/Tab; file view
      j/k/c/Esc/s/Tab).

## 5. Dir navigator

- [x] 5.1 On entering `stateFileNav` (first time after attach):
      call `workspace.Walk(paneCwd, gitignore.CompileDefault)`.
      Stash result in `m.fileEntries`. Build the visible tree
      by walking the entry slice and applying
      `m.fileCollapsed` at each directory boundary.
- [x] 5.2 Implement `handleFileNavKey`:
      - `j`/`k` move the cursor through visible entries
      - `h` collapses the current directory (if expanded) or
        jumps the cursor to the parent entry
      - `l` expands the current directory or jumps to the first
        child
      - `Enter` on a file → load file → enter `stateFileView`
      - `Enter` on a directory → toggle collapse
      - `c` on a file → enter `stateCommentComposer` with a
        whole-file anchor
      - `Esc` / `Tab` → toggleTab back to message view

## 6. File viewer

- [x] 6.1 Implement `handleFileViewKey`:
      - `j`/`k` move the cursor line-by-line
      - `v` enters a visual selection at the cursor (line +
        char byte offset within the line); `h` retreats the
        cursor one rune within the current line, `l` advances
        by one rune; `j`/`k` while visual extends the line
        range (`charA`/`charC` snap to the start/end of the
        destination line)
      - `c` opens the comment composer with anchor from the
        selection: line-range comment when no visual mode,
        inline char-range comment when visual mode is active
        (carries `Path`, `LineStart`, `LineEnd`, `CharStart`,
        `CharEnd`, plus the verbatim `Source` excerpt)
      - `s` calls `submitAllComments`
      - `Esc` returns to `stateFileNav`
      - `Tab` toggles back to message view

## 7. Comment composer: file-kind save path

- [x] 7.1 Extend `commentAnchor` with `kind`, `path`,
      `lineStart`, `lineEnd`, `charA`, `charC`. `charA` /
      `charC` are byte offsets within the file's raw content
      (used only when visual mode was active in
      `stateFileView`).
- [x] 7.2 In `saveComment`: branch on `m.commentAnchor.kind`:
      block-kind path keeps existing logic; file-kind path
      writes a `Comment{Kind: CommentFile, Path, LineStart,
      LineEnd, Text, CreatedAt}`. When `charA >= 0` the path
      also writes `CharStart`, `CharEnd`, and the verbatim
      `Source` excerpt from the file content.
- [x] 7.3 In `buildCommentAnchor` callers (currently
      `stateNav`): add the file-nav (whole-file) and file-view
      (line-range or inline char-range) callers.

## 8. Status bar tab indicator

- [x] 8.1 Add `m.tabLabel()` returning "msg" or "files".
      Render in `statusLine` next to the existing `·`/pane
      line, in dim style.

## 9. Tests

- [x] 9.1 `model_test.go`: `TestTab_TogglesBetweenMessageAndFiles`
      — press Tab from `stateNav`, assert `m.tab == tabFiles`
      and `m.state == stateFileNav`. Press Tab again, assert
      restore to `stateNav`.
- [x] 9.2 `model_test.go`: `TestFileNav_OpenFileOpensViewer`
      — fixture workspace, cursor on a file, press Enter,
      assert `stateFileView` and `m.fileViewer.Path` set.
- [x] 9.3 `model_test.go`: `TestFileView_CommentWholeFile`
      — open file, press `c`, type, press Enter, assert a
      file-kind comment in `m.comments` with
      `Path`, `LineStart=1`, `LineEnd=lineCount`.
- [x] 9.4 `model_test.go`: `TestFileView_CommentSelection`
      — open file, `v`, `j` twice, `c`, type, Enter, assert
      file-kind comment with `LineStart..LineEnd` covering
      the selection (line range, no char range).
- [x] 9.4b `model_test.go`: `TestFileView_CommentInlineCharRange`
      — open file, `v`, `l` three times, `c`, type, Enter,
      assert a file-kind comment with `Kind == CommentFile`,
      `Path` set, `LineStart == LineEnd` (single line), and
      `CharStart`/`CharEnd` covering the three-rune inline
      selection, plus a non-empty `Source` excerpt.
- [x] 9.5 `model_test.go`: `TestUnifiedFlush_SendsAllKinds`
      — stage one block-kind and one file-kind comment, press
      `s`, assert `sendToPane` receives a single payload
      containing both formatted entries.
- [x] 9.6 `model_test.go`: `TestFileNav_COnDirectoryIsNoop`
      — cursor on a dir, press `c`, assert state remains
      `stateFileNav` and `m.comments` is empty.

## 10. README

- [x] 10.1 In `README.md` nav table: add `Tab | Switch to file
      review tab`.
- [x] 10.2 Add a new "File review tab" section listing the dir
      navigator and file viewer keys (j/k, h/l, Enter, Esc, c,
      s, Tab).
- [x] 10.3 Update the "Include comments in redirect" section to
      note file-kind comments are flushed alongside message
      comments, formatted as `- file "<path>" (lines X-Y):`.
- [x] 10.4 Update the compose-mode `i`/`s` description to note
      the appendix now mixes block-kind and file-kind comments.

## 11. Spec archive

- [x] 11.1 Run `openspec validate add-file-review-tab` and
      resolve any flagged issues.
- [ ] 11.2 Run
      `openspec archive --no-validate --yes add-file-review-tab`.
