# Spec Delta

## MODIFIED Requirements

### Requirement: Block-level annotation via `c`

The system SHALL allow the user to attach a free-text comment to
the current source line in the nav view. Pressing `c` in nav
when no visual mode is active (or when visual mode is active
with an empty range) SHALL open a comment composer
pre-anchored to the whole current source line (anchor
`(byteA, byteC)` covering every byte on that line in
`m.latest.Text`, inclusive of the trailing newline). Pressing
`Enter` in the comment composer SHALL save the new comment
with that anchor and SHALL return to the nav view. `Enter`
SHALL NOT dispatch any comments to the agent — staging and
sending are decoupled. The flush happens via `s` in `stateNav`
(see `### Requirement: One-shot submit all comments via \`s\``
in `single-cursor-nav`). Cancelling (`Esc`) SHALL discard the
composer text and return to the nav view without creating or
sending a comment.

#### Scenario: `c` opens composer for current block
- **WHEN** the user presses `c` in `stateNav` and the cursor's
  `lineIdx` is `L`
- **THEN** the TUI enters comment composer state with the
  composer textarea focused and empty

#### Scenario: Enter saves the comment and returns to nav without sending
- **WHEN** the user is in comment composer state with text in
  the textarea and presses `Enter`
- **THEN** a comment is stored with anchor `(byteA, byteC)`
  covering the whole source line at `L`, the TUI returns to
  `stateNav`, and **no** `sendToPane` call is made

#### Scenario: Multiple `c → Enter` cycles accumulate comments in memory
- **WHEN** the user repeats `c → type → Enter` twice, producing
  two comments anchored to different source lines
- **THEN** after the second `Enter`, `m.comments` contains both
  comments in insertion order, the TUI is in `stateNav`, and
  `sendToPane` has not been called

#### Scenario: Enter on empty composer is a no-op
- **WHEN** the user is in comment composer state with an empty
  textarea and presses `Enter`
- **THEN** no comment is stored, `sendToPane` is not called,
  and the TUI returns to `stateNav`

#### Scenario: Cancel discards composer
- **WHEN** the user is in comment composer state and presses `Esc`
- **THEN** no comment is stored, no inject call is made, the TUI
  returns to nav state, and the cursor's `(lineIdx, charPos,
  preferred)` is unchanged

#### Scenario: `c` with no focused block is a no-op
- **WHEN** the user presses `c` in `stateNav` and no source
  line is available (empty state)
- **THEN** the TUI remains in `stateNav` and no composer is
  opened

### Requirement: Character-granularity visual selection

The system SHALL provide a character-granularity visual mode for
selecting a byte range within `m.latest.Text`. Pressing `v` in
`stateNav` SHALL enter visual mode and seed the selection's
anchor at the cursor's current `(lineIdx, charPos)`. While in
visual mode, `h` SHALL retreat the cursor one rune within the
current source line, `l` SHALL advance the cursor one rune,
`j` SHALL move the cursor to the next source line (applying
the preferred-column rule: `charPos = min(preferred, len(newLine))`,
`preferred` unchanged), and `k` SHALL move to the previous
source line (same rule). Pressing `Esc` or `v` again SHALL exit
visual mode without saving. Pressing `c` SHALL open the comment
composer pre-anchored to the selection range `(byteA, byteC)`
where `byteA` is the byte offset of the anchor in `m.latest.Text`
and `byteC` is the byte offset of the cursor in `m.latest.Text`.

#### Scenario: `v` enters visual at the cursor
- **WHEN** the user presses `v` in `stateNav`
- **THEN** the TUI enters visual mode with the selection's
  anchor seeded at the cursor's current `(lineIdx, charPos)`
  and `byteA == byteC` (single-point selection)

#### Scenario: `l` extends visual cursor right
- **WHEN** the user is in visual mode and presses `l`
- **THEN** the cursor's `charPos` advances by the byte length of
  the rune at the current `charPos` (clamped at
  `len(currentLine)`), `preferred = max(preferred, charPos)`,
  and `byteC` is updated to the cursor's new byte offset in
  `m.latest.Text`

#### Scenario: `h` extends visual cursor left
- **WHEN** the user is in visual mode and presses `h`
- **THEN** the cursor's `charPos` retreats by the byte length of
  the rune preceding the current offset (clamped at `0`),
  `preferred` is unchanged, and `byteC` is updated to the
  cursor's new byte offset in `m.latest.Text`

#### Scenario: `j` extends visual selection across blocks
- **WHEN** the user is in visual mode and presses `j`
- **THEN** the cursor's `lineIdx` advances by one
  (clamped at the last source line), `charPos` becomes
  `min(preferred, len(newLine))`, `preferred` is unchanged,
  and `byteC` is updated to the cursor's new byte offset in
  `m.latest.Text`

#### Scenario: `Esc` exits visual without saving
- **WHEN** the user is in visual mode and presses `Esc`
- **THEN** the TUI returns to nav state, the visual flag and
  selection are cleared, and no comment is stored

#### Scenario: `v` toggles visual off
- **WHEN** the user is in visual mode and presses `v` again
- **THEN** the TUI exits visual mode (same effect as `Esc`)

#### Scenario: `c` over a selection opens the composer
- **WHEN** the user is in visual mode and presses `c`
- **THEN** the TUI enters comment composer state with the
  composer textarea focused and empty, and the saved anchor is
  the selection range `(byteA, byteC)` over `m.latest.Text`

### Requirement: Inline selection anchors by byte offset

The system SHALL anchor inline comments by global byte offsets
into `m.latest.Text`. When the comment composer is opened from
visual mode, the saved anchor SHALL be `(byteA, byteC)` where
`byteA` is the byte offset of the selection's anchor in
`m.latest.Text` and `byteC` is the byte offset of the selection's
cursor in `m.latest.Text`. The byte offsets are the verbatim
anchors set by `v` and extended by `h` / `l` / `j` / `k`; no
projection through a markdown block AST is required because
the cursor already lives in source-byte space.

#### Scenario: Inline anchor uses byte offsets
- **WHEN** the user opens the comment composer from visual mode
  and saves
- **THEN** the saved comment has anchor `(byteA, byteC)` where
  `0 <= byteA <= byteC <= len(m.latest.Text)` and both are byte
  offsets into `m.latest.Text`

#### Scenario: Inline anchor preserves source text
- **WHEN** a comment is saved with anchor `(byteA, byteC)`
- **THEN** the verbatim slice `m.latest.Text[byteA:byteC]` is
  stored alongside the comment so the original text can be
  quoted in the redirect appendix

### Requirement: Include comments in redirect

While in compose state, pressing `i` SHALL toggle whether
the queued redirect will be sent with a comments appendix. When
the flag is enabled and the user sends the redirect (`s`), the
inject payload SHALL be the redirect text followed by the
comments appendix (see `### Requirement: Comments appendix split
by kind with new format`). The appendix SHALL NOT be prefixed
with `---` or any other separator when the redirect text is
non-empty; the appendix renderer emits only the section labels
and entries. The append happens only when the flag is on AND
there is at least one comment.

#### Scenario: `i` toggles include flag
- **WHEN** the user is in compose state and presses `i`
- **THEN** the include-comments flag is toggled, and the status
  line reflects the new state (e.g., `[I] include N comments —
  ON/OFF`)

#### Scenario: `s` with flag ON appends appendix
- **WHEN** the user is in compose state, the include flag is ON,
  and the user presses `s` with non-empty text
- **THEN** the inject payload sent to the agent's pane is
  `<redirect text>` immediately followed by the appendix body,
  where the appendix body is the output of `FormatCommentsAppendix`
  (a "Comments on the message:" section, a blank line, and a
  "Comments on files:" section). No leading `---` separator is
  emitted.

#### Scenario: `s` with flag OFF sends plain redirect
- **WHEN** the user is in compose state, the include flag is OFF,
  and the user presses `s` with non-empty text
- **THEN** the inject payload is exactly the redirect text with
  no comments appendix

#### Scenario: `s` with flag ON and zero comments sends plain redirect
- **WHEN** the user is in compose state, the include flag is ON,
  and there are no comments, and the user presses `s`
- **THEN** the inject payload is exactly the redirect text (no
  appendix because the count is zero)

### Requirement: Comment shape extended for file-kind comments

The system SHALL extend the Comment struct with a Kind
discriminator that takes two values: message-kind comments and
file-kind comments. Message-kind comments SHALL carry anchor
`(byteA, byteC)` over `m.latest.Text`. File-kind comments SHALL
carry anchor `(Path, LineStart, LineEnd)` over the file's raw
content with optional `(byteA, byteC)` for inline byte ranges.

#### Scenario: Block-kind comment keeps the zero-value Kind
- **WHEN** a message-kind comment is saved through the message
  view's composer
- **THEN** the saved comment has `Kind == CommentMessage` (the
  zero value of the `CommentKind` discriminator) and `Path`,
  `LineStart`, `LineEnd` are empty / zero

#### Scenario: Message-kind comment carries global byte anchors
- **WHEN** a message-kind comment is saved through the message
  view's composer with anchor `(byteA, byteC)` covering one or
  more source lines of `m.latest.Text`
- **THEN** the saved comment has `Kind == CommentMessage`,
  `byteA` and `byteC` are non-negative byte offsets into
  `m.latest.Text`, and `Path`, `LineStart`, `LineEnd` are
  empty / zero

#### Scenario: File-kind comment carries the path and line range
- **WHEN** a file-kind comment is saved through the file
  viewer's composer with anchor `(internal/foo.go, 12, 24)`
- **THEN** the saved comment has `Kind == CommentFile`,
  `Path == "internal/foo.go"`, `LineStart == 12`,
  `LineEnd == 24`, and `byteA`, `byteC` are `0` (line-range
  comment, no inline byte range)

## REMOVED Requirements

### Requirement: Comment footnote rendering
**Reason**: Footnote-below-block rendering is removed. Saved
comments are now indicated by a line-direct yellow `▍` gutter
on every source line touched by the comment's byte range (see
Line-direct gutter for saved comments). No footnote text is
rendered on the source.
**Migration**: No user-facing migration. Comments are still
visible in the comments appendix when sent via `s` (in nav) or
appended to a redirect via the include flag (in compose). The
on-source signal is now a yellow gutter per touched line.

### Requirement: Border and comment highlight coexist
**Reason**: The cyan `▍` focus gutter is gone (replaced by an
inline block cursor at `charPos`, defined in `latest-message-
view`). The yellow comment gutter is now line-direct (defined
in Line-direct gutter for saved comments). The coexistence rule
between cyan and yellow gutters has no referent.
**Migration**: No user-facing migration. Comment presence is
now indicated by the yellow gutter per touched line; cursor
position is indicated by the inline block cursor at the byte
the cursor points at.

### Requirement: Cursor drives the gutter highlight
**Reason**: The cyan `▍` gutter highlight is removed (replaced
by an inline block cursor at `charPos`). The cursor still
drives the viewport (see Viewport follows cursor with a 1-line
cushion in `single-cursor-nav`) but no longer drives a gutter.
**Migration**: No user-facing migration. The cursor's byte
position is rendered as the inline block cursor; no gutter is
drawn for focus.

### Requirement: Appendix format covers both comment kinds
**Reason**: Replaced by Comments appendix split by kind with new
format, which drops the count line, splits the body into two
labelled sections ("Comments on the message:" and "Comments on
files:"), and formats message-kind entries as
`- comment on "<excerpt>": <text>` (no line position).
**Migration**: Agent-side: messages now reference comments by
quoted text only, with section labels separating message-kind
from file-kind comments. File-kind comments keep today's format
(`- file "<path>" (lines X-Y): <text>` and
`- file-inline "<excerpt>" (line Z): <text>`).

## ADDED Requirements

### Requirement: Line-direct gutter for saved comments

Every source line in `m.latest.Text` whose byte range overlaps
at least one saved comment's byte range SHALL carry the
character `▍` in yellow (foreground colour `228`) in the
leftmost column when rendered. Source lines that overlap no
saved comment SHALL carry a single space in that column. The
gutter SHALL NOT modify the message text itself; it sits on top
of the rendered output as an overlay. The yellow gutter SHALL
coexist with the inline block cursor at `charPos` (see Inline
block cursor at charPos in `latest-message-view`): when the
cursor's source line has yellow gutter and the cursor's byte
position is on that line, both SHALL render (yellow gutter in
the leftmost column; inline cursor at `charPos` in the body).

#### Scenario: Line touched by a comment shows yellow gutter
- **WHEN** a saved comment has anchor `(byteA, byteC)` and
  source line `L` covers at least one byte in
  `[byteA, byteC)`
- **THEN** source line `L` carries a yellow `▍` in its leftmost
  column when rendered

#### Scenario: Uncommented source line shows empty gutter column
- **WHEN** no saved comment's anchor covers any byte on source
  line `L`
- **THEN** source line `L` carries a single space in its leftmost
  column

#### Scenario: Multi-line comment marks every touched line
- **WHEN** a saved comment has anchor `(byteA, byteC)` that
  spans source lines `L1` through `L2`
- **THEN** every source line in `[L1, L2]` carries a yellow `▍`
  in its leftmost column when rendered

#### Scenario: Yellow gutter coexists with inline cursor on the same line
- **WHEN** a saved comment's anchor covers source line `L` and
  the cursor's `lineIdx == L`
- **THEN** the rendered view shows a yellow `▍` in `L`'s
  leftmost column AND the inline block cursor at `(L, charPos)`
  in the body

#### Scenario: Annotation overlay survives width reflow
- **WHEN** the terminal width changes and the rendered message
  is re-flowed
- **THEN** the yellow gutter is re-applied without loss; no
  annotation references a stale line index from the previous
  width

### Requirement: Comments appendix split by kind with new format

When comments are flushed via `s` (in `stateNav`) or appended
to a redirect via the include flag (in `stateCompose`), the
appendix body SHALL be emitted by
`render.FormatCommentsAppendix(m.comments)` and SHALL have the
following structure:

- A single line `Comments on the message:` followed by one
  entry per message-kind comment, in chronological order
  (oldest first). Each entry SHALL have the form
  `- comment on "<excerpt>": <text>` where `<excerpt>` is the
  verbatim text of the comment's anchor
  (`m.latest.Text[byteA:byteC]`), truncated to 40 characters
  with an ellipsis (`…`) appended if longer, with internal
  newlines replaced by single spaces and runs of whitespace
  collapsed to a single space, then trimmed.
- A blank line.
- A single line `Comments on files:` followed by one entry per
  file-kind comment, in chronological order (oldest first).
  Each entry SHALL have the form
  `- file "<path>" (lines X-Y): <text>` for line-range
  comments or `- file-inline "<excerpt>" (line Z): <text>` for
  inline byte-range comments, where `<path>` is the relative
  path under the agent pane's cwd and `<excerpt>` is the
  verbatim text of the inline range (truncated to 40 chars
  with `…` if longer).

The appendix SHALL NOT be prefixed with `---` or any other
separator; it SHALL NOT be prefixed with a count line; and
empty sections (no message-kind comments or no file-kind
comments) SHALL omit the corresponding label and entries but
still emit the blank line between sections (so two empty
sections collapse to a single blank line).

#### Scenario: Mixed-kind appendix contains both sections
- **WHEN** the comment slice contains one message-kind comment
  (anchor covering one or more source lines) and one file-kind
  comment (anchor `internal/foo.go` lines 12-24)
- **THEN** the appendix output is:

  ```
  Comments on the message:
  - comment on "<excerpt>": <text>

  Comments on files:
  - file "internal/foo.go" (lines 12-24): <text>
  ```

#### Scenario: Single file-kind comment omits message section
- **WHEN** the comment slice contains exactly one file-kind
  comment
- **THEN** the appendix output is:

  ```
  Comments on files:
  - file "<path>" (lines X-Y): <text>
  ```

#### Scenario: Single message-kind comment omits files section
- **WHEN** the comment slice contains exactly one message-kind
  comment
- **THEN** the appendix output is:

  ```
  Comments on the message:
  - comment on "<excerpt>": <text>
  ```

#### Scenario: Empty comment slice emits empty appendix
- **WHEN** the comment slice is empty
- **THEN** the appendix output is the empty string

#### Scenario: Excerpt truncates long quoted text
- **WHEN** a message-kind comment's anchor
  `m.latest.Text[byteA:byteC]` is longer than 40 characters
- **THEN** the rendered `<excerpt>` in the appendix is the
  first 40 characters followed by `…`

#### Scenario: Excerpt collapses newlines and whitespace runs
- **WHEN** a message-kind comment's anchor
  `m.latest.Text[byteA:byteC]` contains one or more newline
  characters or runs of whitespace
- **THEN** the rendered `<excerpt>` in the appendix replaces
  each newline with a single space, collapses runs of
  whitespace to a single space, and trims leading and trailing
  whitespace

#### Scenario: File-kind and message-kind comments preserve chronological order within each section
- **WHEN** two message-kind comments are saved in order A then
  B, and two file-kind comments are saved in order C then D
- **THEN** the appendix output lists A before B in the message
  section and C before D in the files section, regardless of
  the comments' anchor positions