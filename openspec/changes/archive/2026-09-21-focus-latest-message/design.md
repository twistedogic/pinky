## Context

Pinky currently tails a pi or codex agent's session JSONL and surfaces every
assistant text message into a scrollback view (`m.entries []entry` in
`model.go`). The view accumulates entries indefinitely and the user scrolls
line-by-line through the whole transcript. Agent text is rendered with a
single lipgloss foreground color and a 2-space prefix — headings, code
blocks, lists, and emphasis are visually indistinguishable from prose.

The change converts pinky from a transcript viewer into a focused single-
message viewer: show only the latest agent message, render it as markdown,
and let the user navigate within it by markdown structure.

## Goals / Non-Goals

**Goals:**

- Show one message at a time — the latest assistant message from the agent.
- Render that message as markdown with terminal styling and width-aware wrap.
- Provide markdown-AST-block navigation: jump between blocks, jump between
  headings, jump to top/bottom.
- Mark the currently-focused block so the user knows where they are.
- Use vim-style keys (`j`/`k`/`}`/`{`/`gg`/`G`) as the primary navigation,
  with arrow keys and `PgUp`/`PgDn` as aliases for muscle-memory continuity.

**Non-Goals:**

- Multi-message scrollback or history navigation within pinky.
- Syntax-highlighted code blocks (whatever glamour's preset provides is
  accepted; no explicit syntax-highlighting dependency).
- Customizable glamour style (one preset shipped, no config file).
- Detection of "agent done speaking" — every JSONL line pinky reads is
  already complete, so the streaming/complete distinction is invisible at
  the message level.
- Turn-level grouping (multi-block turns show as a sequence of blocks, not
  one merged "turn").
- Hyperlink-following, search, fold/unfold, copy-hotkey.
- Other agents (pi/codex only, unchanged).
- Other multiplexers (tmux only, unchanged).

## Decisions

### D1. Single-message data model

Replace `m.entries []entry` with `m.latest entry`. On each `sessionMsg`,
replace `m.latest` with the last message from the poll (don't append).

**Why:** The scrollback accumulation is the root of the UX problem. Dropping
it simplifies the model, the rendering pipeline, the block index, and the
streaming behavior. The history file continues to capture everything for
audit, but is never read back into the view.

**Alternative considered:** Keep `m.entries` for scrollback but change
default view to "latest only" with scroll-up access. Rejected — it preserves
the complexity without giving up anything meaningful, since the scrollback
is no longer the primary view.

### D2. Markdown rendering: glamour with the `dark` preset

Add `github.com/charmbracelet/glamour`. Build a `TermRenderer` per width
with `glamour.WithStandardStyle("dark")` and `glamour.WithWordWrap(width)`.

**Why:** Charm family (same maintainer as bubbles/bubbletea/lipgloss),
designed for the exact use case, supports width wrap, includes heading/
code/list/blockquote styling out of the box.

**Alternatives considered:**
- `goldmark` raw: parses to AST, but no terminal renderer. Glamour uses it
  internally; we use glamour directly for output and goldmark directly for
  the block index.
- DIY renderer: would cover a narrow subset. Don't reinvent what glamour
  already does.

### D3. Block index from goldmark AST

Use `github.com/yuin/goldmark` (transitive via glamour, but accessed
directly) to parse the markdown text into an AST. Walk top-level block
nodes, filter empty ones, and classify by kind: `heading`, `paragraph`,
`code`, `list-item`, `quote`. For each non-empty block, record
`(kind, startLine, endLine)` against the line indices of the rendered
output.

**Why:** Glamour does not expose block boundaries in its rendered output.
We need those boundaries to drive `}`/`{` and the border indicator.
Walking the AST directly is correct, cheap, and stays in the charm/goldmark
family.

**Concrete shape:**

```go
type blockKind string
const (
    blockHeading   blockKind = "heading"
    blockParagraph blockKind = "paragraph"
    blockCode      blockKind = "code"
    blockListItem  blockKind = "list-item"
    blockQuote     blockKind = "quote"
)

type block struct {
    kind      blockKind
    startLine int  // 0-indexed line in rendered output
    endLine   int  // inclusive
}
```

**Block discovery:** render each AST block individually with glamour, count
lines, accumulate `startLine`/`endLine`. This means glamour parses the same
text twice (once globally, once per block), but glamour is fast enough
(sub-millisecond for typical agent messages) and the simpler implementation
is worth the small duplication.

**List granularity:** each `ListItem` is its own block. A 10-item list
produces 10 navigation targets, not 1. Without this granularity, lists
become useless as jump targets.

**Skipped:** empty paragraphs, thematic breaks (`---`), HTML blocks,
tables (treated as a single block — no per-row navigation).

### D4. Vim keybindings with two-key state

Primary navigation keys (idle state):

| Key | Action |
|---|---|
| `j` | line down |
| `k` | line up |
| `}` | next block |
| `{` | previous block |
| `]]` | next heading |
| `[[` | previous heading |
| `gg` | top of message |
| `G` | bottom of message |

Aliases: `↑`/`↓` → `j`/`k`; `PgUp`/`PgDn` → `}`/`{`.

**Two-key state:** `m.lastVimKey rune` and `m.lastVimKeyAt time.Time`.
A key clears the state if it doesn't match the expected continuation or
the 500ms timeout has passed. State is reset to zero on any non-vim key
(arriving through the regular handler) and on entering compose mode.

**Why 500ms:** matches vim's `timeoutlen=500` default. Fast enough for
normal typing, slow enough that the user can pause between keys without
losing the state.

**Compose mode:** no vim keys. `j`/`k` there type those characters. The
existing textarea handles its own input.

### D5. Border indicator: top + bottom only

Draw a horizontal `─` line above the first line of the current block and
below the last line. Color: lipgloss `212` (matches the picker-header
accent). Width: viewport width.

**Why not a full box:** drawing left/right borders around already-glamour-
styled content would require either re-parsing ANSI or wrapping glamour
output in a lipgloss border (which fights glamour's width handling).
Top + bottom is a clean two-line injection.

**Why not a side gutter marker (`▌`):** equivalent signal, slightly more
visual noise per line. Top + bottom is simpler and reads as "this is a
region."

### D6. Streaming: re-render on every chunk, yank to bottom

When a new text block from the agent arrives, update `m.latest.text` and
re-render the viewport. After re-render, set `m.viewport.GotoBottom()`.

**Yank-to-bottom behavior:** the user is at the bottom by default. New
content appears at the bottom, so they see it. If the user has scrolled
up to read earlier in the message, a new chunk yanks them back to the
bottom. This matches the current scrollback behavior (where new entries
also yanked to the bottom) and is the principle of least surprise for
the redesign. Revisit if it becomes a complaint (see Open Questions).

### D7. History file: keep appending, stop re-seeding

`internal/history.History.Append` continues to be called for each surfaced
message. The file format (one JSON object per line: `ts`, `pane_id`,
`role`, `text`) is unchanged. `history.Load` is removed from the attach
path — pinky does not read history on startup.

**Why keep the file:** audit, debugging, possible future use. The cost is
negligible (one append per message, append-only file).

**Why drop the re-seed:** the view shows only the latest live message; the
history file's contents are no longer relevant to the view. Removing the
load simplifies startup and removes a class of "why is pinky showing stale
content" bugs.

### D8. Empty state: dim placeholder

When `m.latest` is empty (no message yet for this session), render a
single dim-styled line: `waiting for agent…` centered or left-aligned.
No blocks, no border, no viewport scroll.

**Why:** without a placeholder, the empty viewport would be ambiguous
(working? waiting? broken?). A one-line placeholder answers the question.

### D9. Width handling: rebuild renderer on `WindowSizeMsg`

Cache glamour renderer by width in a `map[int]*glamour.TermRenderer`. On
`WindowSizeMsg`, clear the cache and rebuild for the new width. On each
`refreshViewport`, look up the renderer for `m.width`.

**Why cache:** glamour's `NewTermRenderer` is cheap but not free.
Caching avoids rebuilding on every refresh tick.

**Why a map (vs single renderer with width setter):** glamour's API takes
width at construction, not as a per-render parameter. Caching by width is
the natural fit.

## Risks / Trade-offs

- **[Streaming markdown is partial]** Glamour renders partial markdown
  gracefully but cross-chunk spans (e.g. `**bold` split across two
  appends) can render mid-stream without their closing style. → Mitigation:
  re-render on every chunk; visual artifacts self-correct within one poll
  cycle.

- **[Block boundaries shift during streaming]** As new content arrives, the
  AST re-parses and a paragraph may grow or split into a heading. The
  border follows the current block by absolute line index, so it stays
  attached to the right content as boundaries shift. → Mitigation: the
  `currentBlockIdx()` lookup runs against the freshly-rendered block
  index, so the indicator stays correct.

- **[Yank-to-bottom on stream is hostile to mid-message readers]** A user
  scrolling up to re-read a paragraph gets yanked back when the next
  chunk arrives. → Mitigation: this matches the prior scrollback behavior;
  surface as an open question for follow-up if it bites.

- **[Glamour ANSI + lipgloss ANSI]** Glamour output contains ANSI escapes;
  lipgloss styles do too. Alternating them in the same viewport is safe
  (lipgloss handles ANSI in inputs to its own styles), but a future
  refactor that nests them more tightly could produce stray codes. →
  Mitigation: only top + bottom borders go through lipgloss; block content
  goes through glamour unchanged.

- **[Per-message double-parsing]** Glamour parses internally; we also parse
  via goldmark for the block index. For a 1000-line message, this is two
  full parses per refresh tick (every 500ms while streaming). → Mitigation:
  glamour parses are cheap (~sub-millisecond); measured cost is acceptable.
  Revisit if profiling shows otherwise.

- **[History file grows unbounded]** History still appends for every
  surfaced message, with no rotation. → Mitigation: not new — pre-existing
  behavior. Out of scope for this change.

- **[Breaking change to scrollback users]** Anyone relying on PgUp to
  review earlier messages loses that capability. → Mitigation: the
  proposal is explicit about this; release notes must call it out.

## Migration Plan

No data migration. The history file format is unchanged, so existing
files remain readable by `jq` and other tools. Pinky's runtime behavior
changes — users see one message instead of a scrollback. No settings to
preserve (pinky has no config file).

Rollback: revert the commit. No state to restore.

## Open Questions

1. **Glamour style preset.** `dark` is the obvious default and matches a
   dark terminal. Should we define a custom glamour style aligned with the
   lipgloss palette (color 250 for body text, 42 for user, 212 for the
   border/header)? Lean toward `dark` preset for v1 — less code, less to
   maintain.

2. **Auto-follow while streaming.** Should `viewport.GotoBottom()` only
   fire when the user was already at the bottom (i.e. preserve their scroll
   position when they've scrolled up)? Lean toward "always yank" for v1
   (matches prior behavior); revisit if it bites.

3. **`Ctrl+R` manual refresh.** In the scrollback model it cleared and
   re-polled. In the focused model it would re-poll for new messages. Is
   the key still meaningful, or vestigial? Lean toward "keep, behavior
   is just re-poll now" — it's harmless and keeps the muscle-memory
   continuity.

4. **Table blocks.** The AST walk treats a markdown table as one block.
   Should we add per-row navigation? Lean: no — tables are rare in agent
   output, and per-row granularity is a v2 ask.
