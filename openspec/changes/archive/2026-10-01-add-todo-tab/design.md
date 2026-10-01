# Design

## Context

Pinky has two top-level tabs (`tabMessage`, `tabFiles`) wired through a
binary `toggleTab()`. Adding a third tab turns the cycle 3-way and means
each tab needs its own "return state" memory (currently only `fileReturn`
exists, sized for 2 tabs). Storage is the new piece: the project has
`internal/history` (write-only JSONL), and the new `internal/todo`
package mirrors that pattern but adds load-on-attach and a typed schema.

## Goals / Non-Goals

**Goals:**
- Three tabs cycle linearly via `Tab` (Message → Files → Todos →
  Message).
- Todo items persist on every mutation; load-on-attach; no in-memory
  merging.
- Workspace identity is `filepath.Base(cwd)`; collision is intentional
  and documented.
- Items are personal and never reach `inject.Send`.

**Non-Goals:**
- Reorder (no `J`/`K` shift, no UUIDs).
- Todo → agent flush (`s` is a no-op in todo states).
- Multi-list / tags / filters / priorities. Items are a flat list with
  text + done.
- Sync, cloud, multi-device.
- Reverse cycle (`Shift+Tab`).
- `~/.local/share/pinky/<workspace_name>/` itself — created lazily by
  the first save.

## Decisions

- **Hard-coded storage path, no XDG.** `~/.local/share/pinky/<workspace_name>/todos.json`
  literally; `$XDG_DATA_HOME` is not consulted. Diverges from
  `internal/history`'s XDG-then-fallback pattern, intentionally.
- **Atomic write on every mutation.** Single file rewrite per change
  is ~µs for v0 lists; atomicity (`*.tmp` + `os.Rename`) avoids
  half-written files from a process kill. Trades IO for safety.
- **Two states mirror `stateFileNav` / `stateFileView`.** `stateTodoList`
  + `stateTodoEdit` reuse the existing sub-state pattern; no new
  architecture for sub-states.
- **Insertion-order list.** Simplest model; stable order on disk ==
  display order; no UUIDs; no reorder.
- **`s` no-op in todo states.** Comments remain the only flushable kind;
  todo items are sealed-off personal state.
- **3-way cycle, fixed order.** M → F → T → M. No `Shift+Tab` (YAGNI).
- **Esc shortcuts back to message view** in todo states so the user has
  a one-key return without cycling.

## Risks / Trade-offs

- **Same-name collision**: `/work/pinky` and `/home/pinky` share one
  file. Documented in spec; fix (suffix with parent dir or hash) when
  it bites. Most users have one project per directory name.
- **Disk write on every keypress**: every toggle / edit / delete
  triggers a write. For a list of <1000 items this is negligible
  (~µs). If profiling shows otherwise, debounce.
- **First-run creation**: `~/.local/share/pinky/<workspace>/` doesn't
  exist before the first save. `os.MkdirAll` on save handles this; no
  upfront mkdir.
- **No sort / filter**: a long list is a flat scroll. Fine for v0; add
  filters when the list becomes uncomfortable.

## Migration Plan

None — `~/.local/share/pinky/<workspace_name>/` is a fresh directory;
nothing exists to migrate from.

## Open Questions

None.