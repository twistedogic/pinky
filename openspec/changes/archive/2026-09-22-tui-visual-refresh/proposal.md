## Why

Pinky's rendered view leans on heavy pink horizontal borders (━━━)
above and below the focused block, a magenta left bar (┃) on the
focused block's lines, and magenta headings with literal `#`/`##`/
`###` prefixes. The visual weight is concentrated in one loud accent
(212 magenta) and competes with the content for attention. The
[plannotator-tui](https://github.com/plannotator/plannotator-tui)
house style offers a calmer alternative: a single character of left
gutter (▍) that carries two facts (focused vs. has-annotations) via
color, headings styled by weight rather than literal markers, and a
discipline that reserves background fills for selection rather than
chrome. Adopting that discipline for pinky is a visual refresh —
no keymap changes, no data model changes — that makes the agent's
text the dominant thing on screen.

## What Changes

- Replace the heavy pink `━━━` borders and `┃` left bar with a
  one-character left gutter: `▍` cyan spans every line of the
  focused block, `▍` yellow spans every line of a block that has
  saved comments, and a single space otherwise. Border removal is
  the single largest visual change. The gutter color follows the
  visual cursor when visual mode is active (same routing logic
  the border used).
- Drop the `▸` (block-level) and `•` (inline) gutter markers
  prepended to the first rendered line of commented blocks. Block
  vs. inline is no longer distinguished in the main gutter color;
  both classes of comments render as yellow ▍. The `▸`/`•`
  glyphs remain on the footnote lines below commented blocks
  (the text format of the footnote does not change) and the
  `Comment.Marker()` method stays in place to produce them.
- Render markdown headings with weight (bold; h1 also underlined)
  and color (cyan; h3 a softer cyan), without prefixing the
  rendered heading with literal `#` characters from the source.
- Glamour's left margin drops from 2 to 1 to keep total left
  indent visually equivalent (the gutter consumes the freed column).
- The visual-mode indicator chip in the status bar changes
  background from magenta (212) to cyan (51), so it matches the
  new selection color.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `latest-message-view`: heading rendering no longer prefixes
  rendered output with `#` markers; a new requirement specifies
  the cyan, weight-driven heading style. A new requirement
  specifies the cyan left-gutter focus indicator (replacing the
  prior "border" behavior, which is removed without a separate
  REMOVED requirement because the border is presentation, not
  a user-facing requirement — its migration is "the gutter
  replaces it").
- `message-comments`: comment footnote rendering no longer
  prepends a gutter marker to the first rendered line of
  commented blocks; the left-margin gutter color (yellow) is
  the sole in-block annotation signal. The background tint
  over commented line ranges is unchanged. The existing
  requirements about the border are re-scoped to the gutter.

## Impact

**Code touched**

- `internal/render/render.go` — `Block` gains `HasComment bool`;
  populated by `RenderMessageWithComments`. `Comment.Marker()`
  stays.
- `internal/render/comment_render.go` — drop the
  `gutterMarkerANSI` + `markerByBlock` injection from
  `applyHighlights`; keep `tintLine` background. The
  `footnote.marker` field and the footnote line text are
  unchanged.
- `internal/render/style.go` — h1/h2/h3 `Prefix` → empty
  strings; h1 cyan bold underlined; h2 cyan bold; h3 soft-cyan
  bold; `Margin` 2 → 1.
- `model.go` — delete `injectBorder`, `trimLeadingVisible`,
  `hasCommentGutter`, `borderStyle`. Add `injectGutter` (single
  pass over the rendered output, gutter char chosen per line
  via `focusedBlockIdx()` + a per-block `hasComment` map). Update
  `visualModeStyle` background 212 → 51.
- `internal/render/border_test.go` — delete (whole file).
- `internal/render/comment_render_test.go` — remove assertions
  that expect `▸`/`•` on the first line of commented blocks.
  Keep assertions on footnote text (which still contains the
  glyph).
- `internal/render/gutter_test.go` — new file. Covers the
  focused, commented, both, and gap-line cases for `injectGutter`
  and the cyan/yellow/space colour choices.

**Behaviour changes visible to users**

- The heavy pink borders around the focused block are gone; the
  cyan left gutter is the only focus indicator in the document
  area.
- Headings are styled by color and weight without `#` prefixes
  in the rendered output, freeing ~3 characters per heading and
  matching the plannotator house style.
- A commented block no longer carries `▸` or `•` on its first
  line; the yellow gutter is the only annotation signal in the
  block body. Footnote lines below the block keep the
  block/inline distinction in their leading glyph.

**Out of scope**

- Keymap changes (separate `vim-keymap` change is in flight).
- Right-rail annotations panel (plannotator pattern; not
  applicable to pinky's split-pane layout).
- Floating toolbar over selections (plannotator pattern;
  requires state-machine work that overlaps with `vim-keymap`).
- Background tint on the focused block (plannotator uses
  `Indexed 236`; skipped here to keep the change small — the
  gutter alone carries the focus signal).
- The `Include comments` toggle in the status bar; not
  restructured in this change.