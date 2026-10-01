# Tasks

## 1. Storage package

- [x] 1.1 Add `internal/todo/todo.go` with `Item`, `Save(path string, items []Item) error` (write `<path>.tmp` then `os.Rename`), and `Load(path string) ([]Item, error)` returning a sentinel `ErrNotFound` for missing files; verify with a round-trip test (`go test ./internal/todo/...`)

## 2. State and model wiring

- [x] 2.1 Add `tabTodos`, `stateTodoList`, `stateTodoEdit` to the `tab` and `state` enums; add `m.todos []todo.Item`, `m.todoCursor int`, `m.todoEdit string`, and three return-state slots (`m.todoReturn`, rename `m.fileReturn` → `m.fileReturn` stays, add `m.messageReturn`) to the model; verify `go build .` succeeds

## 3. Attach-load and mutation-save

- [x] 3.1 Write a failing test `TestTodo_LoadOnAttach` that points the model at a temp cwd containing a pre-written `~/.local/share/pinky/<basename>/todos.json`, attaches, and asserts `m.todos` matches the file; verify it fails on current implementation by running `go test -run TestTodo_LoadOnAttach ./...`
- [x] 3.2 Implement attach-time `todo.Load` keyed by `filepath.Base(m.cwd)`, plus `todo.Save` on every mutation in `stateTodoList` / `stateTodoEdit` (add, toggle, edit, delete); verify the new test passes and the existing tests stay green

## 4. Three-way tab cycling

- [x] 4.1 Write a failing test `TestTab_CycleOrder` asserting Tab from message → files, files → todos, todos → message, and that sub-state survives a round-trip per tab; verify it fails on current binary `toggleTab` (`go test -run TestTab_CycleOrder ./...`)
- [x] 4.2 Replace the binary `toggleTab` with a 3-way cycle that records and restores the three sub-state slots; verify the new test passes and existing `toggleTab` tests stay green

## 5. Three-cell tab header

- [x] 5.1 Extend `TestTabHeader_FixedCellOrder` to assert all three cells (`Message`, `Files`, `Todos`) appear in that order for every `tab` value, and extend `TestTabHeader_PresentInAttachedStates` to include `stateTodoList` and `stateTodoEdit`; verify they fail on current 2-cell implementation
- [x] 5.2 Update `tabHeader()` to render three cells in canonical Message / Files / Todos order, picking `activeTabStyle` per slot; verify the new tests pass and the existing tab-header tests stay green

## 6. Todo list view

- [x] 6.1 Write `TestTodoList_KeysAndRender` covering `j`/`k` cursor moves (clamped), `a` opens edit, `space` toggles done, `d` deletes, `Esc` returns to message; verify it fails on current implementation
- [x] 6.2 Implement `stateTodoList` keymap and renderer (one row per item: `[ ]` / `[x]` + text, cursor `▶` on focused row, footer keymap); verify the new test passes

## 7. Todo edit view

- [x] 7.1 Write `TestTodoEdit_EnterSavesAndEscDiscards` asserting `Enter` writes `m.todoEdit` into the focused item's text, calls `todo.Save`, and returns to `stateTodoList`; and that `Esc` discards `m.todoEdit` without touching the item; verify it fails on current implementation
- [x] 7.2 Implement `stateTodoEdit` (textarea bound to `m.todoEdit`, `Enter` commits + saves + returns, `Esc` discards + returns); verify the new test passes

## 8. No-flush guarantee

- [x] 8.1 Write `TestTodo_SIsNoOp` using a stub for `inject.Send` (or an observable side-effect counter) asserting `s` in `stateTodoList` and `stateTodoEdit` does not call into the inject pipeline and does not modify the todo list; verify it fails until `s` is wired as a no-op, then verify it passes after the wiring

## 9. Final verification

- [x] 9.1 Run `task check` (vet + tests) and confirm the full suite stays green