# Tasks

## 1. Baseline

- [ ] 1.1 Confirm the repo builds and all tests pass before any
  edits; verify by running `task check` and reading its exit
  code

## 2. Add powernap dependency

- [ ] 2.1 Add `github.com/charmbracelet/x/powernap` to `go.mod`
  via `go get github.com/charmbracelet/x/powernap@<pinned>` and
  pin the version explicitly; verify by reading `go.mod`
- [ ] 2.2 Run `task tidy` and confirm `go.sum` is updated with no
  unrelated churn; verify by `git diff go.sum` showing only
  powernap-related lines
- [ ] 2.3 Run `task check` to confirm the new dependency
  compiles and existing tests still pass; verify by exit code 0

## 3. Column cursor in `fileViewer`

- [ ] 3.1 Add `charPos int` to the `fileViewer` struct in
  `model.go`; initialise to `0` in `openFileViewer`; verify by
  reading the diff
- [ ] 3.2 Update `fileViewMoveLine(delta)` so that after
  adjusting the line cursor, `charPos` is clamped to
  `len(m.fileViewer.lines[m.fileViewer.cursor-1])`; preserve the
  existing visual-mode extension logic; verify by reading the
  diff
- [ ] 3.3 Add a `fileViewMoveRune(delta)` helper (or inline the
  rune move in `handleFileViewKey`) that adjusts `charPos` by
  the rune byte length and clamps to `[0, len(currentLine)]`;
  preserve visual-mode `CharC` tracking; verify by reading the
  diff
- [ ] 3.4 In `handleFileViewKey`, replace the existing
  `h`/`l`-as-visual-only branches with branches that always
  call `fileViewMoveRune`; preserve the visual-mode extension
  inside the helper; verify by reading the diff
- [ ] 3.5 Add `model_test.go` cases:
  `TestFileViewer_ColumnCursor_ClampsAtLineEnd`,
  `TestFileViewer_ColumnCursor_PreservedAcrossLineMove`,
  `TestFileViewer_ColumnCursor_PreservesVisualRange`;
  verify by `go test -run 'TestFileViewer_ColumnCursor'
  ./...` passing
- [ ] 3.6 Run `task check`; verify by exit code 0

## 4. Keymap additions

- [ ] 4.1 Add `FileViewDefinition`, `FileViewReferences`,
  `FileViewHover` fields to `keyMap` in `keymap.go` with
  `key.WithKeys("d")`, `key.WithKeys("R")`, `key.WithKeys("K")`
  respectively and short help text; verify by reading the diff
- [ ] 4.2 Update the `defaultKeyMap` singleton with the three
  new bindings; verify by reading the diff
- [ ] 4.3 Run `go build ./...` and `task vet`; verify by exit
  codes 0

## 5. `internal/lsp` package — types

- [ ] 5.1 Create `internal/lsp/types.go` with pinky-facing
  `Location`, `Request`, `Result`, `Hover`, `LocationsMsg`,
  `HoverMsg` types; verify by `go build ./internal/lsp/...`
  exiting 0
- [ ] 5.2 Add `internal/lsp/types_test.go` with a small
  round-trip test for the request ID generation; verify by
  `go test ./internal/lsp/...` passing

## 6. `internal/lsp` package — Manager

- [ ] 6.1 Create `internal/lsp/manager.go` with a `Manager`
  struct holding `clients map[string]*clientEntry`,
  `unavailable map[string]time.Time`, `workDir string`,
  `mu sync.Mutex`, and `inflight map[int64]chan Result`;
  verify by reading the diff
- [ ] 6.2 Implement `New(workDir string) *Manager` that loads
  powernap's default server registry via
  `powernapconfig.NewManager().LoadDefaults()`; verify by
  reading the diff
- [ ] 6.3 Implement `Start(ctx, path string) (chan Result, error)`
  that detects the language, looks up the server, lazily spawns
  it (mark unavailable for 30 s on `exec.LookPath` miss), sends
  `didOpen` for the path, and returns a reply channel; verify
  by reading the diff
- [ ] 6.4 Implement `FindDefinition(ctx, uri, line, char)`,
  `FindReferences(ctx, uri, line, char)`, `Hover(ctx, uri, line,
  char)` wrappers that delegate to `powernap.Client` and write
  the reply to the request's reply channel; verify by reading
  the diff
- [ ] 6.5 Implement `Shutdown(ctx)` that closes every open
  file's `didClose`, calls `powernap.Client.Shutdown` and
  `Exit`, and clears the client map; verify by reading the
  diff
- [ ] 6.6 Add `internal/lsp/manager_test.go` covering:
  missing-server → 30 s unavailable; same-server-twice →
  second call returns the existing client (no respawn);
  shutdown is idempotent; verify by
  `go test ./internal/lsp/...` passing

## 7. `internal/lsp` package — read-only handlers

- [ ] 7.1 In `internal/lsp/manager.go`'s client-creation path,
  register a `workspace/applyEdit` handler that returns
  `protocol.ApplyWorkspaceEditResult{Applied: false,
  FailureReason: "pinky is read-only"}`; verify by reading the
  diff
- [ ] 7.2 Register no-op or log-only handlers for
  `workspace/configuration`, `client/registerCapability`,
  `window/workDoneProgress/create`; verify by reading the diff
- [ ] 7.3 Register a `textDocument/publishDiagnostics` handler
  that logs via `slog` and discards the diagnostics; verify by
  reading the diff

## 8. Async bridge — `chan → tea.Cmd`

- [ ] 8.1 Create `internal/lsp/bridge.go` with
  `RequestLocations(m *Manager, uri, line, char, kind) tea.Cmd`
  that returns a `tea.Cmd` whose message is `LocationsMsg` (or
  `HoverMsg` for hover); verify by reading the diff
- [ ] 8.2 Add request ID generation: every request gets an
  incrementing ID; the bridge matches replies to the
  originating `chan` by ID; verify by reading the diff
- [ ] 8.3 Add cancellation: if a second request arrives while
  the first is in flight, the first reply is dropped (the
  channel is closed without delivering); verify by reading the
  diff
- [ ] 8.4 Add `internal/lsp/bridge_test.go` covering: chan
  receives the reply once; superseded requests are not
  delivered; verify by `go test ./internal/lsp/...` passing

## 9. Generalise the picker

- [ ] 9.1 Factor the existing session picker into a generic
  `pickItems[T any](items []T, label string, render func(int, T)
  string, onSelect func(T))` helper inside `model.go` (or
  extracted to `internal/picker` if it earns its own package);
  verify by reading the diff
- [ ] 9.2 Rewrite the session picker to call `pickItems` over
  `[]session.AgentSession`; verify by reading the diff
- [ ] 9.3 Add `stateLSPPicker` to `model.go`'s state machine
  with `lspPickerLocations []Location`,
  `lspPickerLabel string`, `lspPickerCursor int`; verify by
  reading the diff
- [ ] 9.4 Add `handleLSPPickerKey(msg)` routing `j`/`k`,
  `Enter`, `Esc`, `q` per the picker keymap; on `Enter`, call
  `m.jumpToLocation(lspPickerLocations[cursor])`; verify by
  reading the diff
- [ ] 9.5 Add `model_test.go` cases:
  `TestLSPPicker_JumpsOnSameFile`,
  `TestLSPPicker_OpensNewFileOnCrossFile`,
  `TestLSPPicker_EscDismissesWithoutJump`; verify by
  `go test -run 'TestLSPPicker' ./...` passing

## 10. Definition / References / Hover handlers

- [ ] 10.1 In `handleFileViewKey`, route `d` to
  `RequestLocations(m.lsp, m.fileViewer.path, cursor, charPos,
  KindDefinition)` returning a `tea.Cmd`; verify by reading the
  diff
- [ ] 10.2 Route `R` to `RequestLocations(...,
  KindReferences)`; verify by reading the diff
- [ ] 10.3 Route `K` (via `key.Matches` against
  `defaultKeyMap.FileViewHover`) to `RequestHover(m.lsp,
  ...)`; verify by reading the diff
- [ ] 10.4 In `Update`, add `LocationsMsg` and `HoverMsg` cases
  that switch on `len` for definition (silent / jump / picker)
  and references (silent / picker), and that set
  `m.hoverFooter` for hover; verify by reading the diff
- [ ] 10.5 Implement `m.jumpToLocation(loc Location)` that
  branches on same-URI vs cross-URI per the spec; verify by
  reading the diff
- [ ] 10.6 Add `model_test.go` cases:
  `TestFileView_Definition_OneResult_Jumps`,
  `TestFileView_Definition_NResults_Picker`,
  `TestFileView_Definition_ZeroResults_Silent`,
  `TestFileView_References_AlwaysPicker`,
  `TestFileView_Hover_SetsFooter`,
  `TestFileView_Hover_TruncatesMultiLine`; verify by
  `go test -run 'TestFileView_(Definition|References|Hover)'
  ./...` passing

## 11. Hover footer rendering

- [ ] 11.1 Add `m.hoverFooter string` to the model; clear it
  whenever `handleFileViewKey` processes a non-hover key
  (except `?` for the help overlay); verify by reading the
  diff
- [ ] 11.2 In `refreshFileView`'s render path, insert a
  one-line row containing `m.hoverFooter` (or an empty row if
  empty) immediately above the help line; verify by reading
  the diff
- [ ] 11.3 Truncate `m.hoverFooter` to one line at the first
  newline, appending `…` if truncation occurred; verify by
  reading the diff
- [ ] 11.4 Add `model_test.go` case
  `TestFileView_HoverFooter_DismissedByOtherKey`; verify by
  `go test -run TestFileView_HoverFooter ./...` passing

## 12. Per-file `didOpen` / `didClose`

- [ ] 12.1 In `openFileViewer(path)`, after loading the file
  content, call `m.lsp.DidOpen(path, content, lang)`; verify by
  reading the diff
- [ ] 12.2 In the file-viewer exit paths (Esc to dir nav, Tab
  to message view, `q` quit), call `m.lsp.DidClose(path)`
  before transitioning; verify by reading the diff
- [ ] 12.3 Add `model_test.go` cases
  `TestFileView_DidOpenOnEnter`, `TestFileView_DidCloseOnEsc`
  that mock the LSP manager and assert the notifications fire
  in the right order; verify by
  `go test -run 'TestFileView_Did(Open|Close)' ./...` passing

## 13. Missing-server hint

- [ ] 13.1 Add `m.missingServerHint string` to the model; when
  the LSP manager returns an `ErrServerMissing`, the bridge
  sets this string to the install hint; verify by reading the
  diff
- [ ] 13.2 Render the hint in the file viewer (same row as the
  hover footer, but a separate style) and obey the 30 s quiet
  window; verify by reading the diff
- [ ] 13.3 Add `model_test.go` case
  `TestFileView_MissingServerHint_ShownOnce`; verify by
  `go test -run TestFileView_MissingServerHint ./...` passing

## 14. Verification

- [ ] 14.1 Run `task check` and confirm every test passes
  (existing workspace-files scenarios + new LSP scenarios);
  verify by reading the test output
- [ ] 14.2 Run `go build ./...` and confirm the binary
  compiles cleanly; verify by exit code 0
- [ ] 14.3 Run `task vet` and confirm no new vet diagnostics;
  verify by exit code 0
- [ ] 14.4 Manual smoke test against a real Go project: open
  a `.go` file, press `d` on a function call (jumps to
  definition), press `R` on a function (picker), press `K` on
  a parameter (hover footer), press `d` on whitespace
  (silent); verify by running the binary and observing the
  behaviour
- [ ] 14.5 Manual smoke test for the read-only guarantee:
  configure a mock LSP server that sends
  `workspace/applyEdit` and confirm pinky replies
  `applied: false`; verify by reading the server's log