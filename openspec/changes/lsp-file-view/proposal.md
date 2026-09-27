# Proposal

## Why

The file viewer in `stateFileView` lets the user open a file from the
agent pane's working directory, browse it line-by-line, and stage
anchored comments — but the only way to navigate within or across files
is `j`/`k`/`h`/`l` plus the file tree. There is no way to ask "where
is this symbol defined?" or "where is it used elsewhere?" without
leaving pinky and grepping the codebase by hand. That round-trip
breaks the loop pinky is designed for: stay alongside the agent, push
back precisely, never context-switch.

The fix is to wire a Language Server Protocol (LSP) client into the
file viewer. LSP is the right interface for this — every modern editor
uses it, every common language ships a server, and a read-only slice
(definition / references / hover) is the natural fit for pinky's
observation-only stance. We do not need to write a JSON-RPC stack
ourselves: `github.com/charmbracelet/x/powernap` provides one, and
pinky's needs reduce to a thin manager layer, three method wrappers,
and a generalised picker.

The interaction is read-only. Pinky never edits a file, never calls
`workspace/applyEdit`, never lets the server apply edits to its
buffer. The server-side `workspace/applyEdit` request is rejected
unconditionally with `Applied: false`. This makes the change a
strict addition to the user-facing surface and a strict refusal at
the protocol layer.

## What Changes

- **Column cursor in the file viewer.** `fileViewer` gains a
  `charPos int` field. `j`/`k` move the line; `h`/`l` move the
  column; in visual mode `h`/`l` continue to extend the visual range
  exactly as today (charC snaps at the line end). The cursor's column
  is the position the LSP client uses for queries.
- **Three new keys in `stateFileView`.**
  - `d` — `textDocument/definition`. 0 results: silent no-op.
    1 result: jump (same-file → cursor + scroll; cross-file → close
    current viewer + open new file at the result's line/char).
    N>1 results: picker.
  - `R` (Shift+R) — `textDocument/references`. 0 results: silent
    no-op. N≥1 results: picker. The picker is the same component
    used by `d` for its N>1 case.
  - `K` (Shift+K) — `textDocument/hover`. 0 results: silent no-op.
    1+ results: render a one-line hover footer in the file viewer.
    Multi-line content is truncated to one line.
- **New shared picker state.** A new `stateLSPPicker` handles
  multi-location selection for both `d` (N>1) and `R` (N≥1). The
  picker is a thin generalisation of the existing session picker:
  it accepts `[]Location` with `(URI, line, char, snippet)` tuples
  and shows `<relpath>:<line>:<col>  <snippet>`. `Enter` selects,
  `Esc` dismisses, `q` quits pinky.
- **Hover footer.** When `K` resolves, the file viewer grows a
  one-line footer between the file body and the help line. Pressing
  any other key (including `K` again to refetch at a new cursor
  position) dismisses it.
- **LSP manager and async bridge.** A new `internal/lsp` package
  wraps `charm.land/x/powernap`. The manager lazily spawns servers
  on first file open, marks missing servers as unavailable for 30 s
  before retrying, and exposes `FindDefinition`, `FindReferences`,
  `Hover`, `DidOpen`, `DidClose`, `Shutdown`. Responses arrive
  asynchronously and are delivered to Bubble Tea via `chan → Msg`.
- **Read-only enforcement.** The powernap client's
  `workspace/applyEdit` handler is registered to return
  `Applied: false, FailureReason: "pinky is read-only"`. Any other
  server-initiated write requests (`workspace/executeCommand`,
  `client/registerCapability` for file-watching) are likewise
  declined or ignored.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `workspace-files`: the file viewer gains a column cursor, three
  new keys (`d` / `R` / `K`), a shared location picker, a hover
  footer, and an LSP lifecycle that auto-spawns language servers
  on first file open. Requirements for "file viewer keys" and
  "column cursor movement" are added; existing requirements for the
  file tree, scroll viewport, and Tab toggle are untouched.

### New Dependencies

- `charm.land/x/powernap` — JSON-RPC client + LSP server registry +
  capability negotiation. Pinned in `go.mod`.

## Impact

- `internal/lsp/` (new):
  - `types.go` — `Request`, `Result`, `Location`, `Hover` types
    pinky surfaces.
  - `manager.go` — bespoke `Manager`: lazy spawn, 30 s unavailable
    retry, per-server state (`Starting | Ready | Error | Stopped |
    Disabled`), request/reply registry.
  - `client.go` — thin wrapper around `powernap.Client`. Exposes
    `FindDefinition`, `FindReferences`, `Hover`, `DidOpen`,
    `DidClose`, `Shutdown`. Registers the read-only `applyEdit`
    handler.
  - `bridge.go` — `chan → tea.Cmd` glue. `RequestLocations` returns
    a `tea.Cmd` whose `Msg` is `LocationsMsg` or `HoverMsg`.
- `model.go`:
  - `fileViewer` gains `charPos int`.
  - `fileViewMoveLine` clamps `charPos` to the new line's byte
    length.
  - `handleFileViewKey` routes `d`, `R` (via `key.Matches` against
    `FileViewReferences`), and `K` (via `key.Matches` against
    `FileViewHover`) to the new request commands. `h`/`l` always
    move `charPos` (visual-mode extension unchanged).
  - New `stateLSPPicker` state + `handleLSPPickerKey` handler.
  - New `LocationsMsg` and `HoverMsg` cases in `Update` (the
    asynchronous reply arms).
  - `refreshFileView` grows a hover-footer rendering pass.
- `keymap.go`:
  - `FileViewDefinition`, `FileViewReferences`, `FileViewHover`
    bindings added for the help-overlay rows.
  - `FileViewReferences` is `key.WithKeys("R")`,
    `FileViewHover` is `key.WithKeys("K")`,
    `FileViewDefinition` is `key.WithKeys("d")`.
- `picker.go` (existing, in `internal/session` or wherever the
  session picker lives — currently inline in `model.go`):
  - The session picker is factored out into a generic
    `pickItems[T any]` helper. The session picker calls
    `pickItems` over `[]AgentSession`; the new location picker
    calls `pickItems` over `[]Location`. The two pickers share the
    same keybindings (↑/↓/k/j, Enter, Esc, q).
- `go.mod`:
  - `+charm.land/x/powernap` and its transitive requirements.
- Tests:
  - `internal/lsp/manager_test.go` — spawn lifecycle, unavailable
    retry, missing-server path.
  - `internal/lsp/bridge_test.go` — chan → Msg round-trip, request
    cancellation when superseded.
  - `model_test.go` — new cases for column cursor clamping,
    `d`/`R`/`K` routing, picker state transitions, hover footer
    render/dismiss.
  - Existing `workspace-files` tests (file viewer render, scroll
    keys, gutter highlights) untouched.
- No persisted state. No config file. No new keymap rows beyond the
  three new bindings. No changes to `task check` or `Taskfile.yml`.