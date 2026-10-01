# Proposal

## Why

Users currently have no way to keep personal notes or track working items that
survive across runs. Compose mode (`n`) is ephemeral, comments are sent to
the agent on flush, and the file tab is for code review — none of them
cover a private, persistent scratchpad scoped to the workspace.

## What Changes

- Add a third tab "Todos" reachable via `Tab` cycling in the order
  Message → Files → Todos → Message.
- Add `stateTodoList` and `stateTodoEdit` states (mirroring `stateFileNav`
  / `stateFileView`) for the new tab.
- Persist the todo list to `~/.local/share/pinky/<workspace_name>/todos.json`,
  where `<workspace_name>` is the last path segment of `m.cwd`. No
  `$XDG_DATA_HOME` fallback.
- Items have `text` (UTF-8) and `done` (bool). List is insertion-ordered;
  No UUIDs; no reorder keys.
- Mutations (add / toggle / edit / delete) write the full list to disk
  atomically (tmp + rename) before returning to the UI loop.
- Tab header grows from two cells to three (Message, Files, Todos).
- `Tab` cycles forward through three tabs; `Esc` shortcuts back to message.
- `s` in todo states is a no-op — todos are personal and never flushed to
  the agent.

## Capabilities

### New Capabilities
- `todo-tab`: per-workspace structured todo list with add, toggle, edit,
  delete, and persistent storage.

### Modified Capabilities
- `tab-header`: cell count grows from two to three; tab header covers
  the new states.
- `workspace-files`: `Tab` cycles three tabs in fixed order instead of
  toggling message↔files; the `Tab` row in `stateFileNav` /
  `stateFileView` keymap tables reflects the cycle, not a direct return
  to message view.

## Impact

- `model.go`: state enum (`stateTodoList`, `stateTodoEdit`), tab constant
  (`tabTodos`), `toggleTab` becomes a 3-way cycle, three return-state
  slots.
- New `internal/todo/` package: `Item` struct, atomic `Save` / `Load`
  (tmp + rename), in-memory model fields.
- `view.go`: `tabHeader()` renders three cells in canonical order;
  `todoListView()` and `todoEditView()` renderers.
- `keymap.go`: bindings for `stateTodoList` and `stateTodoEdit`; `s` in
  those states is a no-op.
- Tests: new `internal/todo/todo_test.go`, `todo_test.go` (main package),
  extensions to `view_test.go` tab-header tests.