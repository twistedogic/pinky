# Spec Delta

## ADDED Requirements

### Requirement: Manual refresh via `r` after attach

After the one-shot `Init()` poll has resolved, the system SHALL NOT
poll the watched agent session on a timer. The only way for the user
to surface new assistant text in the main view after attach SHALL be
by pressing `r` in `stateNav`. Each `r` press SHALL trigger exactly
one fetch from the session source. After that fetch resolves, the
system SHALL return to idle with no further `tea.Tick` scheduled.

The `Init()` poll SHALL fire within 500 ms of attach so the main view
is populated before the user can reasonably press any key. If the
`Init()` poll resolves with no assistant message, the main view SHALL
remain on its "waiting for agent…" placeholder until the user presses
`r` and a subsequent fetch surfaces a message.

A `r` press that resolves with the same assistant text as the
current `m.latest` (i.e. the session source has nothing new) SHALL be
a no-op: `m.latest` is unchanged, `m.comments` is preserved, and the
viewport position is preserved.

A `r` press that resolves with a different assistant text SHALL
replace `m.latest`, clear `m.comments` (so any annotations on the
prior message do not leak into the new one), and re-render the
viewport.

#### Scenario: `Init()` fires one warm-up poll

- **WHEN** pinky enters `stateNav` from attach
- **THEN** the system returns exactly one `pollCmd(m.src)` from
  `Init()` and does not schedule any further tick from the resulting
  `sessionMsg` handler

#### Scenario: `r` press is the only post-attach fetch trigger

- **WHEN** the user is in `stateNav` and the `Init()` poll has
  already resolved
- **THEN** pressing `r` returns exactly one `pollCmd(m.src)` from
  `handleNavKey`'s `ActionRefresh` branch, and the `sessionMsg`
  handler does not schedule a follow-up poll

#### Scenario: `r` with no new content is a true no-op

- **WHEN** the user presses `r` and the session source returns no
  new assistant messages
- **THEN** `m.latest.Text` is unchanged, `m.comments` retains every
  previously-saved annotation, and the viewport's `YOffset` is
  unchanged

#### Scenario: `r` with new content replaces `m.latest` and clears comments

- **WHEN** the user presses `r` and the session source returns a
  assistant message whose `Text` differs from `m.latest.Text`
- **THEN** `m.latest` is replaced with the new message,
  `m.comments` is set to `nil`, the viewport is re-rendered, and the
  viewport is pinned to the top (per "New message always scrolls to
  bottom")

#### Scenario: `r` while a session error is active

- **WHEN** the user presses `r` and the session source returns an
  error
- **THEN** the error is surfaced via the existing error-handling
  path and no follow-up poll is scheduled

## MODIFIED Requirements

### Requirement: Single-letter nav surface in stateNav

In `stateNav` (formerly `stateIdle`) the system SHALL provide a
single-letter nav surface. The bindings and actions are:

| key  | action |
|------|--------|
| `j`  | move cursor to next block |
| `k`  | move cursor to previous block |
| `h`  | move cursor one rune left within the current block |
| `l`  | move cursor one rune right within the current block |
| `v`  | enter visual mode (toggle); re-press while visual exits |
| `Esc`| exit visual mode (no-op when not in visual) |
| `c`  | open comment composer (selection-anchored if visual, block-anchored otherwise) |
| `s`  | send to agent (idle batch when comments exist; compose otherwise) |
| `r`  | fetch the tailed session once and re-render the latest message (the only post-attach fetch trigger) |
| `q`  | quit pinky |
| `?`  | toggle short / full help overlay |

The previous two-key state machine (`gg`, `]]`, `[[`) and the
`Ctrl+C` / `Ctrl+N` / `Ctrl+R` / `Ctrl+S` / `Ctrl+I` bindings
SHALL NOT exist in `stateNav`.

#### Scenario: `j` moves to the next block
- **WHEN** the user presses `j` in `stateNav`
- **THEN** the cursor's `blockIdx` increments by one (clamped at
  the last block) and the viewport scrolls to keep the cursor
  visible

#### Scenario: `k` moves to the previous block
- **WHEN** the user presses `k` in `stateNav`
- **THEN** the cursor's `blockIdx` decrements by one (clamped at
  the first block) and the viewport scrolls to keep the cursor
  visible

#### Scenario: `h` moves one rune left
- **WHEN** the user presses `h` in `stateNav` and the cursor's
  `charPos` is greater than `0`
- **THEN** the cursor's `charPos` decreases by the byte length of
  the rune preceding it

#### Scenario: `l` moves one rune right
- **WHEN** the user presses `l` in `stateNav` and the cursor's
  `charPos` is less than `len(block.Source)`
- **THEN** the cursor's `charPos` increases by the byte length of
  the rune at the current offset

#### Scenario: Two-key sequences do not exist
- **WHEN** the user presses `g` followed by `g` in `stateNav`
- **THEN** the first `g` is processed as an unknown rune (no
  state, no timeout); the second `g` is processed as an unknown
  rune. No `gg` action fires.