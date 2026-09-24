## Why

Two scroll bugs make long content unreadable. (1) The latest-message viewport
is force-`GotoBottom()`-ed on every 500ms poll, so the moment the user scrolls
up to re-read older content the next tick yanks them back. (2) The file
viewer in `stateFileView` has no scrolling at all — it renders every line of
the file as one string and the terminal clips past the viewport height, so
the user sees only the top ~20 lines of any file longer than that and `j`/`k`
move an invisible cursor.

## What Changes

- **Latest-message view**: replace unconditional `GotoBottom()` on every poll
  with sticky-bottom — only `GotoBottom()` when the user was already at the
  bottom before the refresh. Scrolling up "freezes" the viewport at that
  position; the next poll re-attaches when the user scrolls back to the
  bottom (or presses `End`). A new message (different `Text` from the prior
  `m.latest`) still force-scrolls to bottom so the new content is visible.
- **File viewer**: replace the all-lines-in-one-string render with a
  `bubbles/viewport.Model` owned by `fileViewer`. The existing `j`/`k` cursor
  movement continues to extend the visual selection and anchor comments, and
  also drives the viewport (scroll-on-cursor-off-screen, same 1-line cushion
  as the message view). Arrow keys and PageUp/PageDown are forwarded to the
  viewport for fine-grained scrolling. The header line gains a `lines N-M of
  K` indicator so the user can see position.

No new capabilities are introduced — both are requirement changes against
existing specs.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `latest-message-view`: the "Yank to bottom on new content" requirement is
  replaced by a sticky-bottom requirement (don't yank while the user is
  scrolled up; re-attach when they return to the bottom; force-attach on a
  genuinely new message).
- `workspace-files`: a new requirement is added so the file viewer scrolls
  its content via a `bubbles/viewport.Model` and shows a position indicator
  in the header.

## Impact

- `model.go`: ~5 lines change in the `sessionMsg` poll handler (sticky-bottom
  guard); ~30-50 lines restructuring in `fileViewer` + `handleFileViewKey` +
  `fileViewView` to use a viewport (new field, new render path, key
  forwarding).
- `internal/render/`: unchanged.
- `model_test.go` + `internal/render/*_test.go`: new test cases for
  sticky-bottom re-attach and file-viewport scroll behaviour.
- No new dependencies. The `bubbles/viewport` package is already a direct
  dependency.
- No wire-format / file-format / API changes.
