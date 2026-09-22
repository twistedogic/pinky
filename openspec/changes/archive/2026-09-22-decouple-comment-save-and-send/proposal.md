# Decouple comment save from send, and fix leading-separator bug

## Why

The comment composer today couples two actions on `Enter`: save the
comment to memory **and** immediately flush every accumulated comment
to the agent pane via tmux. The recent `save-and-send-on-enter`
change made every `c → type → ⏎` cycle end with a network round-trip
to tmux, which made multi-comment workflows noisy and surprised users
who expected `c` to mean "stage a comment, not fire one".

We want the composer to be a **staging buffer**, not a firing
mechanism. `Enter` should commit the current draft to memory and
return to nav; the user decides when to flush the whole batch with a
single explicit `s`. This also fixes a real bug in the same area: a
flush via `s` currently sends a payload whose very first bytes are
`\n\n---\n` (the appendix's hard-coded separator with nothing before
it), which the agent receives as a naked `---` line.

## What Changes

- **`Enter` in the comment composer** saves the new comment to
  `m.comments`, returns to `stateNav`, and **does not** call
  `submitAllComments`. Multi-comment accumulation across multiple
  `c → type → ⏎` cycles is now the normal flow.
- **`s` in `stateNav`** becomes the **primary** flush mechanism,
  not the retry fallback. It dispatches every comment in
  `m.comments` via `submitAllComments` → tmux in one inject.
- **`render.FormatCommentsAppendix`** no longer hard-codes the
  leading `\n\n---\n` separator. The caller owns the prefix and
  adds it when (and only when) there is preceding text to separate
  from.
- **Keymap help** for `SaveComment` changes from `save & send all`
  → `save` (the substring `save` is preserved so the help test
  still passes). The comment-composer placeholder drops its
  send-implication.

## Capabilities

### Modified Capabilities

- `message-comments`: the `Block-level annotation via c`
  requirement's two `Enter sends…` scenarios are replaced with a
  single `Enter saves and exits without sending` scenario; the
  `One-shot submit all comments via s` requirement stops describing
  itself as the retry path and becomes the primary send path; a
  new scenario covers empty-`Enter` (no-op). The `Single-line
  comment composer` requirement's `Enter saves…` scenario loses
  its "and sends all comments" clause.

## Impact

- `model.go`: in `handleCommentComposerKey`, the `tea.KeyEnter`
  branch drops the `m.submitAllComments()` call. `saveComment`
  already returns to `stateNav`, so no state-machine wiring
  changes. `handleSend → stateCompose` prepends `"\n\n---\n"`
  itself when `text != "" && m.includeComments && len(m.comments)
  > 0`. The doc comment on `handleCommentComposerKey` is
  rewritten. `submitAllComments` keeps its `hist.Append` +
  `sendToPane` flow.
- `keymap.go`: `SaveComment` help text `save & send all` →
  `save`.
- `internal/render/comment_appendix.go`: `FormatCommentsAppendix`
  body drops `b.WriteString("\n\n---\n")`. Doc comment updated.
- `README.md`: comment-composer table row (`Enter` →
  `Save comment and return to nav`); nav table `s` row drops the
  "retry path" parenthetical and becomes the primary send key;
  comment-composer placeholder paragraph trimmed.
- `model_test.go`: `TestCommentComposer_EnterSavesAndSends` is
  reframed as `TestCommentComposer_EnterSavesAndExits` (asserts
  comment stored, `sendToPane` **not** called, state returns to
  `stateNav`); `TestCommentComposer_EnterSendsMultipleInOneCall`
  is replaced by `TestCommentComposer_MultipleCommentsAccumulate`
  (two `c → ⏎` cycles yield two comments in `m.comments` with no
  inject); `TestCommentComposer_EnterDoesNotInsertNewline`
  stays; `TestNavSend_FlushesAccumulatedComments` is added to
  cover the new primary send path.
- `internal/render/comment_appendix_test.go`: existing tests
  against the *body* keep passing (the `---` line was not in the
  expected output for empty-comments tests, but a `HasLeadingSeparator`
  test is added to lock the new invariant that
  `FormatCommentsAppendix` output starts with the count line, not
  with `---`).
- `openspec/specs/message-comments/spec.md`: delta updates per
  above.