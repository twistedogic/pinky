# Tasks

## 1. Setup

- [x] 1.1 Add `headerHeight int` field to `model` (next to existing
  `fileRoot`) and add `pendingIdleStyle` / `pendingArmedStyle` package
  vars next to `statusBarStyle`; verify with `go build ./...`
- [x] 1.2 Implement pure `shortenCwd(path string, maxSegs int) string`
  helper next to `padRight`; verify with table-driven test in
  `model_test.go` covering: empty path, single segment, two segments,
  three+ segments, leading slash, already-`~/`-prefixed

## 2. Header render

- [x] 2.1 Implement `func (m *model) headerView() string` returning a
  fully-padded 1- or 2-line string with cwd (left, dim) and pending
  count (right, yellow; bold when `len(m.comments) > 0`); verify with a
  `TestHeader_RendersSingleLineWhenFits` test in `view_test.go` at
  width 80 with `m.fileRoot = "/Users/a012/Dev/pinky"` and
  `m.comments = nil`
- [x] 2.2 Implement `func splitOnSlash(s string, max int) (string,
  string)` greedy-prefix-on-`/` helper; verify with
  `TestSplitOnSlash` covering: shorter than max (no split), single
  segment longer than max (returns original), and a long multi-segment
  path splitting at a `/` boundary that fits

## 3. Wire-up

- [x] 3.1 Modify `View()` to call `m.headerView()` at the top of the 5
  attached-state `JoinVertical` chains (stateNav, stateCompose,
  stateCommentComposer, stateFileNav, stateFileView); verify with
  `TestView_HeaderPresentInAttachedStates` checking that the first
  rendered line of `View().Content` matches the header at width 80
- [x] 3.2 Modify `reflow()` to set `m.headerHeight = strings.Count(
  m.headerView(), "\n") + 1` and subtract it from `vpHeight`; verify
  by running existing `TestView_PadsStatusLineToFullWidth` plus a new
  `TestReflow_ShrinksViewportByHeaderHeight` at width 40 (forces wrap)
  asserting `m.viewport.Height() == 24 - statusHeight - helpHeight -
  2 - composeHeight - 1` for `stateCompose`
- [x] 3.3 Slim `statusLine()`: drop `m.pane` from the text and drop the
  `len(m.comments)` block (now in the header); keep streaming dot, tab
  chip, `[I]` chip in compose, `VISUAL` chip in visual mode; verify
  with existing `TestView_PadsStatusLineToFullWidth` still passing and
  `TestView_StateCompose_ShowsIncludeChip` (new) plus
  `TestStatusLine_SlimmedOmitsPaneAndCommentCount` (new) in
  `view_test.go`

## 4. Verification

- [x] 4.1 Run `task test` (or `go test ./...` if Taskfile has no test
  target) and confirm all existing tests plus the new ones from 1.2,
  2.1, 2.2, 3.1, 3.2 pass; verify the binary builds with
  `go build ./...`
- [ ] 4.2 Manual smoke test: run `go run .` against a real `pi` or
  `codex` agent, accumulate 2 comments, narrow the terminal to 30
  columns, and confirm (a) the header wraps to 2 lines with the cwd
  shortened via `~/…` and (b) the bottom status line no longer
  contains `m.pane` or the comment count; verify by capturing a
  transcript of the rendered output