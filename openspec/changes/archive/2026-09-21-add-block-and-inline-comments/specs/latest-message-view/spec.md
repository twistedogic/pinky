## ADDED Requirements

### Requirement: Render comment annotations as overlay

The latest-message view SHALL be capable of rendering comment
annotations on top of the rendered assistant message. The annotation
overlay consists of:

- A footnote line for each saved comment, rendered immediately
  below the block it annotates and before the next block begins.
- A background color tint covering the rendered line range of each
  commented block.
- A left-gutter marker (▸ or •) prepended to the first rendered
  line of each commented block.

The annotation overlay SHALL NOT modify the assistant message text
itself; the markdown rendering of the assistant message is
unchanged. The overlay is composed after markdown rendering as a
post-processing pass on the rendered line ranges, using the block
index from "Build markdown block index" as the line-range source.

#### Scenario: Comented block shows footnote below

- **WHEN** a saved block-level comment exists for a block in the
  rendered message
- **THEN** the main view shows the rendered block, immediately
  followed by a footnote line containing the marker and the comment
  text, before the next block's first line

#### Scenario: Comented block shows gutter marker

- **WHEN** a saved comment exists for a block
- **THEN** the first rendered line of that block is prepended with
  the appropriate gutter marker

#### Scenario: Annotation overlay survives width reflow

- **WHEN** the terminal width changes and the rendered message is
  re-flowed
- **THEN** the comment annotations (footnotes, gutter markers, and
  background tints) are re-applied to the new line ranges without
  loss; no annotation references a stale line index from the
  previous width

### Requirement: Always-visible help footer

The latest-message view SHALL render a one-line keymap footer at
the bottom of every state's view (`s` picker / idle / compose /
comment-composer / error). Pressing `?` SHALL expand the footer
into a multi-column full-help view; pressing `?` again SHALL
collapse it back. The footer SHALL be sourced from the bubbles
`help.Model` package and SHALL satisfy the `help.KeyMap` interface
with state-aware `ShortHelp()` and `FullHelp()` methods. The
viewport SHALL be shortened by the footer's height (1 line for
short, N lines for the largest full-help group) so the footer
never overlaps the message content.

#### Scenario: Idle view shows short help
- **WHEN** the TUI is in idle state
- **THEN** the bottom of the view contains a one-line keymap
  footer listing the most relevant keys for the current state

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
