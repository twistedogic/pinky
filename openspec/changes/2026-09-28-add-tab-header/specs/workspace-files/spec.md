# Spec Delta: workspace-files

## REMOVED Requirements

### Requirement: Tab indicator in the status bar

The status line SHALL show a small chip ("msg" / "files")
indicating the active tab. The chip SHALL render in dim style
and SHALL update whenever `m.tab` changes.

#### Scenario: Status bar shows "msg" in message view

- **WHEN** the TUI is in `stateNav` or `stateCompose`
- **THEN** the status line contains the `msg` chip

#### Scenario: Status bar shows "files" in file tab

- **WHEN** the TUI is in `stateFileNav` or `stateFileView`
- **THEN** the status line contains the `files` chip

**Reason**: redundant. The new `tab-header` capability renders
both labels and the active state on a dedicated row at the top
of every attached state; carrying the same information in the
bottom-right chip no longer earns its keep.

**Migration**: see the `tab-header` spec for the replacement
behavior. Tests that asserted on the `msg` / `files` chip in
`statusLine()` should move their assertions to the new
`tabHeader()`.
