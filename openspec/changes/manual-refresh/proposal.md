# Proposal

## Why

Pinky currently polls the watched agent session every 500 ms for the entire
lifetime of the attach. The poll keeps `m.latest` in sync with whatever the
agent last emitted, but it has three costs that have stopped being worth
it: (1) a steady stream of `tea.Tick` work even when the user is just
reading a finished message, (2) comments silently evaporating the moment
the agent appends a few more bytes to the same message, and (3) the
implicit "I am watching" assumption in `latest-message-view` requirements,
which makes the surface feel reactive even when the user wants a static
read. Switching to a manual `r` refresh — with one warm-up poll at attach
so the screen is never blank — gives the user explicit control over when
the view re-reads the session and lets `latest-message-view` document
that the surface is pull-based, not pushed.

## What Changes

- **Remove the recurring poll.** `pollCmd(m.src)` is no longer returned
  from any branch of the `sessionMsg` handler in `model.go`. Polling
  stops after the first `Init()` poll completes. **No new tick is
  scheduled.**
- **Keep the one-shot init poll.** `Init()` still returns `pollCmd(m.src)`
  when entering `stateNav`, so the screen populates within the first
  ~500 ms of attach. After that warm-up poll resolves, the loop ends.
- **`r` becomes the only way to fetch.** `handleNavKey`'s
  `ActionRefresh` branch already calls `pollCmd(m.src)`; it keeps doing
  so. The keymap label "r — re-poll session" stays accurate. Each `r`
  press fires exactly one poll.
- **Comment-clearing semantics are unchanged.** The existing
  `if last.Text != m.latest.Text { m.comments = nil }` line in the
  `sessionMsg` handler is preserved. Comments are cleared when the
  surfaced assistant text actually changes; a no-op `r` (source returned
  nothing new) is a true no-op and keeps the user's annotations intact.
- **No new requirements introduced.** This is a tightening of existing
  behaviour, not a feature addition. Capability delta lives entirely in
  `latest-message-view`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `latest-message-view`: requirements that assume pinky polls the session
  on a timer need to be re-stated in terms of "the user pressed `r`" or
  "the init poll resolved". In practice this means one requirement
  ("Show latest complete agent message") gains a clarification that the
  surface is pull-based after attach, and the "Yank to bottom on new
  content" requirement's polling mentions get rewritten against the
  manual refresh trigger. The single-letter nav surface table's `r`
  row gets a sharper description.

## Impact

- `model.go`:
  - `Update`'s `sessionMsg` case: three `return m, pollCmd(m.src)`
    lines become `return m, nil` (err branch, empty branch, end-of-case).
  - `Init()`: unchanged.
  - `handleNavKey` `ActionRefresh` branch: unchanged.
- `poll_test.go`:
  - `TestSessionMsg_ReschedulesPoll_OnEntries` → assertion flips to
    "must NOT reschedule after delivery".
  - `TestSessionMsg_ReschedulesPoll_OnEmpty` → same flip.
  - `TestSessionMsg_ContinuousPollingDeliversNewMessages` → rewritten as
    a manual-refresh scenario: "second `r` press delivers new messages".
- `picker_test.go`: untouched (its scenarios describe the
  handler's commit / clear-comments logic, which is unchanged).
- No new dependencies. No keymap changes. No help-overlay text changes
  (the existing `r — re-poll session` row is now load-bearing instead
  of aspirational).
- `hist.Append` semantics unchanged. Source offset advances per call,
  so manual cadence still yields one history entry per new assistant
  message.
- `m.streaming` semantics unchanged at the field level. The dot just
  flips less often. Tracked as an open question in `design.md`.