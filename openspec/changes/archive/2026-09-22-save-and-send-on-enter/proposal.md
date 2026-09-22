## Why

The comment composer saves the comment and returns to nav, but the
user still has to press `s` afterwards to actually dispatch the batch
to the agent. Two keystrokes per comment is more friction than the
flow needs — they typed the comment and pressed Enter, the natural
expectation is that it goes out. Make `Enter` in the comment composer
save the new comment AND send every accumulated comment as one tmux
inject, so the per-comment cost is one keystroke.

## What Changes

- **`Enter` in the comment composer** now saves the new comment and immediately calls `submitAllComments()`. The entire batch (the new comment plus any prior comments accumulated since the last send) is dispatched as one tmux paste-buffer + send-keys via the existing inject pipeline.
- **`s` in nav** stays as the retry key for the case where a previous send failed and `m.comments` was kept.
- **Help verb** for the comment composer key changes from `save comment` → `save & send all` (matches the new behavior; still contains the `save` substring the help test requires).
- **Placeholder** updates to reflect the new single-key flow.

## Capabilities

### Modified Capabilities

- `message-comments`: the `Block-level annotation via c` requirement's `Save creates a block-level comment` scenario is replaced with `Save sends all comments in one batch`; an additional scenario covers the multi-comment case; the comment-composer placeholder paragraph is updated to reflect the save-and-send flow.

## Impact

- `model.go`: `handleCommentComposerKey` calls `saveComment` then `submitAllComments` on `tea.KeyEnter`. `initCommentComposer` placeholder text updated.
- `keymap.go`: `SaveComment` help label updated to `"save & send all"`.
- `model_test.go`: the existing `TestCommentComposer_EnterSaves` is reframed as `TestCommentComposer_EnterSavesAndSends` (asserts comment stored AND sent in one `sendToPane` call with the formatted appendix); `TestCommentComposer_EnterDoesNotInsertNewline` still applies; a new test covers the multi-comment single-batch case.
- `README.md`: comment-composer table updated; "save comment" → "save & send all".
- `openspec/specs/message-comments/spec.md`: delta updates as above.