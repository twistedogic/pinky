## Context

`pinky` is a TUI that sits next to a coding agent (pi / codex) in a
tmux split. It tails the agent's session output, renders the latest
assistant message as markdown (per `latest-message-view`), and lets
the user compose a redirect message back to the agent (per
`agent-redirect`). Since `add-block-and-inline-comments` it also
supports block-anchored and inline-anchored annotations on the
rendered message.

The current implementation routes navigation through two parallel
pipelines:

1. **Idle pipeline.** `handleIdleKey` matches `key.Binding`s
   (`LineUp`, `LineDown`, `PrevBlock`, `NextBlock`, `BottomLine`,
   `Compose`, `Refresh`, `QuitIdle`, plus the comments bindings
   `Mark`, `Visual`, `EditComment`, `DeleteComment`, `NextComment`,
   `PrevComment`, `SubmitComments`) and falls through to a
   `render.VimState` two-key state machine for `gg`, `]]`, `[[`.
   "Where am I" is derived from `m.viewport.YOffset` via
   `render.CurrentBlockIdx(blocks, YOffset)`.

2. **Visual pipeline.** `handleVisualKey` (`m.visual.Mode ==
   SelLine`) ignores all bindings and feeds the rune directly to
   `render.VisualState.Handle`, which manages its own
   `Anchor/Cursor/CharA/CharC/CurBlock` and returns `VisualAction`
   values that map onto the same set of motions (`j`, `k`, `}`, `{`,
   plus `c` for composer and `Esc` for exit). The visual bindings
   (`VisualDown/Up/...`) declared in `keymap.go` are
   "documentation-only" — nothing dispatches them.

The cursor itself is a derived quantity with three independent
sources: `viewport.YOffset` (idle), `visual.cursor` (visual line
index), and `visual.CharC` (visual byte offset). When the user
scrolls the viewport in idle and then enters visual, the visual
cursor parks at the block containing `YOffset` — fine — but if the
user then `j`s through blocks, the border highlight on
`injectBorder` only re-bakes after a `m.refreshViewport()` call
because the visual cursor's `CurBlock` does not equal
`CurrentBlockIdx(blocks, YOffset)`. The two notions of "focused
block" get out of sync.

Goal of the change: one cursor field on the model, one nav state
machine in `internal/render`, one dispatcher in `model.go`, all
nav keys single-letter.

Stakeholders: any user of pinky who has learned the j/k/{ /} /
m / V / e / d / n / N / s / ` / `gg` / `]]` / `[[` / `Ctrl+R` /
`Ctrl+N` / `Ctrl+S` grammar — i.e. us.

## Goals / Non-Goals

**Goals:**

- One cursor (`m.cursor {blockIdx, charPos}`) replaces the implicit
  `YOffset` cursor and the `m.visual` cursor triplet.
- One nav state machine (`internal/render/nav.go`) replaces
  `vim.go` and `visual.go`. The two-key tracker goes away.
- One dispatcher (`handleNavKey`) replaces `handleIdleKey` +
  `handleVisualKey`.
- Nav surface in `stateNav` is `j k h l v Esc c s q r n ?` —
  all single-letter except `?` and `Esc`.
- `s` is the universal send key across `stateNav` (idle batch) and
  `stateCompose` (free text + optional appendix). Compose's `Ctrl+S`
  is gone.
- `c` with no selection opens a block-level comment composer; the
  `m` binding is gone.
- Visual mode is character-granularity: selection is a byte range
  `[charA, charC)` inside a block (or across blocks with `v` then
  `j`/`k`).
- `stateIdle` is renamed to `stateNav` everywhere — `reflow`,
  `View`, `help`, tests.
- Help overlay collapses (one nav group, smaller compose group,
  identical comment-composer group).

**Non-Goals:**

- Vim-faithful operator-pending mode (no `d{motion}`,
  no `y{motion}`, no text-objects). Single keys only.
- Vim-faithful `:` command palette. Compose keeps `Ctrl+I` for
  the comments appendix.
- Char-granularity across wrapped glamour lines. Byte offsets map
  to source lines; glamour's word-wrap may split one source line
  into N rendered lines and the cursor projection will accept the
  same 1-line drift the comment-projection already accepts.
- Disk persistence, undo, history integration for comments /
  selections.
- Word-motion (`b`/`w`/`e`). Per-rune `h`/`l` only.
- Fold/collapse by block kind.

## Decisions

### D1. Single cursor: `{blockIdx, charPos}`

The model holds one field, `m.cursor {blockIdx int, charPos int}`.
The viewport's `YOffset` is derived: lookup
`m.blocks[m.cursor.blockIdx].StartLine` plus the rendered-line
within the block that contains `charPos`. Comments and visual
selection both anchor against the same cursor.

`charPos` is a byte offset into `m.blocks[blockIdx].Source`. Range
`0..len(Source)` inclusive at both ends (cursor can sit one-past
the last byte, matching vim's normal-mode cursor convention).

**Alternative considered:** keep `YOffset` as the source of truth
and derive `blockIdx` / `charPos` from it (status quo, plus a new
field). Rejected because `YOffset → charPos` is not invertible
under glamour word-wrap; one source line maps to N rendered lines
and we can't pin the cursor to a specific rune from a
`YOffset`-row alone.

**Alternative considered:** keep three cursors (idle, visual,
viewport) and synchronize them per action. Rejected: every motion
needs a sync round-trip; the cleanest mental model is one cursor.

### D2. Single nav state machine: `nav.go`

`internal/render/nav.go` exports one entry point:

```go
type Action int
const (
    ActionNone Action = iota
    ActionBlockDown       // j
    ActionBlockUp         // k
    ActionRuneLeft        // h
    ActionRuneRight       // l
    ActionEnterVisual     // v (only when visual == SelNone)
    ActionExitVisual      // Esc / v (only when visual == SelLine)
    ActionComment         // c → caller decides block-level vs selection
    ActionSend            // s → caller dispatches by state
    ActionRefresh         // r
    ActionQuit            // q
    ActionCompose         // n
    ActionHelp            // ?
)

type State struct {
    Visual Mode              // SelNone or SelLine; flag, not dispatch
    Anchor Anchor            // {blockIdx, charPos}; valid iff Visual == SelLine
}

func Handle(r rune, st *State, cursor *Cursor, blocks []Block) Action
```

The state machine owns: cursor mutations on `j k h l v c`; selection
range mutation on `j k h l v Esc` while `Visual == SelLine`. The
state machine does **not** own comment-composer entry, send, quit,
refresh, compose entry, or help — those are model-level actions
dispatched on the returned `Action`. Mode-change keys (`v`, `Esc`)
are returned as actions; the model updates `st.Visual` /
`st.Anchor`.

**Alternative considered:** keep two state machines and route
visual keys through visual.Handle from a unified dispatch. Rejected:
this just relocates the duplication; visual.Handle still owns
motion logic separately.

**Alternative considered:** keep the two-key `VimState` to support
`gg`, `]]`, `[[` (heading jumps). Rejected: with `j/k` doing block
nav and `h/l` doing rune nav, heading jumps become "next/prev
block whose kind is heading" — implementable with `rune → Action`
by checking if the rune is `]` / `[` and walking back through the
collapsed `ActionBlockDown/Up` against `block.Kind`. No two-key
state machine needed.

### D3. Mode keys intercept at the dispatcher, not in the state machine

`v`, `Esc`, `c` are consumed by `handleNavKey` before calling
`nav.Handle`. Reason: the state machine should return a value
(`Action`); whether the value causes a mode transition is decided
in the model where the state lives. If `v` were handled inside
`nav.Handle`, the state machine would mutate `State.Visual`
itself; we'd then have two writers to the visual flag
(`handleNavKey` for `Esc` and `nav.Handle` for `v`). Single writer
is the model.

### D4. Universal send on `s`, dispatched per state

```go
func (m *model) handleSend() (tea.Model, tea.Cmd) {
    switch m.state {
    case stateNav:
        return m, m.submitAllComments()  // batch send; no-op if comments empty
    case stateCompose:
        return m, m.handleComposeSend()  // existing inject.Send path
    }
    return m, nil
}
```

`stateCommentComposer`'s save is `Ctrl+S` (not `s`): the
comment-composer textarea is a multi-line text input, and `s`
should type in it just like any other letter.

**Alternative considered:** use `s` in comment composer too (save
on `s`, treat `s` as a literal in compose only). Rejected:
bifurcating `s`'s meaning by state is worse than having a single
send key (`s`) and a distinct save key (`Ctrl+S`) for the modal
input.

### D5. `c` with no selection = block-level

Pressing `c` in `stateNav`:

- If `m.selection == nil`, anchor the composer to the whole block
  (`blockIdx = m.cursor.blockIdx`, `charA = 0`,
  `charC = len(Source)`).
- If `m.selection != nil`, anchor to `(selection.blockIdx,
  selection.charA, selection.charC)`.
- If `m.visual.Mode == SelLine`, exit visual and open the
  composer in one keystroke (vim's `c{motion}` flavour, but without
  operator-pending mode).

The previous `m` key is removed. Old behaviour is reachable: `c`
in idle with no selection.

### D6. Help overlay shrinks; nav is one group

`keymapGroupsForState` for `stateNav` collapses to a single `nav`
group with the 11 bindings (`j k h l v c s q r n ?`). The previous
idle groups (`mark`, `visual`, `comments`, `nav`) collapse into one.
`Esc` is documented in the visual-mode context line, not in nav.

### D7. `stateIdle` → `stateNav` rename

Touches: `model.go` (enum + all switch arms in `handleKey`,
`View`, `reflow`, `enterCompose`, `enterCommentComposer`, etc.),
`keymap_test.go` references, any comments mentioning "idle".
Estimated: ~12 call sites in `model.go`, ~3 in tests, comment
strings throughout.

**Alternative considered:** keep the name `stateIdle` and just
unify dispatch internally. Rejected because the user's spec said
"merge into navigation" — the rename makes the new mental model
visible in the code. The call-site churn is small and bounded.

### D8. Existing comment projection reused

`add-block-and-inline-comments` already provides
`render.BlockByteRange`, `LineByteOffset`, `ProjectionLines`. We
reuse them: `charA/charC` in the new comment anchor are byte
offsets (matches D4). The 1-line drift tolerance at glamour
word-wrap boundaries already accepted for comments applies to the
selection highlight in nav mode.

### D9. Auto-scroll: viewport follows cursor (1-line cushion)

After every motion, the viewport's `YOffset` is set so that
`YOffset..YOffset+viewport.Height` contains the rendered line of
`m.cursor`. One-line cushion on top and bottom (so per-rune `h/l`
doesn't visibly jump the viewport). On `j`/`k` the cushion is
allowed to slide before recentering.

**Alternative considered:** vim's `scrolloff=5`. Rejected because
in a TUI with a 1-line status bar and a 1-line help footer, the
viewport is ~16 rows tall — `scrolloff=5` would consume a third of
the visible area and make small movements feel jumpy.

## Risks / Trade-offs

- **[Risk] Selection drift across glamour word-wrap.** A byte
  range inside a source line whose render is wrapped to N lines
  will highlight all N rendered lines; the user expected
  `[charA, charC)` to map cleanly. **Mitigation:** the existing
  comment-projection code already accepts 1-line drift; the
  selection highlight uses the same projection. The byte offsets
  themselves remain exact (used in the redirect appendix).

- **[Risk] Single-key collisions across modes.** `s`, `q`, `c`,
  `r`, `n` are bound in nav AND can be valid letters in the
  comment-composer textarea. **Mitigation:** `stateCommentComposer`
  receives raw rune input and only intercepts `Ctrl+S` and `Esc`;
  all other letters type into the textarea. `stateCompose`
  receives raw input; `Ctrl+S` is gone, so `s` types — and `s` is
  not a model-level key in compose (we use `s` for send via
  `tea.KeyRunes`, which matches BEFORE the textarea sees it). Wait:
  if `s` is intercepted by the model handler, it's not typed.
  Intended. The user agrees with this.

- **[Risk] `?` / `Esc` in textareas.** `?` opens help in any state;
  if the user types `?` in compose expecting a literal `?`, the
  help overlay appears. **Mitigation:** already the case. Match
  the previous design: `?` is global.

- **[Risk] `q` accidental quit.** `q` quits in nav mode with no
  confirmation. **Mitigation:** match vim's no-confirm behaviour
  for `q` — accept the trade-off; users who care can use `Esc`
  followed by `q` deliberately.

- **[Risk] `c` over a block with no selection still requires the
  composer.** Old behaviour (`m`) put a block-anchored composer
  on the current block. New behaviour (`c` with no selection)
  does the same. Old `m` users will hit `m`, get no response,
  and need to learn `c`. **Mitigation:** the help footer
  surfaces `c` prominently. No regression for users who already
  used visual mode + `c`.

- **[Risk] Cross-block selection truncation.** Pressing `v` at the
  start of block N then `j` three times and `c` opens a composer
  with anchor `(blockIdx_first, charA_first,
  charC_of_third_block_last_byte)`. The "excerpt" shown in the
  redirect appendix will include whole-blocks-worth of content.
  **Mitigation:** that's the user's intent; the cursor sits on
  its selection range and the appendix quotes exactly that range.
  Same shape as the existing inline-anchor.

- **[Trade-off] Loses `gg`/`G`/`]]`/`[[` for quick top/bottom and
  heading jumps.** Users who relied on those for fast traversal
  need `j`/`k` (block-by-block) instead. **Accepted:** with `h/l`
  available for fine motion and `j/k` for block stepping, the
  5-key mental model is more uniform; top/bottom reach by holding
  `j` or `k` is fast enough in practice on a typical agent
  message. If this hurts, a follow-up can add
  `Ctrl+G`-prefixed jumps.

- **[Trade-off] `Esc` exits visual only.** `Esc` does nothing in
  nav-no-visual and in compose. Compose cancellation stays
  `Esc`-actually-does-cancel because compose is a modal context.
  **Accepted:** matches vim.

- **[Trade-off] `s` to send can be mistyped in compose.** A user
  intending to type "see" hits `s` and sends. **Accepted:** the
  compose textarea is the same risk as before with `Ctrl+S`. Vim
  users learn not to fat-finger `:`; pinky users learn not to
  fat-finger `s`. The visual cue is the empty textarea.

## Migration Plan

This is a keymap refactor; no on-disk format or wire format
changes.

- The change is a single PR; the test suite
  (`go test ./...` + manual smoke-test against a fake `pi` /
  `codex` session in a tmux split) is the migration gate.
- Help overlay (`?`) is the user-facing migration documentation:
  the new nav group is shown on first idle render. No banner is
  shown — the user pressed `?` already to confirm key layout.
- Rollback: revert the PR. No persisted state is changed;
  comments live in-memory and are discarded on quit either way.
- No CLI flag changes. No `Taskfile.yml` changes.

## Open Questions

- **`s` in `stateCompose` and `stateCommentComposer`**: confirmed
  during exploration — `s` is send in compose, `Ctrl+S` is save in
  comment composer. If the user wants both to be `s` (and save to
  be `s` too), the comment composer becomes single-letter saving
  at the cost of being able to type `s` in the textarea. Current
  decision: leave `Ctrl+S` in `stateCommentComposer`. **Reopen if
  the user pushes back.**

- **Block-level comment excerpt in the appendix**: with `c` and
  no selection, the comment source is the whole block. The
  appendix excerpt uses the existing 40-char truncation. **No
  change** — the existing `FormatCommentsAppendix` already handles
  this.
