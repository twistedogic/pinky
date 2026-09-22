## Why

The comment composer saves with `Ctrl+S`, but a multi-line composer where
the only way to commit is a chord is awkward — every comment ends with
the user lifting one finger off home row to reach `Ctrl`. Same problem
for `Ctrl+I` (include-comments toggle) in a single-letter keymap. The
README also documents keys that don't exist and omits ones that do, so
contributors and users can't read it as a source of truth.

The principle: prefer single physical keys for any action that's part of
a normal flow. Keep `Ctrl+C` (Unix quit) as the only deliberate exception.

## What Changes

- **Comment save** (`Ctrl+S` → `Enter`). Composer becomes single-line; `Enter` saves, `Esc` cancels, no newline affordance.
- **Include-comments toggle** (`Ctrl+I` → `i`). Single-key, no chord.
- **Help overlay verb** (`save` → `save comment`) so the action reads unambiguously.
- **Composer placeholder** drops the keymap summary (the help overlay already covers it; the placeholder just notes the remaining keys).
- **README keys section** is rewritten to match the live keymap exactly:
  - delete phantom rows: `}`, `{`, `]]`, `[[`, `gg`, `G`, `PgUp`, `PgDn`
  - rewrite the `c` row (was "compose alias"; actually opens comment composer)
  - add missing nav rows: `v`, `s`, `n`, `r`
  - add compose-mode table (was absent)
  - add comment-composer table (was absent)
  - add missing picker rows: `↑/↓`, `Enter`, `q`, `Ctrl+C`
  - fix the focus-indicator sentence (`horizontal border lines` → cyan left gutter `▍`)
  - fix both ASCII diagrams (`Ctrl+N` → `n`, `Ctrl+S` → `s`)

No breaking API changes. The redirect compose `s` send, nav `s` batch-send, and `Ctrl+C` quit are unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `message-comments`: save key `Ctrl+S` → `Enter` (composer becomes single-line); include-comments toggle `Ctrl+I` → `i`; help verb `save` → `save comment`.

## Impact

- `keymap.go` — rewire `SaveComment` binding to `Enter`; rewire `IncludeComments` to `i`; update help strings.
- `model.go` — `handleCommentComposerKey` switches on `Enter` instead of `KeyCtrlS`; placeholder text updates; single-line composer height.
- `model_test.go` — placeholder string fixture updates (substring matchers still pass).
- `help_test.go` — substring `save` still matches `save comment`; no change.
- `openspec/specs/message-comments/spec.md` — three `Ctrl+S` mentions and the `Ctrl+I` toggle requirement update.
- `README.md` — full Keys section rewrite plus two ASCII diagrams at the top.