# Design

## Context

`stateFileView` is a line-only file viewer today. `fileViewer` has
`cursor int` (a 1-based line index) and a `fileSelection` that
captures `(LineA, CharA, LineC, CharC)` only when visual mode is
active. Outside visual mode there is no column position; `h` and `l`
are no-ops. Adding LSP forces a structural change: every method we
expose (`definition`, `references`, `hover`) needs a position, and
the only sensible position is the cursor's column. So the file
viewer gains `charPos int`, `h`/`l` always move it, and the LSP
client dispatches queries at `(cursor, charPos)`.

The wire protocol is already solved. `charm.land/x/powernap`
(`charm.land/x/powernap/pkg/lsp`) ships a working stdio JSON-RPC
client, a server registry with sensible defaults (gopls,
typescript-language-server, rust-analyzer, clangd,
jedi-language-server), `DetectLanguage(path)` for language
detection, and the standard capability-negotiation / `didOpen` /
`didChange` / `didClose` plumbing. Pinky writes a thin pinky-shaped
manager on top: lazy spawn, 30 s unavailable retry on missing
binaries, per-server state, and three method wrappers
(`FindDefinition`, `FindReferences`, `Hover`) that mirror the
1-based → 0-based conversion powernap expects.

Powernap's API is synchronous. Bubble Tea wants `tea.Cmd` returning a
`Msg`. The bridge is a small goroutine that reads replies from a
shared channel and the model emits `tea.Cmd` values that select on
that channel. Request IDs disambiguate replies; stale requests (the
user pressed `d` twice in a row) are dropped.

The picker is the only existing pattern we change. The current
session picker lives inline in `model.go` and is hard-coded to
`[]session.AgentSession`. Adding LSP needs a second picker for
`[]protocol.Location` with snippet previews. The two pickers have
identical UX, so we factor out `pickItems[T any](items, label, onSelect
func(T))` and let both call sites use it.

Read-only enforcement is a one-line protocol registration: any
`workspace/applyEdit` request from the server gets a reply of
`Applied: false, FailureReason: "pinky is read-only"`. There is no
scenario in which pinky accepts an edit.

## Goals / Non-Goals

**Goals:**

- Wire LSP into `stateFileView` with a column cursor, three new keys
  (`d` / `R` / `K`), a shared location picker, and a hover footer.
- Use `charm.land/x/powernap` directly. No `agentutil` wrapper, no
  copy-and-trim.
- Refuse all server-initiated write requests (`applyEdit`,
  `executeCommand`, file-watching capability registrations).
- Generalise the picker so `d` and `R` share the same UI for
  multi-location results.
- Stay within pinky's "no config" stance: ship with powernap's
  default server registry, no user-editable list.

**Non-Goals:**

- No `textDocument/rename`, `textDocument/completion`,
  `textDocument/codeAction`, or any other write-shape LSP method.
- No `textDocument/documentHighlight` (cuts the wire call even
  though it is free — user opted out).
- No hover popup / multi-line overlay. Hover renders as a single
  footer line; multi-line content is truncated.
- No cross-file navigation history (`Ctrl-O`-style "back"). A
  jump to another file replaces the viewer's file; returning is
  the user's responsibility (the dir navigator preserves their
  tree position).
- No LSP capability in `stateNav`, `stateCommentComposer`,
  `stateCompose`, `statePicking`, or `stateError`. LSP is a
  file-viewer-only feature.
- No per-language configuration. No environment variables for
  server binary paths. (If users need this later, it is a separate
  change with its own spec.)
- No workspace-folder changes. Pinky announces one folder (the
  pane cwd) per session.

## Decisions

### D1 — Use `charm.land/x/powernap` directly

Powernap is the only LSP JSON-RPC client in the charmbracelet
ecosystem; it ships with the registry, defaults, and capability
negotiation pinky needs. Alternatives considered: (a) import
`agentutil/tools/lsp` (a wrapper), (b) copy-and-trim agentutil,
(c) write our own JSON-RPC. (a) and (b) bring a `fantasy`
transitive dep and ~1.5 kLOC of code pinky does not need; (c)
duplicates well-tested code. Powernap direct is the smallest
correct answer.

### D2 — Bespoke Manager, no `ConfigStore` interface

agentutil's `Manager` is generic because it supports crush's many
consumers via a `ConfigStore` interface. Pinky has no config.
The pinky Manager is hard-coded to powernap's defaults, takes the
pane cwd as `workDir`, and exposes a pinky-shaped surface
(`Start(ctx, path)`, `FindDefinition(uri, line, char)`,
`FindReferences(uri, line, char)`, `Hover(uri, line, char)`,
`DidOpen(uri, content, lang)`, `DidClose(uri)`, `Shutdown()`). No
generic `ConfigStore`, no `AutoLSP` toggle, no per-user overrides.

### D3 — Column cursor in `fileViewer`; `h`/`l` always move it

Adding `charPos int` is the smallest change that lets LSP queries
have a position. Two alternatives considered: (a) use the visual-mode
`CharA`/`CharC` as the query position — forces the user into visual
mode, which is hostile; (b) snap `charPos` to the start of the
identifier under the cursor — duplicate of what gopls already does
server-side. (a) and (b) both add user-visible complexity for no
gain. We make `h`/`l` always move `charPos` (clamped to the line's
byte length); in visual mode they extend the visual range exactly
as today.

The rendered view does not gain a column cursor highlight in v0 —
the line cursor's gutter marker is enough for the user to track
their position, and a column indicator would be a separate UI
change. If the user wants one later, it is a follow-up.

### D4 — Definition: 0 silent / 1 jump / N>1 picker

`d`'s UX is the VS Code / LSP convention: a single-result query
jumps immediately; only multiple results land in the picker. This
matches the user's request and is the most natural shape for an
LSP-backed key. 0 results is silent (no "no definition" toast);
the user's keystroke had no effect, and a missing feedback line is
less distracting than a transient banner.

A `LocationsMsg` handler in `Update` switches on `len(locations)`:

```go
switch len(msg.locations) {
case 0: // silent
case 1:
    m.jumpToLocation(msg.locations[0])
default:
    m.lspPicker = newLocationPicker(msg.locations, "definition")
    m.state = stateLSPPicker
}
```

### D5 — References: 0 silent / N≥1 picker

References always go through the picker. There is no "jump on 1"
case for `R`: a single reference is rarely what the user wants
without seeing the list, and the asymmetric UX would confuse.
Both `d` (at N>1) and `R` (always) land in the same picker.

### D6 — Shared picker state

A new `stateLSPPicker` holds `[]protocol.Location` plus a label
("definition" or "references"). The picker is a thin
`pickItems[T any]` helper in `internal/lsp/picker.go` (or wherever
the existing session picker extracts to) that takes a slice and a
`onSelect func(T)` closure; the existing session picker is
rewritten to use the same helper. Both pickers share keybindings
(↑/↓/k/j, Enter, Esc, q) and the same render shape. This is the
cleanest factoring: one picker implementation, two callers.

### D7 — Cross-file navigation = close current viewer + reopen at (line, char)

A location pointing to a different file closes the current
`fileViewer`, transitions to `stateFileView` for the new path, and
seeds `cursor = line`, `charPos = char`. `scrollFileCursorIntoView`
handles scrolling. Same-file locations only mutate `cursor` and
`charPos` and call `scrollFileCursorIntoView` — no state change.

No cross-file history. The user pressed a key; they own the
destination now.

### D8 — Hover is a one-line footer

`textDocument/hover` returns a `Hover` whose `Contents` is
typically a single type signature or 1-3-line doc snippet. Pinky
renders the first line in a dedicated footer between the file body
and the help line. Multi-line content is truncated with an
ellipsis. No popup overlay; no `stateLSPHover`; no scrollable
hover panel.

Any non-hover key (including `K` again to refetch at a new cursor)
clears the footer. The cursor's `(line, col)` is what the next
hover re-queries.

### D9 — Read-only via `applyEdit` handler

The powernap client exposes
`RegisterHandler(method string, fn Handler)`. Pinky registers
`workspace/applyEdit` once at client creation with a handler that
returns:

```go
protocol.ApplyWorkspaceEditResult{
    Applied:      false,
    FailureReason: "pinky is read-only",
}
```

Other server-initiated requests (`workspace/configuration`,
`client/registerCapability`, `window/workDoneProgress/create`) are
either replied to with sensible defaults (empty configuration) or
ignored. `textDocument/publishDiagnostics` notifications are logged
via `slog` and discarded — pinky does not surface diagnostics in v0.

### D10 — Out of scope: documentHighlight, rename, cross-file "back"

User explicitly opted out of `documentHighlight` and the
`Ctrl-O`-style cross-file "back" history. Hover was added
explicitly. Rename / completion / code-action were already out
under the read-only constraint. These four exclusions are
documented in `specs/workspace-files/spec.md` so future readers
do not re-litigate them.

## Risks / Trade-offs

- **First-call latency.** gopls cold-start is 1–5 s; users see a
  "starting…" footer for the first `d`/`R`/`K` press in a session.
  After that, queries are sub-100 ms. Acceptable; documented in the
  spec as "LSP responses are asynchronous; the file viewer shows a
  one-line status hint until the first response arrives."
- **Server cache side effects.** gopls writes `.gopls/` to the
  project root on first query; typescript-language-server has
  similar caches. Not pinky's problem to solve in v0. If the user
  objects later, the answer is a per-session sandbox dir; flagged
  but not built.
- **`powernap` is `charm.land/x/`.** Pre-1.0; can churn. Pinky
  pins the version explicitly in `go.mod`. If a breaking change
  ships, the upgrade is a focused change.
- **Concurrent requests.** A second `d` press while the first is
  in flight bumps the request id; replies for the older id are
  discarded by the bridge. The user sees only the latest result.
- **Server crash.** Powernap's `IsRunning()` returns false when
  the server dies. The Manager marks the client `Error`, the
  footer shows "LSP server stopped — press K to retry" briefly,
  and the next query attempt re-spawns.
- **Hover truncation.** A 50-line doc comment becomes
  `// Foo does X…`. The user can ask for the full thing via a
  follow-up change (hover popup), but it is out of v0.
- **Picker search.** The existing session picker has no `/`-search;
  the location picker inherits that limitation. A 500-reference
  picker is annoying to scroll. Out of scope; flag for follow-up.

## Migration Plan

Single deploy. No persisted state. No backwards compatibility
concerns — pinky has no users on the previous shape. The
`internal/lsp` package lands whole; the file viewer changes are
gated on the column cursor; the picker generalisation lands
alongside the LSP picker call site.

Reverting means `git revert` of the merge commit. The powernap
dependency is removed; `internal/lsp` is removed; `fileViewer.charPos`
is removed; `h`/`l` revert to no-op outside visual mode.

## Open Questions

- **Should the cursor's column render as a visual indicator?** A
  highlighted column (underline / caret) would give the user
  feedback on where their `h`/`l` presses land. Out of v0; the
  line gutter is the only position indicator today and adding a
  column indicator is a separate UI change.
- **Should missing-server hints be persistent or one-shot?** The
  current design shows the hint the first time the user presses
  `d`/`R`/`K` and a server is missing, then stays quiet for 30 s.
  If the user keeps pressing keys with no server, do we re-hint
  every press (annoying) or stay quiet (confusing)? The current
  answer: re-hint on state transition (`Disabled → Retry`), not on
  every press. If users complain, we revisit.
- **Hover popup later?** A multi-line hover overlay is a natural
  follow-up. Out of v0; if it lands, it gets its own spec.