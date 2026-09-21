## Context

Pinky's main view is rendered in two stages. First, `internal/render`
turns the agent's markdown into ANSI-styled text via glamour and
records a `[]Block` index (`StartLine`, `EndLine`, `Kind`). When
comments exist, `RenderMessageWithComments` overlays two visual
treatments on top: a background tint over each commented block's
rendered range, and a `▸`/`•` gutter glyph on the first line of each
commented block (via `applyHighlights`). Footnote lines for each
comment are injected just after the block. The model package then
takes that rendered string and injects heavy pink `━━━` borders and a
magenta `┃` left bar around the focused block (`injectBorder` in
`model.go`), wrapping each focused-block line to replace glamour's
2-char left margin.

The result is loud. The focused block consumes a full terminal row
top and bottom for its border, a column on the left for its bar,
and the only color used for both chrome and selection is 212
magenta. Headings re-add the literal `#`/`##`/`###` characters via
glamour's `Prefix` config, eating 2–3 columns per heading in a tool
that lives in a tmux split and often sees ~60–100 columns total.

The plannotator-tui project (Rust + ratatui) is the visual reference
for this change. Its discipline: backgrounds are reserved for
selection and annotation; chrome carries its weight via bold and
underline; a single character of left gutter carries two facts via
color. Adopting that discipline is a presentation-only change — no
keymap, no data model, no state machine.

## Goals / Non-Goals

**Goals**

- Replace the pink border + left-bar focus indicator with a cyan
  left-gutter marker (▍) that spans the focused block's lines.
- Replace the `▸`/`•` in-block markers with a yellow left-gutter
  marker spanning the commented block's lines. Block-vs-inline
  distinction is preserved in the footnote line text, not the
  main gutter color.
- Render markdown headings via weight (bold; h1 also underlined)
  and color (cyan; h3 a softer cyan), without literal `#`
  prefixes in the rendered output.
- Keep the focused-block signal coherent with visual mode: the
  gutter tracks the visual cursor's block, same routing the border
  used.
- Keep the existing tinted background over commented line ranges.
  It is the only signal of *which lines* inside a block a comment
  covers; the gutter is block-level.

**Non-Goals**

- A right-rail annotations panel. Plannotator has one; pinky does
  not, and adding one changes the split-pane layout. Out of scope.
- A floating toolbar above selected ranges. Plannotator has one;
  pinky enters compose/comment-composer via the state machine,
  which is the territory of the in-flight `vim-keymap` change.
- A background tint on the focused block. Plannotator uses
  `Indexed 236`; reusing it would collide visually with the
  status bar (which is already 236). Skipped to keep the change
  minimal; add later if the gutter alone feels insufficient.
- Restructuring the `Include comments` toggle or the `[I]` key.
- Any change to keys, keymap surface, or input handling.

## Decisions

### 1. One gutter column, three states

The leftmost column of every rendered line is one of:

- `▍` cyan if the line is inside the focused block,
- `▍` yellow if the line is inside a commented block (regardless
  of focus),
- space otherwise.

The focused state takes precedence over the commented state so
a focused-and-commented block reads as "you're here" first and
"has feedback" second. Cyan and yellow are the two brightest
non-error colors available in the ANSI-256 space and read
clearly against both the default document background and the
237 background used by the commented-range tint.

**Alternatives considered**

- Two channels (▍ margin + ▸/• inside the block) — two ways to
  say "this block has comments." Rejected; redundant.
- Distinct marker chars for block vs inline (`▸` block, `•`
  inline) in the gutter, plannotator-style via color only —
  rejected because the data is already preserved in the
  footnote text and the in-block distinction is rarely actionable.

### 2. The gutter is drawn at the model layer, not the render layer

Pinky's render package is concerned with markdown → ANSI; it does
not know which block is "focused" (that's a cursor concept owned
by the model). The render package is the right place for the
yellow `▍` (it knows which blocks have comments, since it
populates `Block.HasComment` itself), but the cyan `▍` is
model-state. Two options were considered:

- **Both layers emit gutter characters, last writer wins.** The
  render layer emits `▍` yellow on commented blocks; the model
  layer overwrites with `▍` cyan on focused blocks. Rejected as
  fragile — a future change to line offsets could cause the
  model's overwrite to miss lines.
- **Model layer owns the entire gutter column.** The render layer
  stops emitting the `▸`/`•` first-line marker but keeps
  `Block.HasComment` and `tintLine`. The model layer walks the
  rendered output line-by-line and decides the gutter char per
  line from `focusedBlockIdx()` and `m.blocks[i].HasComment`.
  Selected. A single pass, a single source of truth.

### 3. Cyan discipline; magenta retained for brand accent

Three colors were already in active use: 212 (magenta), 228
(yellow), 51 (cyan, not currently used). Plannotator's model is
"selection color is the loudest non-error color, and chrome
colors are muted."

- The new selection color is cyan (51), used for the focused
  gutter, the visual-mode indicator chip, and the headings.
- Magenta (212) is retained as a brand accent for the picker
  header (the only screen where pinky introduces itself as
  "pinky"). It is no longer used for borders, the visual chip,
  or any selection context.
- Yellow (228) is retained for "comment" signals (the gutter
  and the existing h1 highlight; the h1 highlight was yellow
  before this change and stays so — it reads as "section
  break," not "selection").
- 87 (softer cyan) is used for h3 to give the headings a
  three-step weight/color hierarchy without introducing a new
  hue.

### 4. No `#` prefixes via glamour `Prefix: ""`

Glamour's parser strips the source `#`; the `Prefix` config
re-adds it during render. Setting `Prefix: ""` on h1/h2/h3
removes the re-added characters. This is the plannotator house
style and reclaims ~3 columns per heading — meaningful in a
narrow tmux split. No need to touch the markdown source.

### 5. Glamour `Margin: 2 → 1`

The current 2-char left margin is replaced by a 1-char left
margin plus the 1-char gutter, keeping total visible text width
roughly constant. The 1-char margin prevents lines from sitting
flush against the gutter on terminals where the gutter is
ambiguous-width (the `▍` glyph is 1 cell wide on every common
terminal; the 1-cell margin is a safety buffer).

### 6. Tests scoped to the gutter contract

`internal/render/border_test.go` (227 lines, all about the
removed borders) is deleted outright. `comment_render_test.go`
loses assertions that expect `▸`/`•` on the first rendered line
of commented blocks; assertions on footnote text content stay
(footnote text still contains the glyph). A new
`internal/render/gutter_test.go` covers four cases: focused
block (cyan), commented block (yellow), gap line (space), and
focused + commented (cyan wins).

No snapshot/visual regression test is added in this change.
Visual verification is a one-line smoke check during
implementation ("does the rendered output look right when I
press `j` through three blocks with one commented?"), then
moved on. If a regression test is wanted later, a small
"snapshot the gutter column" test on a fixture would do it.

## Risks / Trade-offs

- **Block vs inline distinction is no longer visible in the main
  gutter.** Preserved in the footnote text (`▸` vs `•`) and in
  the data (the `CharStart < 0` sentinel), but if a user relied
  on the visual distinction to triage comments, that signal is
  now hidden behind a footnote scroll. Mitigation: footnote
  text still distinguishes, and the comment composer shows the
  anchored range when editing.

- **`Comment.Marker()` stays alive but only feeds the footnote
  text.** A future contributor who doesn't read `comment_render.go`
  closely might add a second caller expecting gutter behaviour
  that no longer exists. Mitigation: keep the existing godoc on
  `Marker()` updated and the method private/internal if
  possible; cross-reference the change in a comment near the
  caller in `footnoteLines`.

- **Gutter char visibility on non-color terminals.** `▍` is a
  thin vertical bar; in a strict ASCII-only terminal it can
  collapse to a `|` or nothing. The colour carries the signal,
  not the glyph shape. Mitigation: ANSI 256 colour is the
  baseline expectation in any terminal that runs tmux + a TUI
  today; if the user has explicitly stripped colour, the gutter
  char still has visual weight from its width.

- **No visual regression test.** A subtle change to glamour's
  output (e.g., a `Prefix` value getting trimmed differently
  by a glamour upgrade) would not be caught by a unit test.
  Mitigation: a one-line smoke render during implementation;
  leave proper regression testing for if/when visual drift
  becomes a recurring problem.

- **The change touches both `internal/render` and `model` in the
  same PR.** Two-package blast radius is small but means
  partial-apply states are unlikely to be useful. Mitigation:
  tasks are ordered so render-package changes land before
  model-package changes, but a build break between commits is
  expected.

## Migration Plan

Single-PR change. The visual difference is large enough that
"deploy" is "merge and use." There is no on-disk data migration
(no persisted view state). The only user-visible change is
what they see on screen; no keymap or behaviour change
accompanies it.

Rollback is `git revert` of the merge commit; there is no
data that survives across the rollback.

## Open Questions

None at apply time. The two tensions we surfaced during
exploration (focused-block bg tint, fate of `Marker()`) were
resolved in the design: no bg tint in this change (T1=(a));
`Marker()` retained for footnote text (T2=(a)).