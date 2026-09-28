# Tasks

## 1. Test fixtures (red)

- [x] 1.1 Add `TestTabHeader_PresentInAttachedStates` in `view_test.go` — sets `m.tab = tabMessage`, runs every attached state (`stateNav`, `stateCompose`, `stateCommentComposer`, `stateFileNav`, `stateFileView`, `stateLSPPicker`) at width 80 / height 24, and asserts `m.View().Content`'s first line contains both `Message` and `Files`. Verify the test compiles and currently FAILS (no `tabHeader()` yet).
- [x] 1.2 Add `TestTabHeader_AbsentInPickerAndError` in `view_test.go` — sets `m.state = statePicking` then `stateError`, asserts the rendered output does NOT contain `Message` or `Files` near the top (no tab row). Verify the test compiles and currently FAILS.
- [x] 1.3 Add `TestTabHeader_ActiveCellSwapsWithMTab` in `view_test.go` — constructs the model at width 80 with two separate fixtures: one with `m.tab = tabMessage`, one with `m.tab = tabFiles`. Both render `stateNav`. Assert the `Message` cell carries the active style (filled background) only in the first fixture, and the `Files` cell carries the active style only in the second. Verify the test compiles and currently FAILS.
- [x] 1.4 Add `TestTabHeader_FoldsIntoHeaderHeight` in `view_test.go` — calls `m.reflow()` with width 80 / height 24 and asserts `m.headerHeight == 2` (tab row + 1-line cwd). Repeats with the cwd forced to a 2-line wrap and asserts `m.headerHeight == 3`. Verify the test compiles and currently FAILS.
- [x] 1.5 Add `TestStatusLine_DropsTabChip` in `view_test.go` — constructs the model in `stateNav` with `m.tab = tabFiles`, calls `m.statusLine()`, asserts the output contains neither `msg` nor `files` as a styled chip (and still contains the streaming dot). Verify the test compiles and currently FAILS the chip part.

## 2. Render methods

- [x] 2.1 Add `activeTabStyle` and `inactiveTabStyle` lipgloss vars to `model.go` near the other chrome styles (same block as `tabChipStyle` and `statusBarStyle`). Verify the variables compile (visible in `go build ./...`).
- [x] 2.2 Add `func (m *model) tabHeader() string` to `model.go`. It SHALL render exactly two cells (" Message " and " Files ") separated by one space, with the cell matching `m.tab` carrying `activeTabStyle` (filled background, fg `232`) and the inactive cell carrying `inactiveTabStyle` (fg `241`, no background). The returned string SHALL have width 10 visible cells minimum (the labels with padding). Verify the new test 1.1, 1.2, 1.3 pass.
- [x] 2.3 Modify `headerView()` to prepend `m.tabHeader()` on every attached state (`stateNav`, `stateCompose`, `stateCommentComposer`, `stateFileNav`, `stateFileView`, `stateLSPPicker`) and skip it on `statePicking` and `stateError`. The existing `m.headerHeight = strings.Count(m.headerView(), "\n") + 1` formula continues to work without change. Verify new test 1.4 passes and existing `TestHeader_RendersSingleLineWhenFits` + `TestView_HeaderPresentInAttachedStates` still pass (after their expected-width updates if needed).

## 3. Cleanup

- [x] 3.1 Remove the `switch m.tab { ... }` block at the bottom of `statusLine()` that appends `tabChipStyle.Render(" files ")` / `tabChipStyle.Render(" msg ")`. Verify `TestStatusLine_DropsTabChip` (1.5) passes.
- [x] 3.2 Delete `tabChipStyle` from `model.go` (no remaining callers after 3.1). Verify `go build ./...` and `grep -n tabChipStyle ./...` return zero hits.

## 4. Verify

- [x] 4.1 Run `task test` (or `go test ./...` from the repo root per `AGENTS.md`). All tests pass. No compile errors.
- [x] 4.2 Run `task lint` (or `go vet ./...`) — clean.
- [ ] 4.3 Manually launch `pinky`, attach to a tmux pane with a pi/codex agent, press `Tab`, confirm the row swaps active style, and confirm the chip is gone from the status bar. Smoke-test only; no automated snapshot assertion in this change.
