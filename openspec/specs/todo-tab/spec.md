# todo-tab Specification

## Purpose

Provides a per-workspace structured todo list for tracking personal notes
and working items that survives across sessions and is independent from
the agent-side comments flush path.

## Requirements

### Requirement: Todo tab is reachable via Tab cycling

The system SHALL provide a third top-level tab "Todos" reachable by
pressing `Tab`. The cycle order SHALL be Message → Files → Todos →
Message. `Tab` SHALL be active in every state except the comment
composer, the picker, and the error screen.

#### Scenario: Tab from message enters file tab
- **WHEN** the user presses `Tab` while `m.tab == tabMessage`
- **THEN** the system enters the file tab (`stateFileNav`)

#### Scenario: Tab from files enters todo tab
- **WHEN** the user presses `Tab` while `m.tab == tabFiles`
- **THEN** the system enters the todo tab (`stateTodoList`)

#### Scenario: Tab from todos returns to message tab
- **WHEN** the user presses `Tab` while `m.tab == tabTodos`
- **THEN** the system enters the message tab and restores the
  message sub-state the user left (nav or compose)

### Requirement: Todo list is persisted per workspace

The system SHALL persist the todo list to
`~/.local/share/pinky/<workspace_name>/todos.json`, where
`<workspace_name>` is `filepath.Base(m.cwd)`. The system SHALL NOT
consult `$XDG_DATA_HOME`.

#### Scenario: First run in a new workspace starts with empty list
- **WHEN** the user attaches to a pane whose cwd's last segment has no
  directory under `~/.local/share/pinky/`
- **THEN** the todo list is empty and the file is created on the first
  mutation

#### Scenario: Subsequent runs load the persisted file
- **WHEN** the user attaches to a pane whose cwd's last segment matches
  an existing `~/.local/share/pinky/<workspace_name>/`
- **THEN** the todo list is loaded from `<workspace_name>/todos.json`
  before the first render

### Requirement: Same workspace name collides intentionally

The system SHALL treat two cwds whose `filepath.Base` is equal as the
same workspace for the purposes of todo storage.

#### Scenario: Two cwds with the same last segment share one file
- **WHEN** the user attaches to pane A with cwd `/work/pinky`, then
  later to pane B with cwd `/home/pinky`
- **THEN** both panes read from and write to
  `~/.local/share/pinky/pinky/todos.json`

### Requirement: Todo items have text and status

The system SHALL store each todo item as a record with two fields: a
UTF-8 `text` string and a `done` boolean. The list SHALL be insertion
ordered and SHALL NOT carry per-item UUIDs.

#### Scenario: Adding an item creates a record with done=false
- **WHEN** the user adds an item with text "ask codex about the race"
- **THEN** the list contains a new record with
  `text="ask codex about the race"` and `done=false`

### Requirement: Todo list supports add, toggle, edit, delete

In `stateTodoList` the system SHALL provide the following keys:

| key             | action |
|-----------------|--------|
| `j` / `↓`       | move cursor down (clamped at last item) |
| `k` / `↑`       | move cursor up (clamped at 0) |
| `a`             | append a new item and open `stateTodoEdit` with empty text |
| `space`         | toggle `done` on the focused item |
| `e`             | open `stateTodoEdit` with the focused item's text |
| `d`             | delete the focused item |
| `Esc`             | return to message tab |
| `Tab`           | forward to next tab in the cycle |
| `q`             | quit pinky |
| `?`             | toggle short / full help overlay |

#### Scenario: Adding appends and opens the editor
- **WHEN** the user presses `a` in `stateTodoList`
- **THEN** a new item with empty text is appended, the cursor moves to
  it, and `stateTodoEdit` opens

#### Scenario: Toggling flips done
- **WHEN** the user presses `space` on a focused item with `done=false`
- **THEN** the item's `done` becomes `true` and the file is rewritten

#### Scenario: Deleting removes the focused item
- **WHEN** the user presses `d` on a focused item
- **THEN** the item is removed from the list, the cursor clamps to the
  new last item (or 0 if the list becomes empty), and the file is
  rewritten

### Requirement: Todo edit view commits on Enter, discards on Esc

In `stateTodoEdit` the system SHALL provide a multi-line input. `Enter`
SHALL commit the typed text to the focused item's `text` field and
return to `stateTodoList`. `Esc` SHALL discard the typed text and return
to `stateTodoList` without modifying the item.

#### Scenario: Enter saves the edit
- **WHEN** the user types "review LSP dispatch" and presses `Enter` in
  `stateTodoEdit`
- **THEN** the focused item's text becomes "review LSP dispatch" and the
  system returns to `stateTodoList`

#### Scenario: Esc discards the edit
- **WHEN** the user types "scratchpad" and presses `Esc` in
  `stateTodoEdit`
- **THEN** the focused item's text is unchanged and the system returns
  to `stateTodoList`

### Requirement: Todo mutations write through to disk

The system SHALL write the full todo list to
`~/.local/share/pinky/<workspace_name>/todos.json` after every
successful mutation (add, toggle, edit, delete) before returning control
to the UI loop. The write SHALL be atomic: write to `<path>.tmp`, then
rename onto `<path>`.

#### Scenario: Quit after edit preserves the change
- **WHEN** the user adds an item, the system writes the file atomically,
  then the user presses `q`
- **THEN** on the next attach, the new item is present

### Requirement: Todos are not flushed to the agent

The `s` key in `stateTodoList` and `stateTodoEdit` SHALL be a no-op.
Todos SHALL NOT be appended to the comments redirect, SHALL NOT appear
in `render.FormatCommentsAppendix`, and SHALL NOT be sent to the agent
pane via `inject.Send`.

#### Scenario: Pressing s in todo tab does not redirect
- **WHEN** the user presses `s` in `stateTodoList`
- **THEN** no `inject.Send` call is made and the todo list is unchanged