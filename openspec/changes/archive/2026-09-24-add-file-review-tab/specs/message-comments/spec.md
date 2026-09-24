## ADDED Requirements

### Requirement: Comment shape extended for file-kind comments

The system SHALL extend the Comment struct with a Kind discriminator that takes two values.

#### Scenario: Block-kind comment keeps the zero-value Kind

- **WHEN** a block-kind comment is saved through the message
  view's composer
- **THEN** `Kind == CommentBlock` (the zero value) and
  `Path`, `LineStart`, `LineEnd` are empty / zero

#### Scenario: File-kind comment carries the path and line range

- **WHEN** a file-kind comment is saved through the file
  viewer's composer with anchor `(internal/foo.go, 12, 24)`
- **THEN** the saved comment has `Kind == CommentFile`,
  `Path == "internal/foo.go"`, `LineStart == 12`,
  `LineEnd == 24`, and `BlockIdx == 0`, `CharStart == 0`,
  `CharEnd == 0`

### Requirement: Appendix format covers both comment kinds

The system SHALL format both block-kind and file-kind comments into a single chronological list via render.FormatCommentsAppendix.

#### Scenario: Mixed-kind appendix contains both shapes

- **WHEN** the comment slice contains one block-kind comment
  (anchor block 3) and one file-kind comment
  (anchor internal/foo.go lines 12-24)
- **THEN** the appendix output contains both
  `- block "<excerpt>" (lines ...)` and
  `- file "internal/foo.go" (lines 12-24):` lines, in
  chronological order, prefixed by `2 comments:`

#### Scenario: Single file-kind comment

- **WHEN** the comment slice contains exactly one file-kind
  comment
- **THEN** the appendix output is `1 comment:\n- file
  "<path>" (lines X-Y): <text>\n`

#### Scenario: File-kind comments preserve chronological order

- **WHEN** two file-kind comments are saved in order A then B
- **THEN** the appendix output lists A before B regardless of
  their `Path` or `LineStart` values

### Requirement: Unified flush path

The system SHALL flush every comment in m.comments through a single inject call when the user presses s in any state where s is bound.

#### Scenario: `s` from file view flushes both kinds

- **WHEN** `m.comments` contains one block-kind and one
  file-kind comment and the user presses `s` in `stateFileView`
- **THEN** a single `sendToPane` call is made with the
  mixed-kind appendix as the payload, and the slice is cleared
  on success

#### Scenario: `s` flush failure keeps both kinds

- **WHEN** the user presses `s` and the inject call returns an
  error
- **THEN** neither the block-kind nor the file-kind comments
  are cleared from `m.comments`; the error is surfaced via the
  `[send failed: …]` placeholder
