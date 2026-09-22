## 1. Implementation

- [x] 1.1 In `model.go`: in `handleCommentComposerKey`, after `m.saveComment()` add `m.submitAllComments()` on the `tea.KeyEnter` branch.
- [x] 1.2 In `model.go`: update the doc comment on `handleCommentComposerKey` ("Enter saves the comment, Esc cancels" → "Enter saves and sends all comments in one batch, Esc cancels").
- [x] 1.3 In `keymap.go`: confirm `SaveComment` help is `"⏎", "save & send all"` (already in place from the prior revert; verify).

## 2. Tests

- [x] 2.1 In `model_test.go`: rename `TestCommentComposer_EnterSaves` → `TestCommentComposer_EnterSavesAndSends`. Stub `sendToPane` to capture the call, assert (a) `m.comments` had the saved comment, (b) `sendToPane` was called once, (c) the captured text equals `render.FormatCommentsAppendix(comments, blocks)` and contains the comment text, (d) `m.comments` is cleared after the call.
- [x] 2.2 In `model_test.go`: keep `TestCommentComposer_EnterDoesNotInsertNewline` (still valid — Enter is still atomic, no newline insertion).
- [x] 2.3 In `model_test.go`: add `TestCommentComposer_EnterSendsMultipleInOneCall`. Pre-seed `m.comments` with one comment, open composer, type a new one, press Enter. Assert exactly one `sendToPane` call (not two), with the formatted appendix containing both comments.
- [x] 2.4 Run `go test ./...` — confirm help_test substring assertions still pass (`"save"` ⊂ `"save & send all"`).

## 3. Spec archive

- [x] 3.1 Run `openspec validate save-and-send-on-enter` and resolve any flagged issues.
- [x] 3.2 Run `openspec archive --no-validate --yes save-and-send-on-enter` (same pre-existing lint quirk that flagged requirements starting with `When`/`In` applies; the merged spec is correct).

## 4. README

- [x] 4.1 In `README.md` comment-composer table: change `Enter` | Save comment → `Enter` | Save comment and send all accumulated comments in one batch.
- [x] 4.2 In `README.md` nav table: update `s` row to note it's now the retry path after a failed send (primary path is Enter in comment composer).