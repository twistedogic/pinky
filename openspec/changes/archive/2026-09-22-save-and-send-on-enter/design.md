## Context

The comment composer currently saves on `Enter` (after the
single-key-keymap change) and returns to nav. The user must then press
`s` in nav to actually dispatch the batch. Two keystrokes per comment is
friction the flow doesn't need — typing + Enter already implies
"commit this"; sending is the natural follow-on, not a separate
intention. Submitting via the same `tmux set-buffer` + `paste-buffer` +
`send-keys Enter` path the compose-mode redirect uses preserves the
single-shot guarantee (all comments go to the agent in one paste, not
one-per-comment).

This change makes `Enter` in the comment composer the single keystroke
that does both: save the new comment and dispatch the whole batch.

## Goals / Non-Goals

**Goals:**

- `Enter` in the comment composer saves the new comment and immediately dispatches every accumulated comment via `submitAllComments` (which uses the existing inject pipeline).
- A failed send keeps `m.comments` so the user can retry with `s` in nav.
- Empty composer + `Enter` is still a no-op (cancel path, no send).
- Help overlay, placeholder, and README all reflect the new behavior.

**Non-Goals:**

- Changing the inject pipeline. `submitAllComments` already does the
  right thing — append to history, send via `sendToPane`, clear on
  success, keep on failure.
- Removing `s` in nav. It remains the retry path.
- Adding multi-comment batch send via a new shortcut.
- Reordering comment types or changing the appendix format.

## Decisions

### Chain `saveComment` + `submitAllComments` in `handleCommentComposerKey`

Both functions are pointer-receiver methods on `*model`. Calling them in sequence in the `case tea.KeyEnter:` branch keeps the change one line. `submitAllComments` is a no-op when `len(m.comments) == 0`, so the cancel paths inside `saveComment` (empty text, invalid block index) naturally short-circuit without a send.

Alternatives considered:
- New combined `saveAndSendComment` function — adds a layer of indirection for one call site.
- Send first, save second — wrong order: the new comment must be in `m.comments` before `submitAllComments` formats the appendix.

### Keep `s` in nav as a retry path

After a failed send, `submitAllComments` keeps `m.comments` populated and surfaces the error. `s` in nav now calls `submitAllComments` and is the user's retry mechanism. The spec for `One-shot submit all comments via s` stays as-is — it now describes a recovery flow rather than the primary send path.

### Help verb is `save & send all`

Two-word verb, fits the 80-column help overlay, still contains the `save` substring the existing `help_test.go` substring assertion checks. Alternative verbs considered: `submit all` (loses "save"), `commit & send` (also loses "save"), `save and dispatch` (too long).

## Risks / Trade-offs

- **[Risk]** User accidentally types Enter on an empty composer and the previous (unrelated) comment batch is sent. → **Mitigation:** empty composer + Enter is already a cancel path; `submitAllComments` only fires if there is at least one comment.
- **[Risk]** Race between `saveComment` and `submitAllComments` if any future caller observes `m.state == stateNav` between them. → **Mitigation:** both are synchronous, same goroutine, no observable mid-state.
- **[Trade-off]** Loss of the "save many, then send once" workflow where a user might `c` + type + Esc + `c` + type + Esc + `s`. That workflow is now collapsed into the default Enter-saves-and-sends flow. If a user wants to accumulate without sending, they have to use Esc (which discards the composer text). Acceptable — the user asked for the single-keystroke flow.

## Open Questions

None blocking. Two follow-up items the design surfaces but does not
resolve:

- Could the placeholder hint at this flow? Currently it says
  `"comment — Esc cancel"`. After this change the placeholder is
  accurate (Esc is the only alternative).
- If a future feature wants to send comments without saving a new one
  (e.g., auto-send on viewport idle), `submitAllComments` is the
  primitive to reach for; no spec change needed.