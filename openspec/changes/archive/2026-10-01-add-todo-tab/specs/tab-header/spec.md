# Spec Delta

## MODIFIED Requirements

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

## REMOVED Requirements
<!-- None -->