## Why

Pinky's current model is a scrollback viewer: it accumulates every captured agent
message and renders the whole transcript in a viewport. The message that matters
most — the one the user needs to read and respond to — is the latest agent
output, but it sits at the bottom of a wall of older context that the user has
to scroll past. Markdown in that transcript renders as undifferentiated gray
text, so headings, code blocks, lists, and emphasis are invisible. Long agent
messages become painful to navigate because the viewport scrolls line-by-line
through a 100-line message.

This change makes pinky a focused single-message viewer: show only the latest
agent message, render it as markdown, and navigate within it by markdown
structure rather than by raw line.

## What Changes

- Pinky shows the latest complete agent message in the main view. Older
  messages are no longer displayed.
- Agent text is rendered as markdown (headings, bold, italic, inline code,
  fenced code blocks, lists, blockquotes) with word wrap to viewport width,
  using `github.com/charmbracelet/glamour`.
- Navigation operates on markdown AST blocks: `j`/`k` move by line, `}`/`{`
  jump between blocks (heading, paragraph, code block, list-item, blockquote),
  `]]`/`[[` jump between headings, `gg` jumps to the top of the message,
  `G` jumps to the bottom.
- The currently-focused block is marked by horizontal border lines above and
  below it, in the picker-header accent color (212).
- Arrow keys and `PgUp`/`PgDn` are kept as aliases for the vim-style keys
  (`↑`/`↓` → `j`/`k`; `PgUp`/`PgDn` → `}`/`{`). Existing muscle memory
  keeps working.
- The history file continues to be appended (raw entries, unchanged format)
  but is no longer re-seeded into the view on startup. Pinky starts fresh and
  picks up the latest message from the live session.
- User redirects are no longer displayed in the main view after being sent.
  The compose textarea acknowledges the send; the agent's response shows up
  when it arrives.
- **BREAKING**: Scrollback navigation is removed. `PgUp` now means "top of
  current message" (no longer "scroll back through history"). The README's
  v0 out-of-scope item "Markdown rendering" becomes in-scope.

## Capabilities

### New Capabilities

- `latest-message-view`: focused single-message viewer with markdown
  rendering, markdown-AST-block navigation, vim keybindings, and a
  current-block border indicator.

### Modified Capabilities

- `agent-redirect`: the "Persist and restore history" requirement changes —
  the history file continues to be appended (for audit/debug) but the
  on-startup scrollback re-seed is removed. The view shows the latest
  agent message from the live session only.

## Impact

- `model.go`: replace accumulating `m.entries []entry` with a single
  `m.latest entry`. Rewrite `refreshViewport` to render one message via
  glamour, build a block index, inject border lines, and handle vim keys.
- `internal/history`: keep `history.Append`, drop `history.Load` from the
  attach path. The file format is unchanged so existing files remain valid.
- New dependency: `github.com/charmbracelet/glamour` for markdown rendering.
  `github.com/yuin/goldmark` is pulled in transitively (glamour depends on
  it) and used directly to walk the AST for block boundaries.
- README: remove "Markdown rendering" from the v0 out-of-scope list.
- No new external services. No changes to the session JSONL parsing path
  (`internal/session` is untouched).
