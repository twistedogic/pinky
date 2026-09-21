## 1. Render package: expose `HasComment` on Block

- [ ] 1.1 Add `HasComment bool` field to `render.Block` in
  `internal/render/render.go`.
- [ ] 1.2 Populate `HasComment` from the `comments` slice in
  `RenderMessageWithComments` (any `c.BlockIdx == i` sets
  `blocks[i].HasComment = true`).
- [ ] 1.3 Add a small unit test in `render_test.go` covering the
  "no comments → all `HasComment` false" and "one comment on
  block N → `blocks[N].HasComment` true and others false" cases.

## 2. Render package: drop the in-block `▸`/`•` marker

- [ ] 2.1 In `internal/render/comment_render.go`, remove the
  `gutterMarkerANSI` constant, the `markerByBlock` map, and the
  branch inside `applyHighlights` that prepends the marker to the
  block's first rendered line. Keep `tintLine` and the background
  tint behaviour.
- [ ] 2.2 In `internal/render/comment_render_test.go`, remove
  the assertions that expect `▸` or `•` to appear on the first
  rendered line of commented blocks. Keep the assertions on
  footnote text content (the footnote still contains the glyph
  via `Comment.Marker()`).
- [ ] 2.3 In `internal/render/render.go`, update the godoc on
  `Comment.Marker()` to note that it now feeds the footnote text
  only (no in-block gutter usage).

## 3. Render package: style changes (headings + margin)

- [ ] 3.1 In `internal/render/style.go`, set `Prefix` to the
  empty string for `H1`, `H2`, and `H3`.
- [ ] 3.2 Change `H1` colour to `"#00d7d7"` (cyan `51`), keep
  bold, add underline.
- [ ] 3.3 Change `H2` colour to `"#00d7d7"` (cyan `51`), keep
  bold; drop the prefix only (no underline).
- [ ] 3.4 Change `H3` colour to `"#5fffff"` (softer cyan `87`),
  keep bold; drop the prefix only.
- [ ] 3.5 Change `Document.Margin` from `uintPtr(2)` to
  `uintPtr(1)`.
- [ ] 3.6 Add a small test in `style_test.go` that asserts the
  `StyleConfig` reflects the new prefix, colour, and margin
  values.

## 4. Model package: replace `injectBorder` with `injectGutter`

- [ ] 4.1 Delete `borderStyle`, `injectBorder`, `trimLeadingVisible`,
  and `hasCommentGutter` from `model.go`.
- [ ] 4.2 Add `injectGutter(rendered string) string` in `model.go`
  that walks the rendered output line-by-line, computes the
  block index for each line (via a new `blockIdxAtLine` helper or
  a search over `m.blocks`), and prepends a 2-character prefix
  per line: `▍` cyan if the line is inside the focused block
  (per `focusedBlockIdx()`), `▍` yellow if the line is inside a
  block with `HasComment == true`, space otherwise.
- [ ] 4.3 Update `refreshViewport` to call `injectGutter` instead
  of `injectBorder`.
- [ ] 4.4 Update `visualModeStyle`: change `Background` from
  `lipgloss.Color("212")` to `lipgloss.Color("51")` (cyan).
- [ ] 4.5 Add `internal/render/gutter_test.go` covering four
  cases: focused block (cyan), commented block (yellow),
  gap line (space), and focused + commented block (cyan wins).

## 5. Cleanup

- [ ] 5.1 Delete `internal/render/border_test.go` (whole file;
  tests the removed `injectBorder` / `hasCommentGutter` /
  `trimLeadingVisible` helpers).
- [ ] 5.2 Remove any other dangling references to
  `render.borderStyle` (the only known reference is the test
  fixture in `border_test.go`, which is deleted in 5.1).
- [ ] 5.3 Run `task test` (or equivalent) and confirm all
  packages build and tests pass.

## 6. Visual smoke check

- [ ] 6.1 Manually render a fixture markdown document with three
  blocks (one commented) under `pinky`, navigate with `j`/`k`,
  and confirm: cyan `▍` follows focus, yellow `▍` spans the
  commented block, headings render in cyan without `#` prefixes,
  no heavy borders remain. Capture as a one-line demo / assertion
  in `render_test.go` if it adds value; otherwise leave as a
  manual-only check.