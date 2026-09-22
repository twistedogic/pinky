## 1. Rewire keymap bindings

- [x] 1.1 In `keymap.go`: change `SaveComment` binding from `ctrl+s` to `enter`, change help label from `"^S", "save"` to `"⏎", "save comment"`. Update the field comment block above it (currently justifies `s` as literal; drop that line since `Enter` no longer has a multi-line conflict).
- [x] 1.2 In `keymap.go`: change `IncludeComments` binding from `ctrl+i` to `i`, change help label from `"^I", "include comments"` to `"i", "include comments"`.
- [x] 1.3 In `keymap.go`: update the `NavGroup` placeholder keys list (the `WithHelp` second arg includes all nav runes); check whether `i` should be added there. (`i` is compose-only, not nav — no change; NavGroup stays `j k h l v Esc c s q r n`.)

## 2. Comment composer behaviour

- [x] 2.1 In `model.go`: in `handleCommentComposerKey`, replace `case tea.KeyCtrlS:` with a handler for `Enter` (either `tea.KeyEnter` or, since the textarea model handles rune input, intercept `KeyRunes` for the single `\r` or `KeyEnter` before the textarea update — pick the one that is consistent with how the rest of the file handles Enter; likely `tea.KeyEnter`).
- [x] 2.2 In `model.go`: in `initCommentComposer`, change `ta.SetHeight(composeHeight)` to `ta.SetHeight(1)` (single-line). Update placeholder from `"comment — Enter newline, Ctrl+S save, Esc cancel"` to `"comment — Esc cancel"`.
- [x] 2.3 In `model.go`: update the doc comment on `handleCommentComposerKey` ("Ctrl+S saves the comment, Esc cancels" → "Enter saves the comment, Esc cancels").

## 3. Tests

- [x] 3.1 In `model_test.go`: update `initCommentTAForTest` placeholder string and `SetHeight(composeHeight)` to match the new single-line composer (SetHeight(1)).
- [x] 3.2 In `model_test.go`: add a regression test `TestCommentComposer_EnterSaves` that seeds the composer with text, presses `Enter`, and asserts the comment is stored and state returns to `stateNav`.
- [x] 3.3 In `model_test.go`: add a regression test `TestCommentComposer_EnterDoesNotInsertNewline` that seeds the composer, types `Enter`, and asserts the saved comment text contains no `\n` characters.
- [x] 3.4 In `model_test.go`: add a regression test `TestCompose_I_TogglesIncludeComments` that toggles includeComments via `i` (mirrors the new spec scenario).
- [x] 3.5 Run `go test ./...` and confirm `help_test.go` substring assertions still pass (`"save"` is contained in `"save comment"`).

## 4. Spec archive

- [ ] 4.1 Run `openspec validate single-key-keymap` and resolve any flagged issues.
- [ ] 4.2 Run `openspec archive single-key-keymap` to fold the spec delta into `openspec/specs/message-comments/spec.md`.

## 5. README

- [x] 5.1 Top of `README.md`: in both ASCII diagrams, change `Ctrl+N` → `n` and `Ctrl+S to send` → `s to send`.
- [x] 5.2 `Keys` section: rewrite the top table to be the nav-mode table (entries for `j`/`k`, `h`/`l`, `}` / `{` removal, `]]`/`[[` removal, `gg`/`G`/`PgUp`/`PgDn` removal, `v`, `c`, `s`, `n`, `r`, `q`, `?`, `Ctrl+C`).
- [x] 5.3 `Keys` section: add a compose-mode table (`Enter` newline, `i` include comments, `s` send, `Esc` cancel, `?` help).
- [x] 5.4 `Keys` section: add a comment-composer table (`Enter` save comment, `Esc` cancel).
- [x] 5.5 `Keys` section: extend the picker table to include `↑`/`↓`, `Enter` (select), `q` (quit), `Ctrl+C`.
- [x] 5.6 `Keys` section: replace the line "currently-focused block is marked by horizontal border lines above and below it" with the spec wording (cyan left gutter `▍` spanning the block's `StartLine..EndLine`).