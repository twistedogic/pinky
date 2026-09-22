## Context

Pinky uses single-rune keys for the nav state machine (`j k h l v c s q r n`), but two actions still require chords: comment save (`Ctrl+S`) and include-comments toggle (`Ctrl+I`). For comment save specifically the composer is also multi-line, so `Enter` is already spoken for as a newline. The chord + multi-line combo means every comment ends with the user reaching for `Ctrl`. The README documents keys that don't exist (`}`, `{`, `]]`, `[[`, `gg`, `G`, `PgUp`, `PgDn`), omits nav keys that do (`v`, `s`, `n`, `r`), and shows the focus indicator as horizontal borders when the spec already mandates a cyan gutter.

This change applies a single principle consistently: prefer single physical keys for any action that is part of a normal flow. Keep `Ctrl+C` (Unix interrupt) as the only deliberate exception.

## Goals / Non-Goals

**Goals:**

- Comment composer saves with `Enter`, not `Ctrl+S`.
- Include-comments toggle uses `i`, not `Ctrl+I`.
- Help overlay verb for the comment composer reads `save comment`, not `save`.
- README Keys section is rewritten to match the live keymap exactly, including a compose-mode table and a comment-composer table that were absent.
- ASCII diagrams at the top of the README reflect the actual keys.

**Non-Goals:**

- Changing `Ctrl+C` (quit), `Ctrl+R` (re-poll — note: code uses `r`, README was wrong), or any nav rune.
- Adding multi-line back into the comment composer. If a long comment is needed, paste it — the textarea still accepts newlines on paste.
- Restoring the `}`, `{`, `]]`, `[[`, `gg`, `G`, `PgUp`, `PgDn` keys. They are phantom rows in the README; nothing in code or specs references them.

## Decisions

### Comment composer is single-line

`Enter` cannot mean both "insert newline" and "save" inside one textarea. Choosing single-line makes `Enter = save` literal and consistent with the single-key principle. Multi-line affordance was speculative — comments in practice are short annotations ("rename to foo", "what does this do"). The textarea model still accepts pasted text with embedded newlines; the user just commits with `Enter`.

Alternatives considered: Ctrl+Enter save, Meta+Enter save, double-Enter save, Shift+Enter save — all add a chord or a pattern that defeats the principle. Single-line is the only shape where saving is a true single-key action.

### Include-comments toggle moves to `i`

`i` is unused across every nav binding (`j k h l v c s q r n ?` plus arrows) and reads clean (`i` = include). The full keymap lives in one spec (`single-cursor-nav`), so any future collision is detectable by re-reading that spec. The mnemonic also matches the existing `[I] include N comments — ON/OFF` status line.

Alternatives considered: `t` (toggle — generic, less mnemonic), `Tab` (breaks tab-typing in nav), keep `Ctrl+I` (violates the principle).

### Help verb is `save comment`, not `save`

`save` alone is ambiguous — save what, to where? After the rebind, the key column would be `⏎ save`, which a user could easily misread as "Enter inserts and saves" or "Enter is the newline key, save is the next column". `save comment` removes both readings. The substring `save` is still matched by the existing `help_test.go` substring assertion, so no test change is needed.

### Placeholder drops the keymap summary

The composer placeholder currently duplicates the keys (`"comment — Enter newline, Ctrl+S save, Esc cancel"`). Once the help overlay covers them, the placeholder's only job is to set the mode — drop the keymap summary and let the placeholder read `"comment — Esc cancel"` (single-line; Enter saves is implied by the lack of any newline affordance). Avoids drift between placeholder and help overlay.

### Focus indicator follows the spec

Spec already says cyan left gutter `▍`. README says horizontal border lines. Pick the spec.

## Risks / Trade-offs

- **[Risk]** Users with muscle memory for `Ctrl+S` on save lose their shortcut. → **Mitigation:** This is a deliberate principle change; the spec is the source of truth going forward.
- **[Risk]** Single-line composer means comments can't be paragraphed from inside pinky. → **Mitigation:** Pasting still works; long-form feedback belongs in the redirect composer, not the comment composer.
- **[Risk]** `i` collision if a future feature wants to use `i`. → **Mitigation:** Single source of truth is `single-cursor-nav` spec; re-read it before adding nav bindings.
- **[Risk]** README drift returns as features land. → **Mitigation:** The `single-cursor-nav` spec already pins the nav surface. This change adds equivalent pinning for the compose and comment-composer surfaces (in `message-comments`). README can be regenerated from the specs without re-auditing.
- **[Trade-off]** `Ctrl+C` is the only remaining chord by design. Worth it for Unix interrupt muscle memory.