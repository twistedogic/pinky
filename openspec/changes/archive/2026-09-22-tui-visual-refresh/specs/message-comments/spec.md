## MODIFIED Requirements

### Requirement: Comment footnote rendering

The system SHALL render saved comments as footnote lines
immediately following the block they annotate. Each comment
footnote SHALL be prefixed with a comment marker (`▸` for
block-level comments, `•` for inline comments) and SHALL display
the comment text. The rendered line range of the commented block
SHALL also receive a background colour tint to make the commented
region visually distinct. The commented block SHALL be marked in
the left-margin gutter column with the character `▍` in yellow
(foreground colour `228`) on every rendered line that falls inside
the block's `StartLine..EndLine` range, regardless of whether the
block is currently focused. No first-line `▸` or `•` glyph SHALL
be prepended to the block's first rendered line in the document
body.

#### Scenario: Footnote appears below commented block
- **WHEN** a block-level comment is saved
- **THEN** the rendered view shows the block's content followed
  by a footnote line containing the `▸` marker and the comment
  text, before the next block's content begins

#### Scenario: Inline comment footnote includes excerpt
- **WHEN** an inline comment is saved
- **THEN** the footnote line contains the `•` marker and the
  comment text, and the verbatim source excerpt stored with the
  comment is included in the footnote for context

#### Scenario: Yellow gutter spans the commented block
- **WHEN** a block has at least one comment
- **THEN** every rendered line within that block's
  `StartLine..EndLine` range has a yellow `▍` in its leftmost
  column

#### Scenario: No first-line gutter marker inside the block
- **WHEN** a block has at least one comment and is not the
  currently-focused block
- **THEN** the block's first rendered line does NOT begin with
  a `▸` or `•` glyph; the only annotation signal in the block
  body is the yellow gutter

#### Scenario: Background tint on commented lines
- **WHEN** a block has at least one comment
- **THEN** every rendered line within that block's
  `StartLine..EndLine` range receives the comment background
  colour tint

### Requirement: Border and comment highlight coexist

> **Note:** The prior version of this requirement described the
> heavy horizontal border around the focused block. The border
> has been replaced by the cyan left-gutter focus indicator
> defined in the `latest-message-view` spec
> (`### Requirement: Block focus indicator`). This requirement
> is preserved because the *coexistence* rule still applies
> (the gutter's cyan state and the comment tint's yellow gutter
> can both apply to the same block).

When the currently-focused block is also a commented block, both
the cyan focus gutter and the yellow comment gutter SHALL be
applied to the block's lines. The focus signal (cyan) SHALL take
precedence over the comment signal (yellow): the gutter character
for a focused-and-commented block SHALL be `▍` cyan. The
background tint over the commented line range SHALL still cover
the block's content lines regardless of focus.

#### Scenario: Focused commented block shows cyan gutter over tint
- **WHEN** a block is both the currently-focused block and has
  comments
- **THEN** the rendered view shows the cyan `▍` gutter on every
  line in the block's range, and the background tint over those
  lines is unchanged

### Requirement: Visual cursor drives the gutter highlight

While in visual line mode, the cyan left-gutter focus indicator
SHALL track the visual cursor's currently-focused block, not the
viewport-driven `YOffset`. As the visual cursor moves between
blocks (via `j`/`k`/`}`/`{`) the cyan gutter SHALL move with it.
The visual mode handler MUST short-circuit `j`/`k`/`}`/`{`
before the idle-view line-movement bindings can match them;
otherwise the visual cursor would never advance and the gutter
would stay on whatever block `viewport.YOffset` happened to be on.

#### Scenario: } moves gutter to next block
- **WHEN** the user presses `}` in visual line mode and there is
  a next block
- **THEN** the cyan gutter moves to the next block, regardless of
  where `viewport.YOffset` sits

#### Scenario: j leaves gutter in place within the same block
- **WHEN** the user presses `j` in visual line mode and the
  visual cursor stays within the same block
- **THEN** the cyan gutter stays on the same block (the cursor
  moved within it)

#### Scenario: Esc returns gutter to viewport-driven block
- **WHEN** the user presses `Esc` in visual line mode
- **THEN** the cyan gutter returns to the block containing
  `viewport.YOffset` (the normal idle-view behavior)

### Requirement: Visual mode indicator chip

While visual line mode is active the status line SHALL display a
distinct "VISUAL" chip in addition to the normal status content,
so the user has unambiguous feedback that `V` was registered. The
chip SHALL use a high-contrast style (foreground `232`, background
`51` — cyan — bold) so it stands out from the dim status
background and is consistent with the cyan selection colour used
by the document gutter.

#### Scenario: V toggles indicator on
- **WHEN** the user presses `V` in idle state
- **THEN** the status line shows the "VISUAL" indicator (cyan
  background, dark foreground, bold) while visual mode is active

#### Scenario: Esc clears indicator
- **WHEN** the user presses `Esc` in visual line mode
- **THEN** the status line returns to its non-visual content (no
  "VISUAL" chip)

#### Scenario: Indicator does not appear outside visual mode
- **WHEN** the TUI is in any non-visual state (idle, compose,
  picker, error)
- **THEN** the status line SHALL NOT show the "VISUAL" chip