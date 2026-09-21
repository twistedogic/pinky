## 1. Cursor data model

- [ ] 1.1 Add `cursor {blockIdx int, charPos int}` and
      `selection {blockIdx int, charA int, charC int}` types to
      `internal/render/render.go`; add constructors
      `cursorAtStart(blocks []Block) cursor` and
      `selectionNone() selection` (returns zero value).
- [ ] 1.2 Add `Mode` flag (`selNone`, `selLine`) on a new
      `render.NavState` type (replaces `SelectionMode`); keep
      `Mode` semantics identical (SelNone | SelLine only).
- [ ] 1.3 Add `LineIndex(blocks []Block, c cursor) int` helper
      that maps `(blockIdx, charPos)` to a rendered line index
      within the concatenated viewport content (uses
      `LineByteOffset` from `add-block-and-inline-comments`).
- [ ] 1.4 Tests `internal/render/cursor_test.go`: cursor at block
      start lands on `block.StartLine`; cursor at charPos past the
      block's last byte clamps to `block.EndLine`; cursor on a
      block that wraps N rendered lines lands within `StartLine+
      0..N-1`.

## 2. Nav state machine (`internal/render/nav.go`)

- [ ] 2.1 Add `Action` enum (`ActionNone`,
      `ActionBlockDown/Up`, `ActionRuneLeft/Right`,
      `ActionEnterVisual/ExitVisual`, `ActionComment`,
      `ActionSend/Refresh/Quit/Compose/Help`) to
      `internal/render/nav.go`.
- [ ] 2.2 Add `NavState {Visual Mode; Anchor Anchor}` to nav.go;
      `Anchor {blockIdx int; charPos int}` carries the visual
      anchor (set on `v`, used to compute `selection.charA/charC`
      on each motion).
- [ ] 2.3 Implement `Handle(r rune, st *NavState, cursor *cursor,
      blocks []Block) Action` that:
      - Returns `ActionNone` for unrecognised keys.
      - On `j`: increments `cursor.blockIdx` (clamped
        `[0, len(blocks)-1]`); on selection path, extends
        `selection` to the new cursor's `charC`; sets
        `charPos` to block start (per D1).
      - On `k`: same, decrement, clamped.
      - On `h`: decrements `cursor.charPos` (clamped
        `[0, len(block.Source)]`); on selection path, extends
        `selection.charC` to new `charPos`.
      - On `l`: same, increment, clamped.
      - On `v`: toggles `st.Visual` and seeds `st.Anchor` from
        current cursor; returns `ActionEnterVisual` /
        `ActionExitVisual`.
      - On `c`: returns `ActionComment` (selection shape
        computed by caller from anchor + cursor).
      - On `s`: returns `ActionSend`.
      - On `q`: returns `ActionQuit`.
      - On `r`: returns `ActionRefresh`.
      - On `n`: returns `ActionCompose`.
      - On `?`: returns `ActionHelp`.
      - On `0x1b` (Esc): if `st.Visual == SelLine` returns
        `ActionExitVisual`, else `ActionNone`.
- [ ] 2.4 Tests `internal/render/nav_test.go`:
      - `j` from middle block lands on next block;
      - `k` from middle block lands on prev block;
      - `j` from last block is a no-op;
      - `h/l` clamp at byte boundaries;
      - `v` toggles state and seeds anchor;
      - `v` while visual exits visual without resetting anchor;
      - `Esc` while visual exits; outside visual is no-op;
      - `c` returns `ActionComment` and selection reflects the
        anchor-to-cursor range across one or more blocks.

## 3. Delete obsolete render files

- [ ] 3.1 Confirm `nav.Handle` (D2) covers every key the old
      `VimState.Handle` and `VisualState.Handle` did; mark
      `vim.go` for deletion.
- [ ] 3.2 Delete `internal/render/vim.go`,
      `internal/render/vim_test.go`,
      `internal/render/visual.go`,
      `internal/render/visual_test.go`. Move any helpers still
      in use (e.g. `JumpBlock`, `JumpHeading`,
      `CurrentBlockIdx`, `blockByteOffset`) into `nav.go` or
      `render.go`.
- [ ] 3.3 Replace `VimAction`, `VisualAction`,
      `SelectionMode`, `VimState`, `VisualState`, `visualState
      = render.VisualState` usages throughout `model.go` with
      the new types.

## 4. Model: introduce `cursor` and `selection`

- [ ] 4.1 Add `cursor cursor`, `selection selection`,
      `nav render.NavState` fields to `model`; initialise
      `cursor.blockIdx = 0, charPos = 0` in `newModel` /
      `attach()`.
- [ ] 4.2 Replace `m.visual.Mode == render.SelLine` checks
      throughout with `m.nav.Visual == render.SelLine`.
- [ ] 4.3 Replace `m.visual.Anchor/Cursor/CharA/CharC/CurBlock`
      reads with `m.selection` / `m.nav.Anchor` / `LineIndex(...)`
      derivations.
- [ ] 4.4 Add `cursorOnScreen(c cursor, viewport viewport.Model)
      bool` (returns true when `viewport.YOffset ≤ LineIndex <
      YOffset+Height`); used by `handleNavKey` to scroll-into-view.

## 5. Model: collapse to `handleNavKey`

- [ ] 5.1 Implement `handleNavKey(msg tea.KeyMsg) (tea.Model,
      tea.Cmd)` that:
      - Intercepts `?` (help toggle) and `Ctrl+C` is now gone —
        use `q` for quit, no global Ctrl+C.
      - For `tea.KeyRunes` with one rune: dispatches to
        `nav.Handle(...)`, then for each returned action executes
        the model-level side-effect:
        - `ActionBlockDown/Up/RuneLeft/Right`: write back the
          cursor, compute selection update, then
          `scrollCursorIntoView()` and `m.refreshViewport()`.
        - `ActionEnterVisual`: seed `m.nav.Anchor` =
          `m.cursor`; set `m.nav.Visual = SelLine`.
        - `ActionExitVisual`: set `m.nav.Visual = SelNone`.
        - `ActionComment`: build the `commentAnchor` (see D5) and
          call `m.enterCommentComposer(anchor)`.
        - `ActionSend`: call `m.handleSend()` (see §6).
        - `ActionRefresh`: return `pollCmd(m.src)`.
        - `ActionQuit`: return `tea.Quit`.
        - `ActionCompose`: call `m.enterCompose()`.
        - `ActionHelp`: handled at the top (before `?` call).
      - For other key types: forward to viewport (`PageUp`,
        `PageDown`, `Home`, `End`, arrow keys).
- [ ] 5.2 Delete `handleIdleKey` and `handleVisualKey`.
- [ ] 5.3 Update `handleKey` so `stateNav` dispatches to
      `handleNavKey`; rename `stateIdle` references to
      `stateNav` (see §7).

## 6. Universal send on `s`

- [ ] 6.1 Extract the existing compose-send body into
      `handleComposeSend() (tea.Cmd)` that:
      - Reads `m.textarea.Value()`; returns nil if empty.
      - Appends the comments appendix if `m.includeComments &&
        len(m.comments) > 0`.
      - Calls `m.hist.Append(...)` and `inject.Send(...)`.
      - On send error, replaces `m.latest` with the
        `[send failed: ...]` placeholder (unchanged).
- [ ] 6.2 Add `handleSend() tea.Cmd` that branches on `m.state`:
      `stateNav → m.submitAllComments()`,
      `stateCompose → m.handleComposeSend()`, else nil.
      `submitAllComments` is unchanged (already a no-op when
      `len(m.comments) == 0`).
- [ ] 6.3 Tests `model_test.go`:
      - `TestSend_NavWithCommentsCallsSubmitAllComments`.
      - `TestSend_NavWithoutCommentsIsNoop`.
      - `TestSend_ComposeWithTextSendsViaInject`.
      - `TestSend_ComposeWithIncludeCommentsAppendsAppendix`.
      - `TestSend_ComposeWithEmptyTextIsNoop`.

## 7. Rename `stateIdle` → `stateNav`

- [ ] 7.1 Update the enum constant in `model.go`.
- [ ] 7.2 Update `handleKey`'s switch arms (idle arm → nav arm).
- [ ] 7.3 Update `View()` and `reflow()` switch arms.
- [ ] 7.4 Update `enterCompose`, `enterCommentComposer`,
      `saveComment`, `cancelCommentComposer` switch arms.
- [ ] 7.5 Update `help.KeyMap` implementations
      (`ShortHelp` / `FullHelp`) where they switch on state.
- [ ] 7.6 Update `keymap_test.go` references (`stateIdle` →
      `stateNav`, plus the keys exercised in each test).

## 8. Collapse `keymap.go`

- [ ] 8.1 Delete `LineUp`, `LineDown`, `PrevBlock`, `NextBlock`,
      `BottomLine`, `Refresh`, `QuitIdle`, `Mark`,
      `EditComment`, `DeleteComment`, `NextComment`,
      `PrevComment`, `SubmitComments`,
      `VisualDown`, `VisualUp`, `VisualNextBlock`,
      `VisualPrevBlock`, `VisualOpen`, `VisualExit`, `Send`,
      `QuitError` `key.Binding` fields.
- [ ] 8.2 Replace with a single `Nav` `key.Binding` group
      containing the per-rune nav keys (`j k h l v c s q r n ?`).
      `key.Binding` is no longer the dispatch mechanism for nav —
      `handleNavKey` reads rune from `tea.KeyRunes` and goes
      straight to `nav.Handle`. The `Nav` group exists only for
      the help overlay's `FullHelp`.
- [ ] 8.3 Keep `Help`, `Newline`, `Cancel`, `IncludeComments`
      bindings (these still match via `key.Matches`).
- [ ] 8.4 Update the `key.Binding` for `Error` (any key dismisses
      + quits) — unchanged behaviour; new key `Esc` or just any
      rune works.

## 9. Help overlay

- [ ] 9.1 Update `keymapGroupsForState` in `keymap_md.go` so the
      nav group lists `j k h l v Esc c s q r n ?` (no headings,
      no `m`/`M`/`e`/`d`/`n`/`N` collision with the new nav).
- [ ] 9.2 Remove the `mark`, `visual`, `comments` groups
      (collapsed into `nav`).
- [ ] 9.3 Compose group: drop `Ctrl+S` (now `s`); keep
      `Enter`, `Esc`, `Ctrl+I`.
- [ ] 9.4 Comment composer group: keep `Ctrl+S`, `Esc`.
- [ ] 9.5 Test `keymap_md_test.go` (or `help_test.go`):
      - Nav short-help lists the top-5 keys.
      - Nav full-help lists every nav key, grouped.
      - Compose short-help does not list `Ctrl+S`.
      - `q` / `s` / `r` / `n` appear once each, in the nav group.

## 10. Render: highlight selection between cursor positions

- [ ] 10.1 In `RenderMessageWithComments` (or replace it with
      `RenderMessageWithSelection`), pass `cursor` and
      `selection` as additional parameters; apply a selection
      background tint across the rendered line range covered by
      `[selection.charA, selection.charC)` (reusing the existing
      projection that comments already use).
- [ ] 10.2 Add `ApplySelection(lines string, blocks []Block,
      cursor cursor, selection selection) string` helper that
      paints the background tint for the rendered lines that
      intersect the selection range.
- [ ] 10.3 Update `injectBorder` / `injectHighlights` order so
      border sits on top of selection tint and footnote tint
      (selection < comments < border).
- [ ] 10.4 Tests `render_test.go`:
      - Selection inside a single paragraph highlights the
        expected rendered-line range.
      - Selection spanning three blocks highlights each block's
        covered lines.
      - Selection cleared (visual off) renders no tint.
      - 1-line drift on a wrapping paragraph accepted.

## 11. Cursor-driven viewport scroll

- [ ] 11.1 After every motion action, call
      `m.scrollCursorIntoView()` which:
      - Computes `targetY = render.LineIndex(m.blocks, m.cursor)`
        (using D8 helper).
      - If `targetY < YOffset` or `targetY ≥ YOffset + Height`,
        sets `YOffset = targetY - 1` (cushion) clamped to
        `[0, totalLines - Height]`.
      - Else leaves `YOffset` alone.
- [ ] 11.2 Replace `m.viewport.LineUp(1)` / `LineDown(1)` calls
      in the old `handleIdleKey` with the cursor-motion path.
- [ ] 11.3 Update `refreshViewport` to keep the bottom-anchor
      behaviour when new assistant content arrives (changes to
      `latest.text` → `m.cursor` stays put at the same byte
      offset if possible, else jumps to the end).

## 12. Test updates

- [ ] 12.1 Update `keymap_test.go` for new key surface: replace
      `TestIdleKey_*` with `TestNavKey_*`; verify `q` quits,
      `n` enters compose, `c` opens comment composer (selection
      first), `r` refreshes, `v` enters visual and second `v`
      exits.
- [ ] 12.2 Add `TestNavKey_V_EnterThenEsc_Exits` (visual toggle
      round-trip).
- [ ] 12.3 Add `TestNavKey_C_WithSelection_AnchorsSelection`,
      `TestNavKey_C_WithoutSelection_AnchorsBlock`.
- [ ] 12.4 Add `TestNavKey_JK_HL_CursorOnly_NoSelection`.
- [ ] 12.5 Add `TestNavKey_S_NavWithCommentsSends`.
- [ ] 12.6 Add `TestNavKey_R_TriggersPoll`.
- [ ] 12.7 Add `TestCursor_LandsOnStartLine` regression test
      (selected from `add-block-and-inline-comments`).
- [ ] 12.8 Update `model_test.go` for the renamed state
      (`stateIdle` → `stateNav` everywhere).

## 13. End-to-end smoke test

- [ ] 13.1 `go build ./...` clean.
- [ ] 13.2 `go vet ./...` clean.
- [ ] 13.3 `go test ./...` clean (covers the new nav_test,
      cursor_test, render_test cases).
- [ ] 13.4 `openspec validate vim-keymap` reports no errors.
- [ ] 13.5 Manual: run pinky against a fake `pi` session in a
      tmux split; exercise the nav surface end-to-end:
      - Idle: `j`/`k`/`h`/`l` move the cursor and visible
        viewport follows; `v` enters visual; `v` exits or `Esc`
        exits; `c` over selection opens composer with the
        selection anchors; `c` over no selection opens with a
        full-block anchor; `s` with accumulated comments batches
        the send; `s` in compose sends the text (+ appendix if
        `Ctrl+I` is on); `n` enters compose; `r` refreshes; `q`
        quits; `?` toggles help.
      - Compose: `s` sends, `Enter` newline, `Ctrl+I` toggles
        appendix, `Esc` cancels.
      - Comment composer: `Ctrl+S` saves, `Esc` cancels.
      - Selection: `v j k h l` extends the highlight across
        blocks; the projected highlight shows on rendered lines.
