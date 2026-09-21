## REMOVED Requirements

### Requirement: Visual block-level annotation
**Reason**: The dedicated `m` key (block-level annotation) is
removed in favour of the unified `c` key. Pressing `c` with no
selection opens the comment composer pre-anchored to the whole
current block, which covers the same use case with one fewer
binding. The "block-level only" anchor shape
(`(BlockIdx, -1, -1)`) is replaced by the byte-range anchor shape
`(BlockIdx, charA, charC)` with `charA = 0, charC =
len(block.Source)`.
**Migration**: Users who previously pressed `m` should press `c`
(no prior selection required). The behaviour is identical from
the user's perspective: composer opens, full-block anchor, save
produces a block-level comment. See `single-cursor-nav` for the
canonical key binding.

### Requirement: Visual inline line selection
**Reason**: Visual mode is no longer a separate dispatch path. The
line-granularity selection cursor (which tracked rendered-line
indices) is replaced by the character-granularity cursor in
`single-cursor-nav`. Pressing `v` enters visual; `j`/`k`/`h`/`l`
extend the cursor and (in visual mode) the selection. The
previous `V` (uppercase) capital-mode and the line-level
visual-mode commands are gone.
**Migration**: Use `v` to enter visual, then `j`/`k` to extend
across blocks or `h`/`l` to nudge within a block. Press `c` to
commit the selection to a comment composer. See
`single-cursor-nav` for the canonical key binding.

### Requirement: Inline selection anchors by byte offset
**Reason**: The byte-offset anchor shape
(`(BlockIdx, CharStart, CharEnd)`) is now defined and used by
`single-cursor-nav`'s selection range. The block-marshal rules
and the source-text verbatim slice from this requirement are
preserved (see MODIFIED comment-storage below), but the
redundant copy in this capability is removed to avoid two
specs defining the same shape.
**Migration**: The byte-offset anchor shape is documented in
`single-cursor-nav`. The `Comment` record's `Source` field
remains the verbatim slice of the original markdown.

### Requirement: Auto-scroll during visual selection
**Reason**: The "auto-scroll when the cursor leaves the visible
range" rule now applies to all states in `stateNav`, not just
visual mode. The previous visual-mode-only auto-scroll is
generalised to "viewport follows cursor with a 1-line cushion"
in `single-cursor-nav`.
**Migration**: The behaviour still applies; it is now driven by
the unified cursor and applies in both visual and non-visual
modes. See `single-cursor-nav` for the rule.

### Requirement: Block-local edit and delete
**Reason**: The single-letter nav surface in `stateNav` does
not have room for `e` (edit), `d` (delete), `n` (next
commented block), or `N` (previous commented block) alongside
the required `j k h l v c s q r n ?`. With the explicit
direction to "prefer single key for all", the per-comment
lifecycle is collapsed: edit/delete/jump are out of scope for
this change and may return in a follow-up if needed.
**Migration**: Comments can be deleted by clearing all comments
on the current message arrival (`msgHash` flips; see
"in-memory storage only" below) and re-creating. There is no
in-app edit or per-comment delete at this time. Users with
existing comments who want to keep them across message updates
must send them first (`s` to batch-submit) before the message
changes.

## MODIFIED Requirements

### Requirement: Comment footnote rendering

The system SHALL render saved comments as footnote lines
immediately following the block they annotate. Each comment
footnote SHALL be prefixed with a comment marker (e.g., `▸`
for block-level comments, `•` for inline comments) and SHALL
display the comment text. The rendered line range of the
commented block SHALL also receive a background color tint to
make the commented region visually distinct. A left-gutter
marker (▸ for block-level, • for inline) SHALL be prepended
to the first rendered line of each commented block. The
selection range (when visual mode is active) SHALL receive a
background tint as well, layered under the comment tint and
the block border (selection < comment < border, in z-order).

#### Scenario: Footnote appears below commented block

- **WHEN** a block-level comment is saved
- **THEN** the rendered view shows the block's content followed
  by a footnote line containing the marker and the comment
  text, before the next block's content begins

#### Scenario: Inline comment footnote includes excerpt

- **WHEN** an inline comment is saved
- **THEN** the footnote line contains the inline marker (•) and
  the comment text, and the verbatim source excerpt stored
  with the comment is included in the footnote for context

#### Scenario: Gutter marker on first commented line

- **WHEN** a block has at least one comment
- **THEN** the first rendered line of that block is prepended
  with the appropriate gutter marker (▸ or •)

#### Scenario: Background tint on commented lines

- **WHEN** a block has at least one comment
- **THEN** every rendered line within that block's
  `StartLine..EndLine` range receives the comment background
  color tint

#### Scenario: Selection tint layered under comment tint

- **WHEN** the user is in visual mode with an active selection
  that overlaps a commented block
- **THEN** the rendered lines at the intersection show both the
  selection tint (background of the nav range) and the comment
  tint (background of the comment range); the layering SHALL be
  selection first, comment on top, with the border on top of
  both

#### Scenario: Clear selection removes selection tint only

- **WHEN** the user exits visual mode (via `v` or `Esc`)
- **THEN** the selection range is cleared and the selection
  tint is removed; comment tints and block borders SHALL
  remain

### Requirement: In-memory storage only

Comments SHALL be stored in memory on the model and SHALL NOT
be written to disk. When the assistant message content
changes (the `latest.text` field is replaced by a new
assistant message), all existing comments SHALL be discarded.
When the TUI detaches from the session or quits, all comments
SHALL be discarded.

The anchor shape for each comment SHALL be
`(BlockIdx, charA, charC)`. Block-level comments SHALL use
`charA = 0, charC = len(block.Source)` (the whole block).
Inline comments SHALL use the byte offsets from the active
selection at the moment the user pressed `c`. The verbatim
source slice `block.Source[charA:charC]` SHALL be stored
alongside the comment so the redirect appendix can quote the
original text without re-parsing.

#### Scenario: New message clears comments

- **WHEN** a new assistant message arrives and replaces the
  currently-displayed message
- **THEN** all existing comments are discarded and the rendered
  view shows no comment footnotes or highlights

#### Scenario: Detach clears comments

- **WHEN** the TUI detaches from the current session
- **THEN** all existing comments are discarded

#### Scenario: Quit clears comments

- **WHEN** the user quits the TUI
- **THEN** no comment state is preserved; the next attach
  starts with zero comments

#### Scenario: Block-level `c` save yields a whole-block anchor

- **WHEN** the user presses `c` with no selection active, types
  a comment in the composer, and saves (Ctrl+S)
- **THEN** the stored comment has `charA = 0, charC =
  len(block.Source)` for the cursor's block

#### Scenario: Inline `c` save yields a byte-range anchor

- **WHEN** the user presses `v`, navigates with `j`/`k`/`h`/`l`
  to extend a selection, presses `c`, and saves
- **THEN** the stored comment has `charA` and `charC` equal to
  the selection's min / max byte offsets within the (possibly
  multi-block) range; the `Source` field stores the verbatim
  slice covering the selection
