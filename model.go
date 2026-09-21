package main

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/twistedogic/pinky/internal/history"
	"github.com/twistedogic/pinky/internal/inject"
	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
)

const (
	composeHeight = 4
	statusHeight  = 1

	placeholderText = "waiting for agent…"

)

type state int

const (
	statePicking state = iota
	stateIdle
	stateCompose
	stateCommentComposer
	stateError
)

type sessionMsg struct {
	entries []session.Message
	err     error
}

// visualState is a thin alias to render.VisualState so the model
// owns a live visual selection cursor with all its methods.
type visualState = render.VisualState

// commentAnchor captures the target for a comment being composed.
// editing is true when re-opening the composer for an existing comment
// (the model's e key); editingIdx points into m.comments.
type commentAnchor struct {
	blockIdx   int
	charA      int
	charC      int
	editing    bool
	editingIdx int
}

type model struct {
	state state

	// Picking state
	agents []session.AgentSession
	cursor int

	// Attached state
	pane string
	src  session.Source
	hist *history.History

	// Latest-message view state. pinky always renders the most recent
	// assistant message; older messages are not displayed.
	latest session.Message
	blocks []render.Block

	viewport viewport.Model
	textarea textarea.Model

	width, height int

	streaming bool

	// Comments slice (block + inline). Lives on the model; cleared
	// when msgHash flips or attach() runs.
	comments []render.Comment

	// visual tracks the inline visual selection cursor; mode==selNone
	// when not in visual mode.
	visual visualState

	// msgHash is a short fingerprint of latest.Text; flips when the
	// assistant message content changes, which triggers a comment reset.
	msgHash string

	// includeComments toggles whether the next redirect will append
	// the comments appendix. Toggled by Ctrl+I in compose mode.
	includeComments bool

	// commentTa is the textarea used by stateCommentComposer; kept
	// separate from m.textarea (the redirect composer) so state
	// doesn't leak between modes.
	commentTa textarea.Model

	// commentAnchor captures the (block, charA, charC) for the next
	// comment. CharStart/End are derived when saving (CharStart ==
	// CharEnd == -1 means block-level).
	commentAnchor commentAnchor

	vim render.VimState

	// Error state: set when initialization or attach fails. The TUI
	// shows the message and exits on any key press.
	err error

	// help renders a one-line (short) or multi-column (full) footer
	// at the bottom of every view. ShowAll toggles between them on `?`.
	// Ponytail: a single line of keybindings at the bottom is enough
	// most of the time; the expanded view is opt-in via `?`.
	help help.Model
}

// newModel returns a picker model. Callers must either call setAgents
// (then run the picker) or call attach (skip the picker, jump to running).

// commentHash returns a short fingerprint of text. Used to detect when
// the assistant message has changed so we can clear stale comments.
// FNV-1a 32-bit, hex-encoded — collision-resistant enough for a
// per-session fingerprint (8 hex chars = 32 bits).
func commentHash(text string) string {
	h := fnv.New32a()
	h.Write([]byte(text))
	return fmt.Sprintf("%08x", h.Sum32())
}

// mostRecentComment returns the index (into m.comments) of the most
// recent comment for the given block, or (-1, false) if none.
func (m *model) mostRecentComment(blockIdx int) (int, bool) {
	for i := len(m.comments) - 1; i >= 0; i-- {
		if m.comments[i].BlockIdx == blockIdx {
			return i, true
		}
	}
	return -1, false
}

// nextCommentedBlock returns the index of the next commented block
// in the direction of delta, wrapping around at the ends. Returns
// -1 if no block has comments.
func (m *model) nextCommentedBlock(delta int) int {
	if len(m.blocks) == 0 {
		return -1
	}
	cur := render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
	if cur < 0 {
		cur = 0
	}
	has := map[int]bool{}
	for _, c := range m.comments {
		has[c.BlockIdx] = true
	}
	if len(has) == 0 {
		return -1
	}
	for step := 1; step <= len(m.blocks); step++ {
		next := cur + delta*step
		// wrap
		for next < 0 {
			next += len(m.blocks)
		}
		for next >= len(m.blocks) {
			next -= len(m.blocks)
		}
		if has[next] {
			return next
		}
	}
	return -1
}

// cursorOnScreen returns true when visual.cursor is within the viewport's
// visible rendered-line range.
func (m *model) cursorOnScreen() bool {
	top := m.viewport.YOffset
	bot := top + m.viewport.Height
	return m.visual.Cursor >= top && m.visual.Cursor <= bot
}

// scrollCursorIntoView adjusts the viewport's YOffset so the visual
// cursor sits one line inside the visible range. Called from
// handleIdleKey after j/k/}/{ in visual mode.
func (m *model) scrollCursorIntoView() {
	if m.cursorOnScreen() {
		return
	}
	off := max(m.visual.Cursor-m.viewport.Height+1, 0)
	m.viewport.SetYOffset(off)
}

func newModel() model {
	vp := viewport.New(40, 20)
	return model{
		state:    statePicking,
		viewport: vp,
		help:     help.New(),
	}
}

func (m *model) setAgents(agents []session.AgentSession) {
	m.agents = agents
	m.cursor = 0
}

// viewportSize returns (w, h) for the viewport at idle state. If the
// window size hasn't been reported yet (m.width/m.height are 0), fall
// back to safe defaults; reflow() will resize once WindowSizeMsg
// fires.
func (m *model) viewportSize() (int, int) {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height - statusHeight
	if h < 1 {
		h = 20
	}
	return w, h
}

// idle is the shared tail of attach/attachWithFile: the view setup
// (placeholder seed, textareas, comment slice, msgHash) once source +
// history are resolved. ponytail: pull the shared 25 lines out so the
// two entry points only have to source/resolve before calling.
func (m *model) idle(src session.Source, hist *history.History, pane string) {
	ta := textarea.New()
	ta.Placeholder = "redirect — Enter newline, Ctrl+S send, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)

	w, h := m.viewportSize()

	m.pane = pane
	m.src = src
	m.hist = hist
	m.viewport = viewport.New(w, h)
	m.textarea = ta
	m.state = stateIdle
	m.refreshViewport()
	m.initCommentComposer()
	m.comments = nil
	m.msgHash = commentHash(m.latest.Text)
}

// attach opens a session source + history for the given pane.
// Always seeds the viewport via refreshViewport so the placeholder
// renders immediately — same whether the user used the picker or
// --target.
func (m *model) attach(pane string) error {
	src, err := session.Open(pane)
	if err != nil {
		return err
	}
	hist, err := history.Open(pane)
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("open history: %w", err)
	}
	m.idle(src, hist, pane)
	return nil
}

// attachWithFile is the --session-file escape hatch: skip pane
// discovery and use the given JSONL path directly. No history, no
// pane → compose still works locally but the redirect has nowhere
// to go.
func (m *model) attachWithFile(path string) error {
	src, err := session.OpenFile(path)
	if err != nil {
		return err
	}
	m.idle(src, nil, "(explicit)")
	return nil
}

func (m *model) selectAgent(idx int) error {
	if idx < 0 || idx >= len(m.agents) {
		return fmt.Errorf("invalid selection")
	}
	return m.attach(m.agents[idx].PaneID)
}

func (m model) Init() tea.Cmd {
	if m.state == stateIdle {
		return pollCmd(m.src)
	}
	return nil
}

func pollCmd(src session.Source) tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		msgs, err := src.NewMessages()
		if err != nil {
			return sessionMsg{err: err}
		}
		var entries []session.Message
		for _, m := range msgs {
			role := session.RoleAssistant
			if m.Role == session.RoleUser {
				role = session.RoleUser
			}
			entries = append(entries, session.Message{Role: role, Text: m.Text})
		}
		return sessionMsg{entries: entries}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.reflow()
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case sessionMsg:
		if msg.err != nil {
			return m, pollCmd(m.src)
		}
		if len(msg.entries) == 0 {
			m.streaming = false
			return m, nil
		}
		// Find the latest assistant message in this poll. A user redirect
		// arriving after the agent's last reply should NOT replace the
		// view — we want the agent's most recent text, not the user's own.
		var last *session.Message
		for i := range msg.entries {
			if msg.entries[i].Role == session.RoleAssistant {
				last = &msg.entries[i]
			}
		}
		if last != nil {
			h := commentHash(last.Text)
			if h != m.msgHash {
				m.comments = nil
				m.msgHash = h
			}
			m.latest = *last
		}
		if m.hist != nil && last != nil {
			_ = m.hist.Append(string(last.Role), last.Text)
		}
		m.streaming = true
		m.refreshViewport()
		m.viewport.GotoBottom()
		return m, nil
	}

	var cmds []tea.Cmd
	switch m.state {
	case stateCompose:
		var taCmd tea.Cmd
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)
	case stateIdle:
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C is global across every state; `?` toggles between the
	// short help footer and the expanded (multi-column) help.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if key.Matches(msg, defaultKeyMap.Help) {
		m.help.ShowAll = !m.help.ShowAll
		m.reflow()
		m.refreshViewport()
		return m, nil
	}

	switch m.state {
	case statePicking:
		return m.handlePickerKey(msg)
	case stateIdle:
		return m.handleIdleKey(msg)
	case stateCompose:
		return m.handleComposeKey(msg)
	case stateCommentComposer:
		return m.handleCommentComposerKey(msg)
	case stateError:
		// Any key dismisses the error and quits.
		if key.Matches(msg, defaultKeyMap.QuitError) {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

// enterCompose transitions to compose mode with the textarea reset
// and focused. Shared by Ctrl+N and the `c` alias.

// sendToPane is the package-level hook for the actual tmux
// dispatch. Tests swap it to capture submit calls without a live
// tmux. ponytail: global mutable state, scoped to tests via
// t.Cleanup.
var sendToPane = inject.Send

// submitAllComments sends every accumulated comment as a single
// redirect through the existing inject pipeline and clears the
// comment slice on success. With no comments to send it is a no-op.
// On inject failure the comments are kept (so the user can retry)
// and the error is surfaced via the same "[send failed: ...]"
// placeholder the compose path uses, keeping both redirect flows
// visible in one channel.
func (m *model) submitAllComments() {
	if len(m.comments) == 0 {
		return
	}
	text := render.FormatCommentsAppendix(m.comments, m.blocks)
	if m.hist != nil {
		_ = m.hist.Append(string(session.RoleUser), text)
	}
	if err := sendToPane(m.pane, text); err != nil {
		m.latest = session.Message{
			Role: session.RoleUser, Text: "[send failed: " + err.Error() + "]",
		}
		m.refreshViewport()
		m.viewport.GotoBottom()
		return
	}
	m.comments = nil
	m.refreshViewport()
}
func (m *model) enterCompose() {
	m.state = stateCompose
	m.vim = render.VimState{}
	m.textarea.Focus()
	m.textarea.Reset()
	m.reflow()
}

// initCommentComposer creates the comment-composer textarea. Called
// from attach() / attachWithFile(). Kept separate from m.textarea so
// the redirect composer state isn't disturbed.
func (m *model) initCommentComposer() {
	ta := textarea.New()
	ta.Placeholder = "comment — Enter newline, Ctrl+S save, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)
	m.commentTa = ta
}

// enterCommentComposer seeds the comment composer with the given
// anchor and switches to stateCommentComposer.
func (m *model) enterCommentComposer(a commentAnchor) {
	m.commentAnchor = a
	m.commentTa.Reset()
	if a.editing && a.editingIdx >= 0 && a.editingIdx < len(m.comments) {
		m.commentTa.SetValue(m.comments[a.editingIdx].Text)
	}
	m.commentTa.Focus()
	m.vim = render.VimState{}
	m.visual.Mode = render.SelNone
	m.state = stateCommentComposer
	m.reflow()
}

// handleCommentComposerKey processes a keypress in stateCommentComposer.
// Ctrl+S saves the comment, Esc cancels.
func (m model) handleCommentComposerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.cancelCommentComposer()
		return m, nil
	case tea.KeyCtrlS:
		m.saveComment()
		return m, nil
	}
	var taCmd tea.Cmd
	m.commentTa, taCmd = m.commentTa.Update(msg)
	return m, taCmd
}

// cancelCommentComposer returns to idle without saving.
func (m *model) cancelCommentComposer() {
	m.commentTa.Blur()
	m.state = stateIdle
	m.reflow()
}

// saveComment commits the comment to m.comments and returns to idle.
// Anchor comes from m.commentAnchor (set by enterCommentComposer).
func (m *model) saveComment() {
	text := m.commentTa.Value()
	if text == "" {
		m.cancelCommentComposer()
		return
	}
	idx := m.commentAnchor.blockIdx
	if idx < 0 || idx >= len(m.blocks) {
		m.cancelCommentComposer()
		return
	}
	src := ""
	cs, ce := -1, -1
	if m.commentAnchor.charA >= 0 {
		cs, ce = m.commentAnchor.charA, m.commentAnchor.charC
		if cs > ce {
			cs, ce = ce, cs
		}
		bsrc := m.blocks[idx].Source
		if cs >= len(bsrc) {
			cs = len(bsrc) - 1
		}
		if ce > len(bsrc) {
			ce = len(bsrc)
		}
		if cs < 0 {
			cs = 0
		}
		src = bsrc[cs:ce]
	}
	c := render.Comment{
		Kind:      m.blocks[idx].Kind,
		BlockIdx:  idx,
		CharStart: cs,
		CharEnd:   ce,
		Source:    src,
		Text:      text,
		CreatedAt: time.Now(),
	}
	if m.commentAnchor.editing {
		if i := m.commentAnchor.editingIdx; i >= 0 && i < len(m.comments) {
			c.CreatedAt = m.comments[i].CreatedAt // preserve original timestamp on edit
			m.comments[i] = c
		}
	} else {
		m.comments = append(m.comments, c)
	}
	m.commentTa.Blur()
	m.state = stateIdle
	m.reflow()
	m.refreshViewport()
}

func (m model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, defaultKeyMap.Up):
		m.moveCursor(-1)
	case key.Matches(msg, defaultKeyMap.Down):
		m.moveCursor(+1)
	case key.Matches(msg, defaultKeyMap.Pick):
		if err := m.selectAgent(m.cursor); err != nil {
			m.state = stateError
			m.err = err
			return m, nil
		}
		// attach() already seeded the viewport with the placeholder
		// (or the current latest). Just kick off polling.
		return m, pollCmd(m.src)
	case key.Matches(msg, defaultKeyMap.QuitPick):
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) moveCursor(delta int) {
	n := len(m.agents)
	if n == 0 {
		return
	}
	m.cursor = (m.cursor + delta + n) % n
}

func (m model) handleIdleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Visual mode must claim keys before the idle-view bindings
	// below, otherwise j/k/}/{ get eaten as viewport line moves /
	// block jumps and the visual cursor never moves. Pressing Esc
	// (or any other unused rune) inside handleVisualKey falls through
	// to its switch arm that exits visual mode.
	if m.visual.Mode == render.SelLine {
		return m.handleVisualKey(msg)
	}

	switch {
	case key.Matches(msg, defaultKeyMap.Compose):
		m.enterCompose()
		return m, nil
	case key.Matches(msg, defaultKeyMap.Refresh):
		return m, pollCmd(m.src)
	case key.Matches(msg, defaultKeyMap.LineUp):
		m.viewport.LineUp(1)
		return m, nil
	case key.Matches(msg, defaultKeyMap.LineDown):
		m.viewport.LineDown(1)
		return m, nil
	case key.Matches(msg, defaultKeyMap.PrevBlock):
		m.applyAction(render.VimPrevBlock)
		return m, nil
	case key.Matches(msg, defaultKeyMap.NextBlock):
		m.applyAction(render.VimNextBlock)
		return m, nil
	case key.Matches(msg, defaultKeyMap.BottomLine):
		m.applyAction(render.VimGotoBottom)
		return m, nil
	case key.Matches(msg, defaultKeyMap.QuitIdle):
		return m, tea.Quit
	}

	switch {
	case key.Matches(msg, defaultKeyMap.Mark):
		// m → comment composer for the current block (block-level).
		idx := render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
		if idx < 0 {
			return m, nil
		}
		m.enterCommentComposer(commentAnchor{blockIdx: idx})
		return m, nil
	case key.Matches(msg, defaultKeyMap.Visual):
		// V → enter visual line mode.
		idx := render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
		startLine := 0
		if idx >= 0 {
			startLine = m.blocks[idx].StartLine
		}
		m.visual.Enter(startLine, idx, m.blocks)
		return m, nil
	case key.Matches(msg, defaultKeyMap.EditComment):
		idx := render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
		if i, ok := m.mostRecentComment(idx); ok {
			c := m.comments[i]
			m.enterCommentComposer(commentAnchor{
				blockIdx:   c.BlockIdx,
				charA:      c.CharStart,
				charC:      c.CharEnd,
				editing:    true,
				editingIdx: i,
			})
		}
		return m, nil
	case key.Matches(msg, defaultKeyMap.DeleteComment):
		idx := render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
		if i, ok := m.mostRecentComment(idx); ok {
			m.comments = append(m.comments[:i], m.comments[i+1:]...)
			m.refreshViewport()
		}
		return m, nil
	case key.Matches(msg, defaultKeyMap.NextComment):
		if next := m.nextCommentedBlock(+1); next >= 0 {
			m.viewport.SetYOffset(m.blocks[next].StartLine)
			m.refreshViewport()
		}
		return m, nil
	case key.Matches(msg, defaultKeyMap.PrevComment):
		if prev := m.nextCommentedBlock(-1); prev >= 0 {
			m.viewport.SetYOffset(m.blocks[prev].StartLine)
			m.refreshViewport()
		}
		return m, nil
	case key.Matches(msg, defaultKeyMap.SubmitComments):
		// One-shot submit: send every accumulated comment as a
		// single redirect, bypassing the compose textarea flow.
		m.submitAllComments()
		return m, nil
	}

	// Vim two-key sequences (gg, ]], [[): match by the first key and
	// let the state machine complete the pair.
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		action := m.vim.Handle(msg.Runes[0], time.Now())
		m.applyAction(action)
		return m, nil
	}

	// Forward unhandled to viewport.
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

// handleVisualKey processes a key while in visual mode. Movement keys
// (j/k/}/{) are routed to the visual state machine; c opens the
// composer; Esc exits. Updates the viewport so the cursor stays
// visible when it scrolls off-screen.
func (m model) handleVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only single-rune keys are meaningful in visual mode.
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		if msg.Type == tea.KeyEsc {
			m.visual.Mode = render.SelNone
			return m, nil
		}
		return m, nil
	}
	totalLines := strings.Count(m.viewport.View(), "\n")
	action := m.visual.Handle(msg.Runes[0], totalLines, m.blocks)
	switch action {
	case render.VisualEnter:
		// Should not happen (we're already in visual mode).
	case render.VisualLineDown, render.VisualLineUp, render.VisualNextBlock, render.VisualPrevBlock:
		m.scrollCursorIntoView()
		// Borders are baked into the viewport content at refresh
		// time, so moving the visual cursor to a different block
		// requires re-rendering — otherwise the highlight would
		// stay on whatever block viewport.YOffset happened to be on.
		m.refreshViewport()
	case render.VisualComposer:
		idx := m.visual.CurBlock
		a := commentAnchor{blockIdx: idx, charA: m.visual.CharA, charC: m.visual.CharC}
		m.enterCommentComposer(a)
	case render.VisualExit:
		m.visual.Mode = render.SelNone
		// Same reason as above: the highlight must return to the
		// viewport-driven block on exit, not stay on the visual one.
		m.refreshViewport()
	}
	return m, nil
}

func (m model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, defaultKeyMap.IncludeComments):
		m.includeComments = !m.includeComments
		return m, nil
	case key.Matches(msg, defaultKeyMap.Send):
		text := m.textarea.Value()
		if text == "" {
			return m, nil
		}
		if m.includeComments && len(m.comments) > 0 {
			text += render.FormatCommentsAppendix(m.comments, m.blocks)
		}
		if m.hist != nil {
			_ = m.hist.Append(string(session.RoleUser), text)
		}
		if err := sendToPane(m.pane, text); err != nil {
			// ponytail: surface a transient placeholder rather than
			// permanently mutating view state; revisit if it bites.
			m.latest = session.Message{
				Role: session.RoleUser, Text: "[send failed: " + err.Error() + "]",
			}
			m.refreshViewport()
			m.viewport.GotoBottom()
			return m, nil
		}
		m.state = stateIdle
		m.textarea.Blur()
		m.textarea.Reset()
		m.reflow()
		return m, nil
	case key.Matches(msg, defaultKeyMap.Cancel):
		m.state = stateIdle
		m.textarea.Blur()
		m.textarea.Reset()
		m.reflow()
		return m, nil
	}
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, taCmd
}

func (m *model) applyAction(a render.VimAction) {
	switch a {
	case render.VimLineDown:
		m.viewport.LineDown(1)
	case render.VimLineUp:
		m.viewport.LineUp(1)
	case render.VimNextBlock:
		if y := render.JumpBlock(m.blocks, render.CurrentBlockIdx(m.blocks, m.viewport.YOffset), +1); y >= 0 {
			m.viewport.SetYOffset(y)
		}
	case render.VimPrevBlock:
		if y := render.JumpBlock(m.blocks, render.CurrentBlockIdx(m.blocks, m.viewport.YOffset), -1); y >= 0 {
			m.viewport.SetYOffset(y)
		}
	case render.VimNextHeading:
		if y := render.JumpHeading(m.blocks, render.CurrentBlockIdx(m.blocks, m.viewport.YOffset), +1); y >= 0 {
			m.viewport.SetYOffset(y)
		}
	case render.VimPrevHeading:
		if y := render.JumpHeading(m.blocks, render.CurrentBlockIdx(m.blocks, m.viewport.YOffset), -1); y >= 0 {
			m.viewport.SetYOffset(y)
		}
	case render.VimGotoTop:
		m.viewport.GotoTop()
	case render.VimGotoBottom:
		m.viewport.GotoBottom()
	}
}

func (m *model) reflow() {
	if m.height == 0 {
		return
	}
	vpHeight := m.height - statusHeight - m.helpHeight()
	if m.state == stateCompose || m.state == stateCommentComposer {
		vpHeight -= composeHeight + 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	if m.state == stateCompose || m.state == stateIdle {
		m.textarea.SetWidth(m.width)
	}
}

// helpHeight reports the number of terminal lines the help footer
// will need. Short help is 1 line; full help grows with the largest
// group in the current state's FullHelp(). Computed from the live
// bindings so adding a new key to a group automatically extends the
// reserved space.
func (m *model) helpHeight() int {
	if !m.help.ShowAll {
		return 1
	}
	maxN := 0
	for _, g := range m.FullHelp() {
		if len(g) > maxN {
			maxN = len(g)
		}
	}
	if maxN < 1 {
		return 1
	}
	return maxN
}

// ShortHelp returns the keybindings rendered in the one-line help
// footer. Curated per state so the most useful keys (not all of them)
// stay visible without truncation. `?` is included where expanding
// to full help is meaningful.
func (m model) ShortHelp() []key.Binding {
	switch m.state {
	case statePicking:
		return []key.Binding{
			defaultKeyMap.Up, defaultKeyMap.Down,
			defaultKeyMap.Pick, defaultKeyMap.Help,
		}
	case stateIdle:
		return []key.Binding{
			defaultKeyMap.Compose, defaultKeyMap.SubmitComments,
			defaultKeyMap.Mark, defaultKeyMap.Help,
		}
	case stateCompose:
		return []key.Binding{
			defaultKeyMap.Send, defaultKeyMap.IncludeComments,
			defaultKeyMap.Cancel, defaultKeyMap.Help,
		}
	case stateCommentComposer:
		return []key.Binding{
			defaultKeyMap.Send, defaultKeyMap.Cancel,
		}
	case stateError:
		return []key.Binding{defaultKeyMap.QuitError}
	}
	return nil
}

// FullHelp returns every keybinding for the current state, grouped
// for the expanded (multi-column) help view. Reuses the same group
// ordering the short view picks from, so adding a binding to a group
// here also surfaces it in the long view.
func (m model) FullHelp() [][]key.Binding {
	return helpGroupsForState(m.state)
}

// helpGroups returns the per-state help binding groups for the
// expanded (full) help view. The model passes this list straight
// through FullHelp() — no per-group wrapper struct is needed since
// the column title was unused (kept only as "in case we want labels
// later", per the original comment).
func helpGroupsForState(s state) [][]key.Binding {
	switch s {
	case statePicking:
		return [][]key.Binding{
			{defaultKeyMap.Up, defaultKeyMap.Down},
			{defaultKeyMap.Pick},
			{defaultKeyMap.QuitPick, defaultKeyMap.Help},
		}
	case stateIdle:
		return [][]key.Binding{
			{defaultKeyMap.LineDown, defaultKeyMap.LineUp, defaultKeyMap.NextBlock, defaultKeyMap.PrevBlock, defaultKeyMap.BottomLine},
			{defaultKeyMap.Mark, defaultKeyMap.Visual},
			{defaultKeyMap.VisualDown, defaultKeyMap.VisualUp, defaultKeyMap.VisualNextBlock, defaultKeyMap.VisualPrevBlock, defaultKeyMap.VisualOpen, defaultKeyMap.VisualExit},
			{defaultKeyMap.EditComment, defaultKeyMap.DeleteComment, defaultKeyMap.NextComment, defaultKeyMap.PrevComment, defaultKeyMap.SubmitComments},
			{defaultKeyMap.Compose},
			{defaultKeyMap.Refresh},
			{defaultKeyMap.QuitIdle, defaultKeyMap.Help},
		}
	case stateCompose:
		return [][]key.Binding{
			{defaultKeyMap.Send},
			{defaultKeyMap.Newline},
			{defaultKeyMap.IncludeComments},
			{defaultKeyMap.Cancel, defaultKeyMap.Help},
		}
	case stateCommentComposer:
		return [][]key.Binding{
			{defaultKeyMap.Send},
			{defaultKeyMap.Cancel},
		}
	case stateError:
		return [][]key.Binding{
			{defaultKeyMap.QuitError, defaultKeyMap.Help},
		}
	}
	return nil
}

// borderStyle is the lipgloss style for the current-block indicator.
// Same color family as the picker header (212 accent).
var borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

// placeholderStyle is the dim style for the empty-state line.
var placeholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

// statusBarStyle paints the bottom status line as a full-width bar so
// pinky visually anchors to the terminal edges instead of leaving
// whitespace on the right.
var statusBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("241")).
	Background(lipgloss.Color("236")).
	Padding(0, 1)

// visualModeStyle is the bright accent used for the [VISUAL] chip in
// the status line when visual mode is active. Picked so it stands
// out against the dim status-bar background and is unambiguous about
// the mode (visual mode has no other persistent on-screen indicator).
var visualModeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("232")).
	Background(lipgloss.Color("212")).
	Bold(true).
	Padding(0, 1)

func (m *model) refreshViewport() {
	if m.latest.Text == "" {
		m.blocks = nil
		m.viewport.SetContent(placeholderStyle.Render(placeholderText))
		return
	}
	// Ponytail: RenderMessageWithComments is a strict superset; for
	// zero comments it returns the same output as RenderMessage.
	rendered, blocks := render.RenderMessageWithComments(m.latest.Text, m.width, m.comments)
	if rendered == "" {
		// Renderer rejected this width (very narrow terminal): plain-text fallback.
		m.viewport.SetContent(m.latest.Text)
		return
	}
	m.blocks = blocks
	m.viewport.SetContent(m.injectBorder(rendered))
}

// focusedBlockIdx returns the block index that should carry the
// highlight. In visual mode the cursor is independent of the
// viewport's scroll position, so we follow the visual cursor; out of
// visual mode we fall back to the viewport-driven YOffset.
func (m *model) focusedBlockIdx() int {
	if m.visual.Mode == render.SelLine && m.visual.CurBlock >= 0 {
		return m.visual.CurBlock
	}
	return render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
}

// injectBorder wraps the rendered output with heavy horizontal border
// lines above and below the currently-focused block, and adds a left
// vertical bar to each line within the block. The left bar replaces
// glamour's 2-char dark-preset margin (visible width unchanged).
//
// The viewport content is rewritten line-by-line so the border sits at
// exact line indices and the left bar covers exactly the focused
// block's StartLine..EndLine range.
func (m *model) injectBorder(rendered string) string {
	idx := m.focusedBlockIdx()
	if idx < 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	heavyBorder := borderStyle.Render(strings.Repeat("━", m.width))
	leftBar := borderStyle.Render("┃")
	var b strings.Builder
	for i, line := range lines {
		if i == m.blocks[idx].StartLine {
			b.WriteString(heavyBorder)
			b.WriteByte('\n')
		}
		if i >= m.blocks[idx].StartLine && i <= m.blocks[idx].EndLine {
			// Replace glamour's leading 2-space margin with the left
			// vertical bar + space. If the line is already prefixed
			// by a comment gutter marker (▸/•), keep that — the
			// gutter conveys its own meaning.
			if !render.HasCommentGutter(line) {
				line = leftBar + " " + render.TrimLeadingVisible(line, 2)
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if i == m.blocks[idx].EndLine {
			b.WriteString(heavyBorder)
			b.WriteByte('\n')
		}
	}
	return b.String()
}


func (m model) View() string {
	// Help footer is rendered for every state; `?` flips it between
	// the one-line short view and the multi-column full view.
	helpView := m.help.View(m)
	switch m.state {
	case statePicking:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.pickerView(),
			helpView,
		))
	case stateCompose:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.textarea.View(),
			helpView,
			m.statusLine(),
		))
	case stateCommentComposer:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.commentTa.View(),
			helpView,
			m.statusLine(),
		))
	case stateError:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.errorView(),
			helpView,
		))
	default:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			helpView,
			m.statusLine(),
		))
	}
}

// fillWidth pads every line of s to m.width so the rendered output
// spans the full terminal. Before WindowSizeMsg fires, m.width is 0
// and we return s unchanged.
func (m model) fillWidth(s string) string {
	if m.width <= 0 {
		return s
	}
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
		if pad := m.width - render.VisibleWidth(line); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	return b.String()
}

// errorView renders a single centered error message. The TUI shows
// this when attach or session discovery fails; any key press quits.
// The help footer carries the "press any key" hint.
func (m model) errorView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	return headerStyle.Render("pinky: error") + "\n\n" +
		bodyStyle.Render(m.err.Error())
}

func (m model) pickerView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	var b strings.Builder
	b.WriteString(headerStyle.Render("pinky: pick an agent session"))
	b.WriteByte('\n')
	b.WriteString(dimStyle.Render("  session          pane    agent"))
	b.WriteByte('\n')
	for i, a := range m.agents {
		marker := "  "
		style := normalStyle
		if i == m.cursor {
			marker = "▶ "
			style = selectedStyle
		}
		line := fmt.Sprintf("%s%-16s %-6s  %s",
			marker,
			fmt.Sprintf("%s:%s.%s", a.Session, a.Window, a.Pane),
			a.PaneID,
			a.Agent,
		)
		b.WriteString(style.Render(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) statusLine() string {
	dot := "·"
	if m.streaming {
		dot = "●"
	}
	text := fmt.Sprintf("%s %s", dot, m.pane)
	if m.state == stateCompose && len(m.comments) > 0 {
		flag := "OFF"
		if m.includeComments {
			flag = "ON"
		}
		text += fmt.Sprintf("  [I] include %d comments — %s", len(m.comments), flag)
	}
	// Visual mode has no other persistent on-screen marker (the
	// borders follow viewport.YOffset, not the visual cursor), so
	// surface it here as the only signal that V did something.
	rendered := statusBarStyle.Render(text)
	if m.visual.Mode == render.SelLine {
		rendered += visualModeStyle.Render(" VISUAL ")
	}
	if m.width <= 0 {
		return rendered
	}
	if pad := m.width - render.VisibleWidth(rendered); pad > 0 {
		rendered += strings.Repeat(" ", pad)
	}
	return rendered
}
