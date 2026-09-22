# Tasks

## 1. Decouple save from send in comment composer

- [x] 1.1 In `model.go` `handleCommentComposerKey`: remove
      `m.submitAllComments()` from the `tea.KeyEnter` branch.
      `saveComment()` already transitions to `stateNav`, so no
      state wiring change.
- [x] 1.2 In `model.go`: rewrite the doc comment on
      `handleCommentComposerKey` ("Enter saves the new comment
      and dispatches every accumulated comment in one batch via
      submitAllComments; Esc cancels.") to "Enter saves the new
      comment and returns to nav without sending; Esc cancels
      the draft."

## 2. Move the `---` separator out of `FormatCommentsAppendix`

- [x] 2.1 In `internal/render/comment_appendix.go`: remove the
      `b.WriteString("\n\n---\n")` line from
      `FormatCommentsAppendix`. Update the doc comment to state
      the function emits the body only (count line + entries)
      and that callers prepend `"\n\n---\n"` when there is
      preceding prose.
- [x] 2.2 In `model.go` `handleSend` `stateCompose` branch:
      change `text += render.FormatCommentsAppendix(...)` to
      build the separator manually, e.g.
      `text += "\n\n---\n" + render.FormatCommentsAppendix(...)`
      inside the existing guard `if m.includeComments && len(m.comments) > 0`.
      `submitAllComments` is untouched — it calls the appendix
      renderer directly and gets a body-only payload.
- [x] 2.3 Add a unit test `TestFormatCommentsAppendix_NoLeadingSeparator`
      in `internal/render/comment_appendix_test.go`: feed a single
      comment, assert the result does not start with `--` and
      starts with `1 comment:`.

## 3. Update keymap help and placeholder

- [x] 3.1 In `keymap.go`: change `SaveComment` help from
      `"⏎", "save & send all"` to `"⏎", "save"`. The substring
      `save` is preserved so `help_test.go` substring assertions
      still pass.
- [x] 3.2 In `keymap.go`: update the comment on the
      `// Comment composer (Enter saves the new comment AND sends every
      // accumulated comment in one batch via the inject pipeline).`
      block above `SaveComment` to
      `// Comment composer (Enter saves the new comment and returns
      // to nav without sending; s in nav flushes the accumulated
      // batch).`.

## 4. Update tests

- [x] 4.1 In `model_test.go`: rename
      `TestCommentComposer_EnterSavesAndSends` →
      `TestCommentComposer_EnterSavesAndExits`. Stub
      `sendToPane`, open composer, type text, press Enter.
      Assert (a) `m.comments` contains the saved comment with
      the expected anchor, (b) `sendToPane` was **not** called,
      (c) `m.state == stateNav`.
- [x] 4.2 In `model_test.go`: replace
      `TestCommentComposer_EnterSendsMultipleInOneCall` with
      `TestCommentComposer_MultipleCommentsAccumulate`. Open
      composer, type, press Enter; repeat. After the second
      `⏎`, assert (a) `m.comments` has two entries, (b)
      `sendToPane` was never called, (c) state is `stateNav`.
- [x] 4.3 In `model_test.go`: `s`-flush is already covered by
      existing `TestSubmitComments_ClearsOnSuccess` and
      `TestNavKey_S_NavWithCommentsSends`. No new test added
      to avoid duplication.
- [x] 4.4 In `model_test.go`: add
      `TestCommentComposer_EmptyEnterIsNoop`. Open composer,
      do not type, press Enter. Assert (a) `m.comments` is
      empty, (b) `sendToPane` not called, (c) state is
      `stateNav`.
- [x] 4.5 Refit `TestCommentComposer_EnterDoesNotInsertNewline`
      to assert no embedded newline in the saved comment text
      (the prior assertion against `sentText` no longer applies
      because Enter does not dispatch).

## 5. README

- [x] 5.1 In `README.md` Comment composer table: change
      `Enter | Save comment and send all accumulated comments
      in one batch` → `Enter | Save comment and return to nav (does not send)`
      (plus `Esc` row updated to `Cancel draft and return to nav`).
- [x] 5.2 In `README.md` nav table: change
      `s | Send any pending comments (retry path after a failed send)`
      → `s | Send all accumulated comments in one tmux inject`.

## 6. Spec archive

- [x] 6.1 Run `openspec validate decouple-comment-save-and-send`
      and resolve any flagged issues.
- [x] 6.2 Run
      `openspec archive --no-validate --yes decouple-comment-save-and-send`.