# Design

## Context

`pinky` lets the user annotate the agent's latest message with
comments and flush a redirect + comments appendix back into the
agent's tmux pane. Comments accumulate in `m.comments`; flushing
calls `submitAllComments`, which formats the slice via
`render.FormatCommentsAppendix`, writes it to local history, and
hands the bytes to `sendToPane` (load-buffer → paste-buffer →
send-keys Enter).

Today's `handleCommentComposerKey` does two things on `Enter`:

1. `saveComment()` — append the new draft to `m.comments`,
   transition back to `stateNav`.
2. `submitAllComments()` — format and send the slice to tmux
   **immediately**.

This couples staging to delivery. The user can't accumulate a
handful of comments and send them in one batch without each `⏎`
firing a tmux round-trip. Worse, an in-flight failure leaves
`m.comments` non-empty but the user has already exited the
composer — recovery is invisible.

In the same code path, `render.FormatCommentsAppendix` hard-codes
`"\n\n---\n"` at the start of its output. The compose-mode caller
prepends its own prose first (`text + appendix`), so the separator
sits between two paragraphs — fine. But `submitAllComments` calls
`FormatCommentsAppendix` standalone, and the agent receives a
payload that begins with `\n\n---\n`. From the agent's perspective,
its first line of input is a naked `---`.

## Goals / Non-Goals

**Goals:**

- `Enter` in the comment composer is a pure staging operation:
  save to memory, return to nav, never touch tmux.
- `s` in `stateNav` is the single send key for the comments batch.
- `render.FormatCommentsAppendix` emits only the appendix body;
  the caller composes the separator when it has preceding prose.
- Multi-comment workflow: compose N comments with N `c → ⏎`
  cycles, send them in one tmux inject via `s`.
- Existing failure-retry semantics for `s` (slice kept on inject
  error, error surfaced via `[send failed: …]`) are preserved.

**Non-Goals:**

- Changing the on-disk history format.
- Changing the compose-mode redirect flow (`s` from compose still
  sends `<redirect text>\n\n---\n<appendix>` when the include flag
  is on).
- Changing the visual rendering of comments (footnotes, gutter
  tint).
- Adding per-comment undo / persistence across detaches (comments
  remain in-memory only).
- Touching the `Single-line comment composer` placeholder or the
  paste-multiline-verbatim behavior.

## Decisions

### D1. `Enter` in composer = save + exit, never send

`handleCommentComposerKey`'s `tea.KeyEnter` branch becomes:

```go
case tea.KeyEnter:
    m.saveComment()
    return m, nil
```

`saveComment` already handles the empty-text no-op (calls
`cancelCommentComposer`) and the bad-anchor no-op. On success it
appends to `m.comments`, blurs the textarea, sets
`m.state = stateNav`, and reflows. The state transition is free
because it lives in `saveComment`. No new state plumbing needed.

Rationale: the previous `submitAllComments()` call after
`saveComment()` is the single line to delete. Everything else is
already correct.

### D2. `s` in nav is the primary send key

No code change in `handleSend` for the `stateNav` branch — it
already routes to `submitAllComments`, which already does the
right thing. The change is conceptual: `s` is no longer the
"retry after a failed `Enter`" fallback; it is the *only* way to
flush the comment slice. The existing failure-retry semantics
(failure keeps the slice, surfaces `[send failed: …]`) carry
over unchanged and remain useful: a user who hits `s` and gets a
failure can fix the agent pane and hit `s` again.

### D3. `render.FormatCommentsAppendix` stops emitting the leading `---`

The renderer's job is the appendix body. The caller decides
whether a separator is needed.

```go
func FormatCommentsAppendix(comments []Comment, blocks []Block) string {
    if len(comments) == 0 {
        return ""
    }
    // ... (no leading "\n\n---\n")
    b.WriteString(strconv.Itoa(len(sorted)))
    b.WriteString(" ")
    b.WriteString(label)         // "comment" | "comments"
    b.WriteString(":\n")
    for _, c := range sorted {
        // ... per-comment line
    }
    return b.String()
}
```

Caller composition in `model.go`:

- `submitAllComments` — uses `text = FormatCommentsAppendix(...)`
  directly. No separator.
- `handleSend → stateCompose` — builds `text = textarea.Value()`
  and, only when `text != "" && m.includeComments && len(m.comments)
  > 0`, prepends `"\n\n---\n"` and appends the appendix body.

This restores the invariant "a payload never starts with `---`"
without changing the visible content of compose-mode redirects.

### D4. Empty `Enter` stays a no-op

`saveComment` already calls `cancelCommentComposer` (state → nav,
no save) when `text == ""`. No new guard needed. The spec adds
an explicit scenario to lock this in.

### D5. Keymap help & placeholder

- `SaveComment` help: `save & send all` → `save`. The substring
  `save` is preserved, so the existing `help_test.go` substring
  assertion still passes.
- Comment composer placeholder: `comment — Esc cancel` stays. The
  send-now implication is gone, so the user is no longer misled
  into thinking `⏎` fires.

## Risks / Trade-offs

- **One extra keystroke per single-comment send.** A user who
  only ever sends one comment pays for `c → type → ⏎ → s`
  instead of `c → type → ⏎`. Trivially offset by the multi-comment
  case (no longer N tmux round-trips for N comments) and the
  clearer mental model.
- **Stale comments accumulate silently** if the user stages some,
  navigates away, then forgets they exist. The existing
  footnote rendering (yellow gutter + `▸`/`•` line) makes them
  visible in the viewport, so this is a soft risk. Not adding a
  status-bar counter in this change.
- **Behavioral revert relative to the archived `save-and-send-on-
  enter` change.** That change explicitly moved `s` to a retry
  path. The spec prose describing `s` as the retry path is now
  wrong and must be rewritten (handled in the spec delta).
- **Two callers of `FormatCommentsAppendix`, two prefix rules.**
  Easy to forget the rule when adding a third caller. Mitigation:
  doc comment on `FormatCommentsAppendix` warns "does not include
  the leading `---`; caller composes it when preceding text exists".