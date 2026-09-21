## REMOVED Requirements

### Requirement: Vim-style navigation
**Reason**: Superseded by the `single-cursor-nav` capability
introduced in this change. The two-key state machine (`gg`, `]]`,
`[[`), the `G` bottom shortcut, the `{`/`}` block jumps, and the
line-granularity `j`/`k` are removed; nav now uses `j`/`k` for block
steps and `h`/`l` for rune steps, with no two-key sequences.
**Migration**: See `single-cursor-nav` for the new keymap and the
cursor-driven dispatch.

### Requirement: Arrow and page keys alias vim keys
**Reason**: With the nav surface collapsed to single-letter keys,
the aliasing rules change. Arrow and page keys are not aliased to
the new keymap; existing muscle memory (↑/↓/PgUp/PgDn) still
scrolls, but with new semantics (↑/↓ = single-line scroll, PgUp =
top, PgDn = next block).
**Migration**: Use the new single-letter keys; arrows / page keys
remain as scroll-only fallbacks. See `single-cursor-nav` for the
canonical surface.

## MODIFIED Requirements

### Requirement: Current-block border indicator

The system SHALL draw a horizontal `─` border line above the first
line and below the last line of the currently-focused block. The
currently-focused block SHALL be the block whose index equals
`model.cursor.blockIdx` (the single navigation cursor). The
border SHALL be styled in lipgloss color `212` (the picker-header
accent) and SHALL span the viewport width. The border SHALL be
re-baked whenever the cursor moves or the viewport re-renders, so
the highlight always surrounds the cursor's block.

#### Scenario: Border above and below the cursor's block

- **WHEN** the cursor's `blockIdx` is `i`
- **THEN** a `─` border line is rendered immediately above the
  first line and immediately below the last line of block `i`

#### Scenario: Border follows the cursor across motion

- **WHEN** the cursor moves from block `i` to block `i + 1` (via
  `j`, `k`, or any motion that changes `blockIdx`)
- **THEN** the border lines are removed from block `i` and
  re-emitted around block `i + 1` without an interim render that
  shows the wrong block bordered

#### Scenario: No border in empty state

- **WHEN** the main view shows the empty-state placeholder (no
  blocks or no current cursor block)
- **THEN** no border lines are rendered

### Requirement: Always-visible help footer

The latest-message view SHALL render a one-line keymap footer at
the bottom of every state's view (picker / **nav** (renamed from
`idle`) / compose / comment-composer / error). Pressing `?` SHALL
expand the footer into a multi-column full-help view; pressing `?`
again SHALL collapse it back. The footer SHALL be sourced from the
bubbles `help.Model` package and SHALL satisfy the `help.KeyMap`
interface with state-aware `ShortHelp()` and `FullHelp()` methods.
The viewport SHALL be shortened by the footer's height (1 line for
short, N lines for the largest full-help group) so the footer
never overlaps the message content.

The nav-state footer SHALL list the surface from
`single-cursor-nav`: a single `nav` group covering `j k h l v Esc c
s q r n ?`. Compose SHALL list `Enter s Esc Ctrl+I`. Comment
composer SHALL list `Ctrl+S Esc`. The previous groupings (`mark`,
`visual`, `comments` as separate nav groups) are merged into the
single `nav` group.

#### Scenario: Nav view shows short help

- **WHEN** the TUI is in `stateNav` (renamed from `stateIdle`)
- **THEN** the bottom of the view contains a one-line keymap
  footer listing the most relevant keys for nav (top of the
  single-letter surface)

#### Scenario: ? expands to full help

- **WHEN** the user presses `?` in any state
- **THEN** the footer expands into a multi-column full-help view
  showing every keybinding for the current state, grouped by
  category; pressing `?` again collapses it back to the short
  footer

#### Scenario: Help footer reserves viewport space

- **WHEN** the footer is shown
- **THEN** the message viewport is shortened by the footer's
  height (1 line for short, N lines for the largest full-help
  group) so the footer never overlaps the message content

#### Scenario: Compose and error states also show help

- **WHEN** the TUI is in compose / comment-composer / error /
  picker state
- **THEN** the help footer is still rendered at the bottom of the
  view with state-appropriate bindings

#### Scenario: Nav full help collapses the old separate groups

- **WHEN** the user expands the help overlay (`?`) from `stateNav`
- **THEN** the full-help view shows exactly one `nav` group
  containing the single-letter surface (`j k h l v Esc c s q r n
  ?`); there SHALL NOT be separate `mark`, `visual`, or
  `comments` groups
