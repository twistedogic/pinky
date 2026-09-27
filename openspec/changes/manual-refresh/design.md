# Design

## Context

Pinky attaches to a `session.Source` and renders the agent's most recent
assistant message into a Bubble Tea viewport. Today the surface is
push-based: `Init()` schedules a `tea.Tick(pollInterval)` (500 ms), and
every `sessionMsg` result reschedules another tick from within the
`Update` handler. Three branches in the `sessionMsg` case each return
`m, pollCmd(m.src)` — the err branch, the empty-poll branch, and the
end-of-case fallthrough. Removing the recurring poll means turning those
three returns into `m, nil`. The `Init()` poll stays, the
`handleNavKey`'s `ActionRefresh` branch (`r`) already calls
`pollCmd(m.src)` and stays, and the comment-clearing line
(`if last.Text != m.latest.Text { m.comments = nil }`) is intentionally
preserved — it now runs under both the init poll and `r` presses, and
its conditional behaviour is the spec contract.

See `proposal.md` for motivation and `specs/latest-message-view/spec.md`
for the behaviour contract.

## Goals / Non-Goals

**Goals:**

- Stop the recurring 500 ms tick once the `Init()` poll resolves.
- Keep `r` as the single user-driven fetch trigger, firing exactly one
  poll per press.
- Preserve the existing conditional comment-clear on text-change (now
  fires under init and under `r`).
- Flip the three polling tests in `poll_test.go` to assert the new
  contract.
- No behavioural changes outside the polling loop.

**Non-Goals:**

- No new keys, no help-overlay text change. The `r — re-poll session`
  row stays accurate.
- No changes to the comment composer, send/save flow, or file tab.
- No changes to `Init()`'s one-shot poll (the screen warms up exactly
  as today).
- No changes to `m.streaming`, the `●`/`·` status dot, or any visual
  indicator. The dot just flips less often.
- No buffered / turn-end commit semantics. The handler still commits
  immediately on the first poll with text — the aspirational
  `pendingLatest` comments in `picker_test.go` are not in scope.

## Decisions

### D1 — Re-arm removal at three sites, one mechanical change

The `sessionMsg` case in `model.go`'s `Update` has three returns that
each carry `pollCmd(m.src)`:

```
err branch      → return m, pollCmd(m.src)
empty branch    → return m, pollCmd(m.src)
end-of-case     → return m, pollCmd(m.src)
```

Replace each with `return m, nil`. The `pollCmd` factory itself
(`model.go:325`) is unchanged — it is still used by `Init()` and by
`handleNavKey`'s `ActionRefresh` branch. The
`// ponytail: every poll result must re-arm the next Tick, …`
comment above the case becomes a comment about the *removed*
behaviour (a `ponytail:` note naming the ceiling: "polling stops
deliberately after the first message; `r` is the manual gate, see
proposal.md"). The ponytail-test convention in this repo puts such
notes on the line that *carries* the simplification, so it sits at
the new `return m, nil` at the end of the case.

**Alternative:** Centralise the re-arm into a single `defer` or
helper that decides whether to re-arm. Rejected — the three sites
are not symmetric (err vs empty vs text) and a guard helper is more
code than three identical line changes. The ponytail ladder says:
shortest diff in the right place wins.

### D2 — Comment-clearing stays in the `sessionMsg` handler, not at the `r`-press site

The spec contract is "clear comments when text changed". A `r` press
with no new content is a true no-op — comments survive. That maps to
the existing line in the handler:

```go
if last.Text != m.latest.Text {
    m.comments = nil
}
```

We considered moving the clear to `handleNavKey`'s `ActionRefresh`
branch (clear before firing the poll, regardless of what comes back).
That would make `r` always destructive. The spec resolves that
question in favour of "clear only on text change", so the clear stays
where it is. The `Init()` poll also runs through this same line, but
on a fresh attach `m.comments` is `nil` already, so the clear is a
no-op in that path.

**Alternative:** Clear unconditionally at the `r`-press site. Already
ruled out by the spec.

### D3 — Three test flips, no new tests

`poll_test.go` currently has three tests that assert the recurring
tick. After the change those tests should assert the *absence* of a
follow-up tick. The flips are:

| Old test                                         | New assertion                                              |
|--------------------------------------------------|------------------------------------------------------------|
| `TestSessionMsg_ReschedulesPoll_OnEntries`       | `cmd == nil` after `sessionMsg` with entries               |
| `TestSessionMsg_ReschedulesPoll_OnEmpty`         | `cmd == nil` after empty `sessionMsg`                      |
| `TestSessionMsg_ContinuousPollingDeliversNewMessages` | Drive an `r` press manually; second `r` after codex appends delivers the new message |

The third test is a rewrite, not a flip: it currently chains two
sessionMsgs and asserts the second one carries the new message. The
new shape is: first `r` press (Init or `ActionRefresh`), sessionMsg
arrives with the first message, no reschedule. Codex appends a second
message. Second `r` press, sessionMsg arrives with the second
message. Assert `m.latest.Text` is the second message.

No new tests are introduced. The seven scenarios in `picker_test.go`
still pass because the commit logic and comment-clear line are
unchanged — they describe handler behaviour that survives the diff.

**Alternative:** Add new tests for `r`-press semantics. Rejected — the
existing handler-level tests already exercise the path; adding more
asserts the same code twice.

### D4 — `m.streaming` and the `●`/`·` dot are unchanged

The status-line dot flips when text arrives (true) or when the poll
is empty (false). Under manual cadence it just flips less often — a
dot that says `●` after attach and stays there is a minor
inconsistency, not a bug. Touching the dot would expand the diff into
`statusLine()` and possibly the help footer; not worth it for this
change. Tracked as an open question.

**Alternative:** Force `m.streaming = false` on every `r` press so the
dot always resets. Considered; rejected — it removes a tiny bit of
useful information ("this fetch brought text") for a marginal UI
consistency win.

## Risks / Trade-offs

- **Help-label drift** — the keymap row still says "re-poll session".
  That phrase is accurate (each `r` press re-polls once) but the
  *implication* under auto-polling was "always re-poll"; under manual
  refresh it's "this is the only way to re-poll". Acceptable; the
  help row is the same string the user already reads.
- **First-paint latency unchanged** — `Init()` still fires a 500 ms
  `tea.Tick`. The screen shows "waiting for agent…" for up to 500 ms
  after attach. No regression.
- **`m.streaming` becomes stale-state** — a user who attaches, sees
  `●`, and walks away leaves a dot that says "streaming" indefinitely.
  The flag is no longer the source-of-truth signal it implied under
  auto-polling. Cosmetic. If it bothers anyone, follow-up work can
  force `streaming = false` after a timeout or on `r`.
- **Test coverage gap** — there is no explicit test asserting "second
  `r` press delivers new messages after a session-source append". The
  rewrite of `TestSessionMsg_ContinuousPollingDeliversNewMessages`
  covers the round-trip; if that test is too coarse, a focused test
  can be added later.
- **No buffered-mode semantic** — `picker_test.go` comments describe
  a `pendingLatest` buffered mode where text is held until an empty
  poll signals turn-end. The code never had that state. This change
  does not introduce it. If the buffered semantic is wanted later,
  it's a separate change with its own spec deltas.

## Migration Plan

Single deploy. No persisted state. Reverting means `git revert` of
the merge commit. The `pollInterval` constant and the `pollCmd`
factory remain — only their *callers* change. The three return-sites
in `sessionMsg` flip together so the change is atomic.

## Open Questions

- **Should the `●` indicator eventually decay?** Under manual cadence
  a stale `●` after a long absence is technically wrong. Two options
  if it ever matters: (a) tie `streaming` to "polled within last N
  seconds" via a small timestamp on the model, (b) force `streaming
  = false` on every `r` press. Neither is blocking — leave the field
  alone for now.
- **Should the buffered `pendingLatest` semantic ever land?** The
  test comments in `picker_test.go` describe it; the code never
  implemented it. Out of scope here; if it lands, it's a separate
  change.