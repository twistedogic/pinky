package main

import (
	"fmt"
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
	stateNav
	stateCompose
	stateCommentComposer
	stateError
)

type sessionMsg struct {
	entries []session.Message
	err     error
}

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
	agents     []session.AgentSession
	pickCursor int

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
	// when the latest message text changes or attach() runs.
	comments []render.Comment

	// cursor is the single nav pointer. BlockIdx into m.blocks;
	// CharPos is a byte offset into blocks[BlockIdx].Source.
	cursor render.NavCursor
	// selection tracks the inline visual selection range. Valid only
	// when nav.Visual == render.NavLine.
	selection render.NavSelection
	// nav is the nav state machine state.
	nav render.NavState

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



// cursorOnScreen returns true when the cursor's rendered line is
// within the viewport's visible range.
func (m *model) cursorOnScreen() bool {
	if len(m.blocks) == 0 {
		return true
	}
	line := render.NavLineIndex(m.blocks, m.cursor)
	top := m.viewport.YOffset
	bot := top + m.viewport.Height
	return line >= top && line <= bot
}

// scrollCursorIntoView adjusts the viewport's YOffset so the cursor
// sits one line inside the visible range. Called from handleNavKey
// after every motion. Per design D9 the viewport follows the cursor
// with a 1-line cushion (not vim's scrolloff=5; pinky's viewport is
// ~16 rows tall and a 5-line cushion would feel jumpy).
func (m *model) scrollCursorIntoView() {
	if len(m.blocks) == 0 {
		return
	}
	if m.cursorOnScreen() {
		return
	}
	target := render.NavLineIndex(m.blocks, m.cursor)
	off := max(target-m.viewport.Height+1, 0)
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
	m.pickCursor = 0
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
// (placeholder seed, textareas, comment slice) once source +
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
	m.state = stateNav
	m.refreshViewport()
	m.initCommentComposer()
	m.comments = nil
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
	if m.state == stateNav {
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
		return sessionMsg{entries: msgs}
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
			if last.Text != m.latest.Text {
				m.comments = nil
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
	case stateNav:
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
	case stateNav:
		return m.handleNavKey(msg)
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
	m.nav = render.NavState{}
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
	m.nav = render.NavState{}
	m.state = stateCommentComposer
	m.reflow()
}

// handleCommentComposerKey processes a keypress in stateCommentComposer.
// Enter saves the new comment and dispatches every accumulated comment
// in one batch via submitAllComments; Esc cancels.
func (m model) handleCommentComposerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.cancelCommentComposer()
		return m, nil
	case tea.KeyEnter:
		m.saveComment()
		m.submitAllComments()
		return m, nil
	}
	var taCmd tea.Cmd
	m.commentTa, taCmd = m.commentTa.Update(msg)
	return m, taCmd
}

// cancelCommentComposer returns to idle without saving.
func (m *model) cancelCommentComposer() {
	m.commentTa.Blur()
	m.state = stateNav
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
	m.state = stateNav
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
		if err := m.selectAgent(m.pickCursor); err != nil {
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
	m.pickCursor = (m.pickCursor + delta + n) % n
}

func (m model) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc is a special key but is part of the nav surface (visual
	// exit). Route it through the SM so single source of truth.
	if msg.Type == tea.KeyEsc {
		action := render.NavHandle(0x1b, &m.nav, &m.cursor, &m.selection, m.blocks)
		if action == render.ActionExitVisual {
			m.refreshViewport()
		}
		return m, nil
	}
	// Single-rune keys go through the nav state machine. Special
	// keys (arrows, PageUp/Down, Home/End) forward to the viewport.
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		action := render.NavHandle(msg.Runes[0], &m.nav, &m.cursor, &m.selection, m.blocks)
		switch action {
		case render.ActionNone:
			// unrecognised rune — forward to viewport (so keys like '/'
			// for find, etc., could still work in future).
		case render.ActionBlockDown, render.ActionBlockUp,
			render.ActionRuneLeft, render.ActionRuneRight:
			m.scrollCursorIntoView()
			m.refreshViewport()
		case render.ActionEnterVisual:
			m.refreshViewport()
		case render.ActionExitVisual:
			m.refreshViewport()
		case render.ActionComment:
			a := m.buildCommentAnchor()
			m.enterCommentComposer(a)
		case render.ActionSend:
			m.handleSend()
		case render.ActionRefresh:
			return m, pollCmd(m.src)
		case render.ActionQuit:
			return m, tea.Quit
		case render.ActionCompose:
			m.enterCompose()
		case render.ActionHelp:
			// handled by handleKey before we get here
		}
		return m, nil
	}
	// Forward unhandled special keys to viewport (arrows, pgup/dn, etc.)
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

// buildCommentAnchor assembles the (block, charA, charC) for the next
// comment. With an active selection, anchor is the selection range;
// otherwise anchor covers the whole block at the cursor.
func (m *model) buildCommentAnchor() commentAnchor {
	if m.nav.Visual == render.NavLine {
		idx := m.selection.BlockIdx
		if idx < 0 || idx >= len(m.blocks) {
			idx = m.cursor.BlockIdx
		}
		a, c := m.selection.CharA, m.selection.CharC
		if a > c {
			a, c = c, a
		}
		return commentAnchor{blockIdx: idx, charA: a, charC: c}
	}
	idx := m.cursor.BlockIdx
	if idx < 0 || idx >= len(m.blocks) {
		return commentAnchor{blockIdx: -1, charA: -1, charC: -1}
	}
	src := m.blocks[idx].Source
	return commentAnchor{blockIdx: idx, charA: 0, charC: len(src)}
}

// handleSend is the universal `s` dispatch. In nav it batch-sends
// the accumulated comments (no-op if empty). In compose it sends the
// textarea content with the optional comments appendix. Called from
// handleNavKey and (via the same ActionSend) from handleComposeKey.
func (m *model) handleSend() {
	switch m.state {
	case stateNav:
		m.submitAllComments()
	case stateCompose:
		text := m.textarea.Value()
		if text == "" {
			return
		}
		if m.includeComments && len(m.comments) > 0 {
			text += render.FormatCommentsAppendix(m.comments, m.blocks)
		}
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
		m.state = stateNav
		m.textarea.Blur()
		m.textarea.Reset()
		m.reflow()
	}
}

func (m model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, defaultKeyMap.IncludeComments):
		m.includeComments = !m.includeComments
		return m, nil
	case key.Matches(msg, defaultKeyMap.Cancel):
		m.state = stateNav
		m.textarea.Blur()
		m.textarea.Reset()
		m.reflow()
		return m, nil
	}
	// `s` is the universal send — dispatch via NavHandle so the
	// single-rune path is the source of truth (matches D3 / D4).
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 's' {
		m.handleSend()
		return m, nil
	}
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	return m, taCmd
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
	if m.state == stateCompose || m.state == stateNav {
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
	case stateNav:
		return []key.Binding{
			defaultKeyMap.NavGroup,
			defaultKeyMap.Help,
		}
	case stateCompose:
		return []key.Binding{
			defaultKeyMap.IncludeComments,
			defaultKeyMap.Cancel, defaultKeyMap.Help,
		}
	case stateCommentComposer:
		return []key.Binding{
			defaultKeyMap.SaveComment,
			defaultKeyMap.Cancel,
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
	case stateNav:
		return [][]key.Binding{
			{defaultKeyMap.NavGroup},
			{defaultKeyMap.Help},
		}
	case stateCompose:
		return [][]key.Binding{
			{defaultKeyMap.Newline},
			{defaultKeyMap.IncludeComments},
			{defaultKeyMap.Cancel, defaultKeyMap.Help},
		}
	case stateCommentComposer:
		return [][]key.Binding{
			{defaultKeyMap.SaveComment},
			{defaultKeyMap.Cancel},
		}
	case stateError:
		return [][]key.Binding{
			{defaultKeyMap.QuitError, defaultKeyMap.Help},
		}
	}
	return nil
}

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
// Cyan (51) matches the gutter focus indicator; magenta (212) is
// reserved for the picker header accent.
var visualModeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("232")).
	Background(lipgloss.Color("51")).
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
	m.viewport.SetContent(m.injectGutter(rendered))
}

// focusedBlockIdx returns the block index that should carry the
// highlight. With the single-cursor model, the cursor's blockIdx
// drives the highlight in all states (visual or not).
func (m *model) focusedBlockIdx() int {
	if m.cursor.BlockIdx >= 0 && m.cursor.BlockIdx < len(m.blocks) {
		return m.cursor.BlockIdx
	}
	return render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
}

// injectGutter delegates to render.InjectGutter with the model's
// focused-block decision. Thin wrapper so the gutter logic lives in
// one place (render package) where it can be tested without a model.
func (m *model) injectGutter(rendered string) string {
	return render.InjectGutter(rendered, m.blocks, m.focusedBlockIdx())
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
		if i == m.pickCursor {
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
	if m.nav.Visual == render.NavLine {
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
