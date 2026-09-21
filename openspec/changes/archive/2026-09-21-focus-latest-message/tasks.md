# Tasks

## 1. Dependencies

- [x] 1.1 Add `github.com/charmbracelet/glamour` to `go.mod` and run `go mod tidy`
- [x] 1.2 Verify `go build ./...` succeeds with the new dependency

## 2. Data model

- [x] 2.1 Replace `m.entries []entry` with `m.latest entry` in `model.go`
- [x] 2.2 Replace the append loop in the `sessionMsg` handler with a single replace (`m.latest = last(msg.entries)`)
- [x] 2.3 Verify `go build ./...` succeeds after the model rewrite

## 3. Markdown rendering

- [x] 3.1 Add a glamour renderer cache keyed by width (`map[int]*glamour.TermRenderer`) to the model
- [x] 3.2 Implement `getRenderer(width)` that returns (or builds) a `TermRenderer` configured with `glamour.WithStandardStyle("dark")` and `glamour.WithWordWrap(width)`
- [x] 3.3 Rewrite `refreshViewport` to render `m.latest.text` via glamour; clear the renderer cache and rebuild on `tea.WindowSizeMsg`
- [x] 3.4 Manually verify pinky compiles and renders a sample agent message (smoke test against a live pi/codex session)

## 4. Block indexing

- [x] 4.1 Add a goldmark-based parser helper that returns top-level block AST nodes (`heading`, `paragraph`, `code`, `list-item`, `blockquote`)
- [x] 4.2 Render each non-empty block individually with glamour and accumulate absolute `startLine` / `endLine` indices in a `[]block` slice
- [x] 4.3 Skip empty blocks, thematic breaks, and HTML blocks from the index
- [x] 4.4 Add unit test `TestBuildBlockIndex_MultiBlockMessage` covering heading + paragraph + code block ordering
- [x] 4.5 Add unit test `TestBuildBlockIndex_ListItemsIndividual` verifying each list item gets its own entry
- [x] 4.6 Add unit test `TestBuildBlockIndex_EmptyParagraphSkipped`

## 5. Block navigation

- [x] 5.1 Add `currentBlockIdx()` helper that returns the index of the block containing the viewport's `YOffset` (the block whose `startLine` is closest to and not greater than `YOffset`); returns `-1` when no block is focused
- [x] 5.2 Add `jumpBlock(delta int)` that scrolls to `m.blocks[idx+delta].startLine` for the current block index; no-op at boundaries
- [x] 5.3 Add `jumpHeading(delta int)` that walks the block index and scrolls to the next/previous `heading` block's `startLine`; no-op at boundaries
- [x] 5.4 Add unit test `TestJumpBlock_AdvancesAndRetreats` covering both directions and the no-op boundary case
- [x] 5.5 Add unit test `TestJumpHeading_SkipsNonHeadingBlocks`

## 6. Vim keybindings

- [x] 6.1 Add `lastVimKey rune` and `lastVimKeyAt time.Time` fields to the model
- [x] 6.2 Implement `handleVimNav(msg tea.KeyMsg)` that handles `j`, `k`, `}`, `{`, `G`, `g`, `]`, `[`
- [x] 6.3 Wire `gg`, `]]`, `[[` two-key state: store the first key + timestamp; on the second matching key within 500ms, fire the jump; clear state on timeout, non-matching second key, or any non-vim key
- [x] 6.4 Reset `lastVimKey` to zero when entering compose mode
- [x] 6.5 Add unit test `TestVimState_TwoKeyTimeout` (verify 500ms boundary)
- [x] 6.6 Add unit test `TestVimState_NonMatchingKeyClears`

## 7. Arrow and page key aliases

- [x] 7.1 Keep `↑` / `↓` mapping to one-line scroll (alias for `k` / `j`)
- [x] 7.2 Map `PgUp` to top of current message (`m.viewport.GotoTop()`)
- [x] 7.3 Map `PgDn` to next block (`m.jumpBlock(+1)`)
- [x] 7.4 Verify all four aliases work alongside the new vim keys without conflict

## 8. Current-block border indicator

- [x] 8.1 Add `borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))` to `model.go`
- [x] 8.2 Modify `refreshViewport` to inject `strings.Repeat("─", m.width)` (styled with `borderStyle`) above the first line and below the last line of the block at `currentBlockIdx()`
- [x] 8.3 Verify the border tracks the viewport when scrolling across block boundaries (manual smoke test)

## 9. Empty state

- [x] 9.1 When `m.latest.text == ""`, render a single dim-styled line `"waiting for agent…"` instead of the glamour-rendered output
- [x] 9.2 Set `m.blocks = nil` in the empty case so no border is drawn
- [x] 9.3 Verify the placeholder appears on attach before any agent message arrives

## 10. Streaming behavior

- [x] 10.1 Confirm `m.viewport.GotoBottom()` fires after each refresh that appended new text
- [x] 10.2 Verify the status line's streaming indicator (`●` vs `·`) toggles correctly while the agent is mid-write and after

## 11. History

- [x] 11.1 Remove the `history.Load(pane)` call from `attach` (no re-seed on startup)
- [x] 11.2 Confirm `history.Append` continues to be called for each surfaced assistant message and each user redirect
- [x] 11.3 Verify the JSONL file format is unchanged (existing files remain valid; `jq` and similar tools still work)

## 12. End-to-end verification

- [ ] 12.1 Run pinky against a live pi or codex session (manual, deferred to operator)
- [ ] 12.2 Verify markdown rendering, block navigation, vim keys, border indicator, empty state, and yank-to-bottom behavior end-to-end (manual, deferred to operator)
- [x] 12.3 Update `README.md`: remove "Markdown rendering" from the v0 out-of-scope list; add a one-paragraph description of the new navigation model under "Keys"
- [x] 12.4 Run `go vet ./...` and `go test ./...` to confirm no regressions
