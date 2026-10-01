# tab-header Specification

## Purpose

A persistent, single-row tab indicator at the top of every attached
TUI state that names both top-level views (message, files) and
shows which one is active. The row replaces the status-line tab
chip so the active view is discoverable from the top of the screen
instead of only from a small chip in the bottom-right.

## Requirements

### Requirement: 1-row tab header above the cwd header

The system SHALL render a 1-row tab header immediately above the
existing cwd header on every attached state
(`stateNav`, `stateCompose`, `stateCommentComposer`,
`stateFileNav`, `stateFileView`, `stateTodoList`, `stateTodoEdit`,
`stateLSPPicker`). The header SHALL show exactly three cells, in
this order: `Message`, `Files`, and `Todos`. The active cell SHALL
render with a filled background; the inactive cells SHALL render
in dim style. The row is a passive indicator — it SHALL NOT
respond to mouse or keyboard input directly, and SHALL NOT change
the active tab on click or focus.

#### Scenario: Tab header is present in stateNav
- **WHEN** the TUI is in `stateNav`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateCompose
- **WHEN** the TUI is in `stateCompose`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateCommentComposer
- **WHEN** the TUI is in `stateCommentComposer`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateFileNav
- **WHEN** the TUI is in `stateFileNav`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateFileView
- **WHEN** the TUI is in `stateFileView`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateLSPPicker
- **WHEN** the TUI is in `stateLSPPicker`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateTodoList
- **WHEN** the TUI is in `stateTodoList`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is present in stateTodoEdit
- **WHEN** the TUI is in `stateTodoEdit`
- **THEN** the rendered output's first row is the tab header
  containing the labels `Message`, `Files`, and `Todos` in that
  order

#### Scenario: Tab header is absent in statePicking
- **WHEN** the TUI is in `statePicking`
- **THEN** the rendered output does NOT contain the tab header
  (no agent is attached yet)

#### Scenario: Tab header is absent in stateError
- **WHEN** the TUI is in `stateError`
- **THEN** the rendered output does NOT contain the tab header

### Requirement: Active cell reflects m.tab

The active cell of the tab header SHALL match `m.tab`. When
`m.tab == tabMessage`, the `Message` cell SHALL render with a
filled background and the `Files` cell SHALL render in dim
style. When `m.tab == tabFiles`, the `Files` cell SHALL render
with a filled background and the `Message` cell SHALL render in
dim style. The active cell SHALL swap as soon as `m.tab` changes
(typically a `Tab` keypress).

#### Scenario: Message tab is filled when m.tab == tabMessage

- **WHEN** `m.tab == tabMessage`
- **THEN** the `Message` cell of the tab header has a filled
  background and the `Files` cell is dim

#### Scenario: Files tab is filled when m.tab == tabFiles

- **WHEN** `m.tab == tabFiles`
- **THEN** the `Files` cell of the tab header has a filled
  background and the `Message` cell is dim

### Requirement: Header height accounts for the tab row

The system's existing header-height accounting (the `m.headerHeight`
field and the `reflow()` math that subtracts it from the viewport
height) SHALL include the tab row. When the tab header is
present, `m.headerHeight` SHALL equal
`1 (tab row) + strings.Count(cwdLine, "\n") + 1`, so the viewport
shrinks by 1 row compared to the same content without the tab
header.

#### Scenario: Single-line cwd adds 2 rows to headerHeight

- **WHEN** the cwd header renders on a single line and the tab
  row is present
- **THEN** `m.headerHeight == 2` and the viewport height has 2
  fewer rows than `m.height - statusHeight - helpHeight()`

#### Scenario: Wrapped 2-line cwd adds 3 rows to headerHeight

- **WHEN** the cwd header wraps to 2 lines and the tab row is
  present
- **THEN** `m.headerHeight == 3` and the viewport height has 3
  fewer rows than `m.height - statusHeight - helpHeight()`

### Requirement: Status line drops the tab chip

The status line SHALL NOT render the previous `msg` / `files`
tab chip. The tab header carries that information. The status
line SHALL continue to render every other chip (streaming dot,
`[I] include N comments — ON/OFF`, `[VISUAL]`) unchanged.

#### Scenario: Status line omits the tab chip in message view

- **WHEN** `m.tab == tabMessage` and the status line is rendered
- **THEN** the status line does NOT contain `msg` or `files` as
  a chip

#### Scenario: Status line omits the tab chip in file tab

- **WHEN** `m.tab == tabFiles` and the status line is rendered
- **THEN** the status line does NOT contain `msg` or `files` as
  a chip
