## 1. Sticky-bottom auto-follow (latest-message view)

- [x] 1.1 In `model.go`, capture `wasAtBottom := m.viewport.AtBottom()` before
      `m.refreshViewport()` in the `sessionMsg` handler, and only call
      `m.viewport.GotoBottom()` afterwards when `wasAtBottom` is true.
- [x] 1.2 Confirm the "new message replaces" path still force-attaches: when
      `last.Text != m.latest.Text`, the GotoBottom should fire regardless
      (treat as `wasAtBottom = true` for that branch, or move the
      `GotoBottom()` call inside the `last.Text != m.latest.Text` branch).
- [x] 1.3 Add a regression test in `model_test.go` that polls after
      `m.viewport.SetYOffset(...)` to a non-bottom position and asserts the
      YOffset is unchanged after the poll handler runs.
- [x] 1.4 Add a regression test that polls, scrolls up, polls again, and
      asserts the YOffset stays put; then presses `End` and polls again
      and asserts YOffset is at the bottom.
- [x] 1.5 Add a regression test that a brand-new `Text` (different from
      prior `m.latest.Text`) lands at the bottom regardless of prior
      YOffset.

## 2. File viewer scrolling

- [x] 2.1 Add a `viewport viewport.Model` field to the `fileViewer` struct
      in `model.go`. Import the bubbles `viewport` package (already a
      direct dependency).
- [x] 2.2 Extract the body of `fileViewView()` (header + per-line loop +
      selection highlight + gutter) into a string-returning helper, e.g.
      `renderFileContent() string`, that builds the full rendered file
      into a single string. Return that string from the helper.
- [x] 2.3 Add `refreshFileView()` that calls `renderFileContent()` and
      hands the result to `m.fileViewer.viewport.SetContent(...)`.
- [x] 2.4 In `openFile` (around `model.go:945`), seed
      `m.fileViewer.viewport = viewport.New(w, h)` using the current
      `viewportSize()` and call `refreshFileView()` so the viewport has
      content immediately.
- [x] 2.5 In `reflow()`, also assign `m.fileViewer.viewport.Width = w` and
      `.Height = vpHeight` so the file viewport follows terminal size
      changes (call only when `m.fileViewer.path != ""` to avoid
      touching the zero-value viewport).
- [x] 2.6 Add `scrollFileCursorIntoView()` that mirrors
      `scrollCursorIntoView`: if `m.fileViewer.cursor-1` (0-indexed) is
      outside `[YOffset, YOffset+Height)`, call `SetYOffset` so the cursor
      lands on the last visible line.
- [x] 2.7 In `fileViewMoveLine`, after updating `m.fileViewer.cursor`, call
      `m.scrollFileCursorIntoView()` and then `m.refreshFileView()` so the
      gutter + selection are repainted against the new YOffset.
- [x] 2.8 In `handleFileViewKey`, after the existing rune handling, forward
      unhandled special keys (`tea.KeyUp`, `KeyDown`, `KeyPgUp`,
      `KeyPgDown`, `KeyHome`, `KeyEnd`) to
      `m.fileViewer.viewport.Update(msg)` and return the viewport cmd.
      `j`/`k` continue to route through `fileViewMoveLine`; `h`/`l`/`v`/
      `c`/`s`/`Esc`/`Tab`/`q`/`?` are unchanged.
- [x] 2.9 Replace the body of `fileViewView()` so it returns
      `m.fileViewer.viewport.View()` instead of building a full-file
      string. Add the `lines <topVisible+1>-<topVisible+Height> of <total>`
      indicator to the rendered string before it goes into the viewport
      when `len(m.fileViewer.lines) > m.fileViewer.viewport.Height`.

## 3. Validation

- [x] 3.1 Run `openspec validate scroll-fixes` and resolve any errors.
- [x] 3.2 Run `go test ./...` and confirm all existing tests still pass.
- [ ] 3.3 Manual smoke: attach to a session with a long message, scroll up,
      wait > 500 ms, confirm the viewport does not jump back to the
      bottom; press `End`, wait > 500 ms, confirm new appended text
      re-anchors the bottom.
- [ ] 3.4 Manual smoke: open a long file (> 100 lines) from the file tab,
      press `j` repeatedly, confirm the viewport scrolls and a
      `lines N-M of K` indicator appears in the header; press `PageDown`,
      confirm the viewport scrolls a page; press `End`, confirm it lands
      at the bottom.
- [ ] 3.5 Manual smoke: open a short file (≤ viewport height), confirm no
      indicator is shown in the header.
