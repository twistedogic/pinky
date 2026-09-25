package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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
	"github.com/charmbracelet/bubbles/filepicker"
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
	stateFileNav
	stateFileView
	stateError
)

type sessionMsg struct {
	entries []session.Message
	err     error
}

// commentAnchor captures the target for a comment being composed.
// For block-kind anchors blockIdx, charA, charC are valid (with
// charA < 0 meaning a whole-block comment). For file-kind anchors
// filePath, lineStart, lineEnd are valid; charA/charC carry an
// optional inline byte range (charA < 0 means a line-range
// comment).
type commentAnchor struct {
	kind      render.CommentKind
	blockIdx  int
	charA     int
	charC     int
	filePath  string
	lineStart int
	lineEnd   int
}

// tab identifies the active top-level view.
type tab int

const (
	tabMessage tab = iota
	tabFiles
)

// fileSelection is the file viewer's visual selection range. All
// values are 1-based; LineA/LineC index source lines, CharA/CharC
// are byte offsets into the corresponding line.
type fileSelection struct {
	LineA, CharA int
	LineC, CharC int
	Active       bool
}

// fileViewer holds the state for stateFileView: the raw content,
// the line index (parallel to lines, drives gutter flags), the
// cursor (line + char), an optional visual selection, and the
// viewport that scrolls the rendered content.
type fileViewer struct {
	path      string
	content   string
	lines     []string // raw lines, no trailing newline
	cursor    int      // 1-based line index
	visual    fileSelection
	lineIndex []render.FileLine // parallel to lines
	viewport  viewport.Model    // ponytail: scrollable view of the rendered file
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
	latest         session.Message
	blocks         []render.Block
	wrappedToSrc   []int // viewport yOffset → source-line index, for nav
	sourceToFirst  []int // source-line index → first wrapped yOffset, for nav

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

	// Tab state. m.tab records which top-level view is active;
	// m.fileReturn remembers the file-review sub-state the user
	// left, so Tab round-trips restore it.
	tab        tab
	fileReturn state

	// File review tab state. filePicker is the bubbles directory
	// browser rooted at fileRoot; selecting a file moves the model
	// into stateFileView.
	filePicker filepicker.Model
	fileRoot      string

	// fileViewer is the state for stateFileView.
	fileViewer fileViewer

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
	srcLine := render.NavLineIndex(m.blocks, m.cursor)
	// Compare in source-line space: both the cursor's source line
	// and the viewport's source line (translating YOffset back via
	// the wrapped-line map).
	wrapTop := m.viewport.YOffset
	wrapBot := wrapTop + m.viewport.Height
	srcTop := m.wrappedYOffsetToSource(wrapTop)
	srcBot := m.wrappedYOffsetToSource(wrapBot - 1)
	return srcLine >= srcTop && srcLine <= srcBot
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
	srcLine := render.NavLineIndex(m.blocks, m.cursor)
	// Translate the target source line into wrapped-YOffset space
	// so the viewport actually lands on that markdown line.
	target := m.sourceYOffset(srcLine)
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
// idle is the shared tail of attach/attachWithFile: the view setup
// (placeholder seed, textareas, comment slice) once source +
// history are resolved. Source and history come from the caller
// (session.Open + history.Open for the picker path; session.OpenFile
// only for --session-file); everything below is identical.
func (m *model) idle(src session.Source, hist *history.History, pane string) {
	ta := textarea.New()
	ta.Placeholder = "redirect — Enter newline, Ctrl+S send, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)

	cta := textarea.New()
	cta.Placeholder = "comment — Esc cancel"
	cta.ShowLineNumbers = false
	cta.CharLimit = 0
	cta.SetHeight(1)

	w, h := m.viewportSize()

	m.pane = pane
	m.src = src
	m.hist = hist
	m.viewport = viewport.New(w, h)
	m.textarea = ta
	m.commentTa = cta
	m.state = stateNav
	m.refreshViewport()
	m.comments = nil

	// Best-effort: record the pane's cwd for the file review tab.
	// Empty on failure (--session-file path, or no tmux server).
	if pane != "" && pane != "(explicit)" {
		if cwd, err := session.PaneCwd(pane); err == nil {
			m.fileRoot = cwd
		}
	}
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
		// reflow already re-anchors the file cursor; refresh the
		// file viewport's cached content so it matches the new size.
		if m.state == stateFileView && m.fileViewer.path != "" {
			m.refreshFileView()
		}
		// ponytail: let the bubbles filepicker size itself.
		m.filePicker.SetHeight(msg.Height - 5)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case sessionMsg:
		// ponytail: every poll result must re-arm the next Tick,
		// not just the error path. Without this the first poll
		// delivers historical entries and polling stops — pinky
		// never picks up new codex messages after attach.
		if msg.err != nil {
			return m, pollCmd(m.src)
		}
		if len(msg.entries) == 0 {
			m.streaming = false
			return m, pollCmd(m.src)
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
			if m.hist != nil {
				_ = m.hist.Append(string(last.Role), last.Text)
			}
			m.streaming = true
			m.refreshViewport()
			// ponytail: Bubble Tea's SetContent preserves YOffset
			// across calls. Pin to top on every commit so the
			// first sentence of the latest message is always
			// visible — the viewport's own scroll state is the
			// single source of truth, no custom buffer needed.
			m.viewport.GotoTop()
		}
		return m, pollCmd(m.src)
	}

	// ponytail: route any unhandled msg to the bubbles filepicker
	// when we're on the file tab — its Init() emits a readDirMsg
	// on entry, and Enter/Back emit follow-up reads. Anything the
	// picker doesn't recognise is silently dropped.
	if m.state == stateFileNav || m.state == stateFileView {
		updated, cmd := m.filePicker.Update(msg)
		m.filePicker = updated
		if cmd != nil {
			return m, cmd
		}
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

	// Tab toggles the message / file-review tab. Active in every
	// state except stateCommentComposer (let the textarea eat it),
	// statePicking (no agent yet), and stateError.
	if key.Matches(msg, defaultKeyMap.Tab) {
		switch m.state {
		case stateCommentComposer, statePicking, stateError:
			// swallowed
		default:
			return m, m.toggleTab()
		}
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
	case stateFileNav:
		return m.handleFileNavKey(msg)
	case stateFileView:
		return m.handleFileViewKey(msg)
	case stateError:
		// Any key dismisses the error and quits.
		if key.Matches(msg, defaultKeyMap.QuitError) {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

// toggleTab switches between the message tab and the file review
// tab, remembering / restoring the sub-state so Tab round-trips are
// cheap.
func (m *model) toggleTab() tea.Cmd {
	switch m.tab {
	case tabMessage:
		// Remember the message sub-state so Tab back returns to it.
		m.fileReturn = m.state
		m.tab = tabFiles
		if m.filePicker.CurrentDirectory == "" {
			m.enterFileNav()
			return m.filePicker.Init()
		}
		m.state = stateFileNav
		m.reflow()
	case tabFiles:
		m.tab = tabMessage
		restore := m.fileReturn
		if restore != stateNav && restore != stateCompose {
			restore = stateNav
		}
		m.state = restore
		m.fileViewer.visual.Active = false
		m.reflow()
		m.refreshViewport()
	}
	return nil
}

// enterFileNav runs the workspace walker against m.fileRoot and
// transitions to stateFileNav.
func (m *model) enterFileNav() {
	if m.fileRoot == "" {
		m.state = stateNav
		m.latest = session.Message{
			Role: session.RoleUser,
			Text: "[no workspace: pane cwd unavailable]",
		}
		m.refreshViewport()
		return
	}
	// ponytail: use the bubbles filepicker rooted at fileRoot.
	// It manages its own directory traversal, selection, and
	// back/forward navigation. pinky only watches filepicker.Path
	// to know when the user has chosen a file.
	fp := filepicker.New()
	fp.CurrentDirectory = m.fileRoot
	fp.DirAllowed = true
	fp.FileAllowed = true
	fp.ShowHidden = false
	m.filePicker = fp
	m.state = stateFileNav
	m.reflow()
}

// enterCompose transitions to compose mode with the textarea reset
// and focused. Shared by Ctrl+N and the `c` alias.

// sendToPane is the package-level hook for the actual tmux
// dispatch. Tests swap it to capture submit calls without a live
// tmux. ponytail: global mutable state, scoped to tests via
// t.Cleanup.
var sendToPane = inject.Send

// dispatch sends text to the agent pane, recording to history on
// success and surfacing a "[send failed: ...]" placeholder in the
// main view on failure. Returns true on success.
func (m *model) dispatch(text string) bool {
	if m.hist != nil {
		_ = m.hist.Append(string(session.RoleUser), text)
	}
	if err := sendToPane(m.pane, text); err != nil {
		m.latest = session.Message{
			Role: session.RoleUser, Text: "[send failed: " + err.Error() + "]",
		}
		m.refreshViewport()
		m.viewport.GotoBottom()
		return false
	}
	return true
}

// submitAllComments sends every accumulated comment as a single
// redirect through the existing inject pipeline and clears the
// comment slice on success. With no comments to send it is a no-op.
// On inject failure the comments are kept (so the user can retry)
// and the error is surfaced via dispatch.
func (m *model) submitAllComments() {
	if len(m.comments) == 0 {
		return
	}
	if !m.dispatch(render.FormatCommentsAppendix(m.comments, m.blocks)) {
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

// enterCommentComposer seeds the comment composer with the given
// anchor and switches to stateCommentComposer.
func (m *model) enterCommentComposer(a commentAnchor) {
	m.commentAnchor = a
	m.commentTa.Reset()
	m.commentTa.Focus()
	m.nav = render.NavState{}
	m.state = stateCommentComposer
	m.reflow()
}

// handleCommentComposerKey processes a keypress in stateCommentComposer.
// Enter saves the new comment and returns to nav without sending; the
// accumulated batch flushes only via `s` in stateNav. Esc cancels.
func (m model) handleCommentComposerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.cancelCommentComposer()
		return m, nil
	case tea.KeyEnter:
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
	a := m.commentAnchor
	switch a.kind {
	case render.CommentFile:
		m.saveFileComment(a, text)
	default:
		m.saveBlockComment(a, text)
	}
	m.commentTa.Blur()
}

// saveBlockComment is the existing block-kind path, factored out
// of saveComment so saveComment can route by anchor kind.
func (m *model) saveBlockComment(a commentAnchor, text string) {
	idx := a.blockIdx
	if idx < 0 || idx >= len(m.blocks) {
		m.cancelCommentComposer()
		return
	}
	src := ""
	cs, ce := -1, -1
	if a.charA >= 0 {
		cs, ce = a.charA, a.charC
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
		Kind:      render.CommentBlock,
		BlockIdx:  idx,
		CharStart: cs,
		CharEnd:   ce,
		Source:    src,
		Text:      text,
		CreatedAt: time.Now(),
	}
	m.comments = append(m.comments, c)
	m.state = stateNav
	m.reflow()
	m.refreshViewport()
}

// saveFileComment handles file-kind anchors: whole-file line-range
// (charA < 0) or inline byte-range (charA >= 0) comments.
func (m *model) saveFileComment(a commentAnchor, text string) {
	if a.filePath == "" {
		m.cancelCommentComposer()
		return
	}
	src := ""
	cs, ce := -1, -1
	if a.charA >= 0 {
		cs, ce = a.charA, a.charC
		if cs > ce {
			cs, ce = ce, cs
		}
		if cs < 0 {
			cs = 0
		}
		if cs > len(m.fileViewer.content) {
			cs = len(m.fileViewer.content)
		}
		if ce > len(m.fileViewer.content) {
			ce = len(m.fileViewer.content)
		}
		src = m.fileViewer.content[cs:ce]
	}
	c := render.Comment{
		Kind:      render.CommentFile,
		Path:      a.filePath,
		LineStart: a.lineStart,
		LineEnd:   a.lineEnd,
		CharStart: cs,
		CharEnd:   ce,
		Source:    src,
		Text:      text,
		CreatedAt: time.Now(),
	}
	m.comments = append(m.comments, c)
	// Re-render the file viewer so the new comment shows the
	// yellow gutter. refreshFileViewer recomputes the lineIndex
	// (HasComment flags); refreshFileView pushes the freshly
	// rendered lines into the viewport cache.
	m.refreshFileViewer()
	m.refreshFileView()
	m.state = m.fileReturnAfterComment()
	m.reflow()
}

// fileReturnAfterComment returns the file-review state to return
// to after saving a file-kind comment (file viewer when the anchor
// came from there, dir nav otherwise).
func (m *model) fileReturnAfterComment() state {
	if m.fileViewer.path != "" && m.fileViewer.path == m.commentAnchor.filePath {
		return stateFileView
	}
	return stateFileNav
}

// refreshFileViewer re-runs RenderFile for the current viewer
// to refresh the lineIndex (HasComment flags), after a comment
// was added that targets this file. The rendered body itself is
// cached in m.fileViewer.viewport (see refreshFileView), so
// callers that want the new gutter to show on screen must also
// call refreshFileView.
func (m *model) refreshFileViewer() {
	if m.fileViewer.path == "" {
		return
	}
	_, idx := render.RenderFile(m.fileViewer.content, m.fileCommentsFor(m.fileViewer.path))
	m.fileViewer.lineIndex = idx
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
			m.enterCommentComposer(buildCommentAnchor(&m.nav, &m.cursor, &m.selection, m.blocks))
		case render.ActionSend:
			m.handleSend()
		case render.ActionRefresh:
			return m, pollCmd(m.src)
		case render.ActionQuit:
			return m, tea.Quit
		case render.ActionCompose:
			m.enterCompose()
		}
		return m, nil
	}
	// Forward unhandled special keys to viewport (arrows, pgup/dn, etc.)
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, vpCmd
}

// handleFileNavKey delegates to the bubbles filepicker for
// directory traversal and selection, then opens the file viewer
// when the picker reports a chosen file via filepicker.Path. The
// pinky-level keys (c, s, q, Tab/Esc) are handled here.
func (m model) handleFileNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// pinky-level keys first.
	if key.Matches(msg, defaultKeyMap.FileNavComment) || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'c') {
		path := m.filePicker.Path
		if path == "" {
			return m, nil
		}
		rel, err := filepath.Rel(m.fileRoot, path)
		if err != nil {
			rel = path
		}
		m.commentAnchor = commentAnchor{
			kind:      render.CommentFile,
			filePath:  rel,
			lineStart: 1,
			lineEnd:   m.fileLineCount(rel),
		}
		m.enterCommentComposer(m.commentAnchor)
		return m, nil
	}
	if key.Matches(msg, defaultKeyMap.FileNavBack) || msg.Type == tea.KeyEsc {
		return m, m.toggleTab()
	}
	if key.Matches(msg, defaultKeyMap.NavSend) || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 's') {
		m.handleSend()
		return m, nil
	}
	if key.Matches(msg, defaultKeyMap.NavQuit) || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q') {
		return m, tea.Quit
	}

	// Delegate to the filepicker. If it set Path, open the viewer.
	prev := m.filePicker.Path
	updated, cmd := m.filePicker.Update(msg)
	m.filePicker = updated
	if m.filePicker.Path != "" && m.filePicker.Path != prev {
		rel, err := filepath.Rel(m.fileRoot, m.filePicker.Path)
		if err != nil {
			rel = m.filePicker.Path
		}
		m.openFileViewer(rel)
		// Clear the picker's selection so the next time the user
		// re-enters stateFileNav the picker doesn't immediately
		// re-open the same file.
		m.filePicker.Path = ""
	}
	return m, cmd
}

// fileLineCount returns the number of source lines in path, or 1
// if the file can't be read (a comment on an unreadable file still
// has a valid anchor).
func (m *model) fileLineCount(path string) int {
	full := m.fileRoot + "/" + path
	data, err := os.ReadFile(full)
	if err != nil {
		return 1
	}
	if len(data) == 0 {
		return 1
	}
	return strings.Count(string(data), "\n") + 1
}

// openFileViewer loads path's content and transitions to
// stateFileView.
func (m *model) openFileViewer(path string) {
	full := m.fileRoot + "/" + path
	data, err := os.ReadFile(full)
	if err != nil {
		m.latest = session.Message{
			Role: session.RoleUser,
			Text: "[file read failed: " + err.Error() + "]",
		}
		m.refreshViewport()
		return
	}
	content := string(data)
	_, idx := render.RenderFile(content, m.fileCommentsFor(path))
	w, h := m.viewportSize()
	h-- // ponytail: one line for the file-viewer header above the viewport.
	if h < 1 {
		h = 1
	}
	m.fileViewer = fileViewer{
		path:      path,
		content:   content,
		lines:     strings.Split(strings.TrimRight(content, "\n"), "\n"),
		cursor:    1,
		visual:    fileSelection{LineA: 1, CharA: 0, LineC: 1, CharC: 0},
		lineIndex: idx,
		viewport:  viewport.New(w, h),
	}
	m.state = stateFileView
	m.reflow()
	m.refreshFileView()
}

// fileCommentsFor returns the file-kind comments that target path.
func (m *model) fileCommentsFor(path string) []render.Comment {
	var out []render.Comment
	for _, c := range m.comments {
		if c.Kind == render.CommentFile && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

// handleFileViewKey routes keys in stateFileView: j/k move the
// line cursor, v enters visual, h/l extend visual cursor by rune,
// c opens the comment composer, s flushes, Esc returns to dir nav,
// Tab returns to message tab.
func (m model) handleFileViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc is handled before rune routing so visual-mode exit feels
	// like the message viewer's.
	if msg.Type == tea.KeyEsc {
		if m.fileViewer.visual.Active {
			m.fileViewer.visual.Active = false
			m.refreshFileView()
			return m, nil
		}
		m.state = stateFileNav
		m.reflow()
		return m, nil
	}

	if key.Matches(msg, defaultKeyMap.FileNavBack) && msg.Type != tea.KeyEsc {
		m.state = stateFileNav
		m.reflow()
		return m, nil
	}

	switch {
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q':
		return m, tea.Quit
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 's':
		m.handleSend()
		return m, nil
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'v':
		m.fileViewer.visual.Active = !m.fileViewer.visual.Active
		if m.fileViewer.visual.Active {
			line := m.fileViewer.cursor
			m.fileViewer.visual.LineA = line
			m.fileViewer.visual.CharA = 0
			m.fileViewer.visual.LineC = line
			m.fileViewer.visual.CharC = 0
		}
		m.refreshFileView()
		return m, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j' {
		m.fileViewMoveLine(+1)
		return m, nil
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k' {
		m.fileViewMoveLine(-1)
		return m, nil
	}

	// h / l: rune-granular in visual mode, no-op outside.
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l' {
		if m.fileViewer.visual.Active {
			m.fileViewMoveRune(+1)
			return m, nil
		}
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h' {
		if m.fileViewer.visual.Active {
			m.fileViewMoveRune(-1)
			return m, nil
		}
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'c' {
		m.openFileComment()
		return m, nil
	}
	// ponytail: forward scroll keys (arrows, PageUp/Down) to the
	// file viewport so the user can scroll without moving the
	// cursor. Home/End aren't in the viewport's default keymap;
	// handle them explicitly so the position-indicator and sticky-
	// bottom reattach patterns stay intuitive.
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
		var vpCmd tea.Cmd
		m.fileViewer.viewport, vpCmd = m.fileViewer.viewport.Update(msg)
		return m, vpCmd
	case tea.KeyHome:
		m.fileViewer.viewport.GotoTop()
		return m, nil
	case tea.KeyEnd:
		m.fileViewer.viewport.GotoBottom()
		return m, nil
	}
	return m, nil
}

// fileViewMoveLine moves the cursor line by delta, extending the
// visual selection if active (charA snaps to 0, charC snaps to the
// end of the destination line). After the move the file viewport
// is scrolled so the cursor stays visible (1-line cushion).
func (m *model) fileViewMoveLine(delta int) {
	max := len(m.fileViewer.lines)
	if max == 0 {
		return
	}
	next := m.fileViewer.cursor + delta
	if next < 1 {
		next = 1
	}
	if next > max {
		next = max
	}
	m.fileViewer.cursor = next
	if m.fileViewer.visual.Active {
		v := &m.fileViewer.visual
		v.LineC = next
		// When extending the line range with j/k, snap the anchor
		// charA to the start of the first selected line and charC
		// to the end of the last selected line. Compute both ends
		// from the line endpoints regardless of delta direction.
		v.CharA = 0
		v.CharC = len(m.fileViewer.lines[next-1])
	}
	m.scrollFileCursorIntoView()
	m.refreshFileView()
}

// scrollFileCursorIntoView mirrors scrollCursorIntoView for the
// file viewer: if the cursor's 0-indexed line falls outside
// [YOffset, YOffset+Height), scroll so it lands on the last
// visible line. Same 1-line cushion as the message view.
func (m *model) scrollFileCursorIntoView() {
	if len(m.fileViewer.lines) == 0 {
		return
	}
	target := m.fileViewer.cursor - 1
	top := m.fileViewer.viewport.YOffset
	bot := top + m.fileViewer.viewport.Height - 1
	if target >= top && target <= bot {
		return
	}
	if target < top {
		m.fileViewer.viewport.SetYOffset(target)
	} else {
		m.fileViewer.viewport.SetYOffset(target - m.fileViewer.viewport.Height + 1)
	}
}

// fileViewMoveRune moves the visual-mode cursor one rune within
// the current line. No-op when visual is inactive.
func (m *model) fileViewMoveRune(delta int) {
	if !m.fileViewer.visual.Active {
		return
	}
	lineIdx := m.fileViewer.cursor - 1
	if lineIdx < 0 || lineIdx >= len(m.fileViewer.lines) {
		return
	}
	line := m.fileViewer.lines[lineIdx]
	c := m.fileViewer.visual.CharC
	if delta > 0 {
		if c >= len(line) {
			return
		}
		_, sz := utf8.DecodeRuneInString(line[c:])
		c += sz
	} else {
		if c <= 0 {
			return
		}
		_, sz := utf8.DecodeLastRuneInString(line[:c])
		c -= sz
	}
	m.fileViewer.visual.CharC = c
	m.refreshFileView()
}

// openFileComment opens the comment composer with an anchor from
// the current cursor / visual selection. No selection → whole
// file; visual mode line-range → line range; visual mode inline
// selection → char range on a single line (or char range across
// the snapped line endpoints).
func (m *model) openFileComment() {
	fv := &m.fileViewer
	a := commentAnchor{
		kind:     render.CommentFile,
		filePath: fv.path,
	}
	if fv.visual.Active {
		a.lineStart, a.lineEnd = fv.visual.LineA, fv.visual.LineC
		if a.lineStart > a.lineEnd {
			a.lineStart, a.lineEnd = a.lineEnd, a.lineStart
		}
		// Snap char endpoints if they look un-snapped (visual just
		// toggled without movement). Treat as whole-line range.
		if a.lineStart == a.lineEnd && fv.visual.CharA != fv.visual.CharC {
			a.charA = fv.visual.CharA
			a.charC = fv.visual.CharC
			if a.charA > a.charC {
				a.charA, a.charC = a.charC, a.charA
			}
		}
	} else {
		a.lineStart = 1
		a.lineEnd = len(fv.lines)
	}
	m.commentAnchor = a
	m.enterCommentComposer(a)
}

// buildCommentAnchor assembles the (block, charA, charC) for the next
// comment. With an active selection, anchor is the selection range;
// otherwise anchor covers the whole block at the cursor.
func buildCommentAnchor(st *render.NavState, cur *render.NavCursor, sel *render.NavSelection, blocks []render.Block) commentAnchor {
	if st.Visual == render.NavLine {
		idx := sel.BlockIdx
		if idx < 0 || idx >= len(blocks) {
			idx = cur.BlockIdx
		}
		a, c := sel.CharA, sel.CharC
		if a > c {
			a, c = c, a
		}
		return commentAnchor{blockIdx: idx, charA: a, charC: c}
	}
	idx := cur.BlockIdx
	if idx < 0 || idx >= len(blocks) {
		return commentAnchor{blockIdx: -1, charA: -1, charC: -1}
	}
	return commentAnchor{blockIdx: idx, charA: 0, charC: len(blocks[idx].Source)}
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
			text += "\n\n---\n" + render.FormatCommentsAppendix(m.comments, m.blocks)
		}
		if !m.dispatch(text) {
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
	if m.state == stateFileView {
		vpHeight-- // ponytail: header line above the file viewport.
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	if m.fileViewer.path != "" {
		m.fileViewer.viewport.Width = m.width
		m.fileViewer.viewport.Height = vpHeight
		// ponytail: every reflow that changes the file viewport
		// size can leave the cursor's line off the new visible
		// range — resize, help toggle (?), tab/state change all
		// route through here. Re-pull the cursor into view once
		// at the source instead of patching every caller.
		m.scrollFileCursorIntoView()
	}
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
			defaultKeyMap.NavComment,
			defaultKeyMap.NavSend,
			defaultKeyMap.Tab,
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
	case stateFileNav:
		return []key.Binding{
			defaultKeyMap.FileNavOpen,
			defaultKeyMap.FileNavComment,
			defaultKeyMap.NavSend,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
		}
	case stateFileView:
		return []key.Binding{
			defaultKeyMap.FileViewComment,
			defaultKeyMap.NavSend,
			defaultKeyMap.FileNavBack,
			defaultKeyMap.Tab,
			defaultKeyMap.Help,
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
	switch m.state {
	case statePicking:
		return [][]key.Binding{
			{defaultKeyMap.Up, defaultKeyMap.Down},
			{defaultKeyMap.Pick},
			{defaultKeyMap.QuitPick, defaultKeyMap.Help},
		}
	case stateNav:
		return [][]key.Binding{
			{defaultKeyMap.NavBlockDown, defaultKeyMap.NavBlockUp, defaultKeyMap.NavRuneLeft, defaultKeyMap.NavRuneRight},
			{defaultKeyMap.NavVisual, defaultKeyMap.NavComment, defaultKeyMap.NavSend, defaultKeyMap.NavCompose, defaultKeyMap.NavRefresh},
			{defaultKeyMap.Tab, defaultKeyMap.NavQuit, defaultKeyMap.Help},
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
	case stateFileNav:
		return [][]key.Binding{
			{defaultKeyMap.FileNavDown, defaultKeyMap.FileNavUp, defaultKeyMap.FileNavCollapse, defaultKeyMap.FileNavExpand},
			{defaultKeyMap.FileNavOpen, defaultKeyMap.FileNavComment, defaultKeyMap.NavSend},
			{defaultKeyMap.FileNavBack, defaultKeyMap.Tab, defaultKeyMap.Help},
		}
	case stateFileView:
		return [][]key.Binding{
			{defaultKeyMap.FileViewDown, defaultKeyMap.FileViewUp},
			{defaultKeyMap.FileViewVisual, defaultKeyMap.FileViewComment, defaultKeyMap.NavSend},
			{defaultKeyMap.FileViewBack, defaultKeyMap.Tab, defaultKeyMap.Help},
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

// tabChipStyle is the dim accent used for the active-tab chip in
// the status line ("msg" / "files"). Kept dim so it doesn't
// compete with the [VISUAL] chip or the pane line.
var tabChipStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("241")).
	Padding(0, 1)

func (m *model) refreshViewport() {
	if m.latest.Text == "" {
		m.blocks = nil
		m.wrappedToSrc = nil
		m.sourceToFirst = nil
		m.viewport.SetContent(placeholderStyle.Render(placeholderText))
		return
	}
	// RenderMessageWithComments is a strict superset of renderBlocks;
	// for zero comments it returns the same output.
	rendered, blocks := render.RenderMessageWithComments(m.latest.Text, m.comments)
	if rendered == "" {
		// Renderer rejected this width (very narrow terminal): plain-text fallback.
		m.wrappedToSrc = nil
		m.sourceToFirst = nil
		m.viewport.SetContent(m.latest.Text)
		return
	}
	// ponytail: word-wrap to fit the viewport before the gutter is
	// prepended. Without this, long lines get truncated on the right
	// by the viewport's ansi.Cut. Width - 1 leaves room for the
	// single-cell gutter character.
	var wrapWidth int
	if w := m.viewport.Width; w > 1 {
		wrapWidth = w - 1
	}
	m.blocks = blocks
	focused := m.cursor.BlockIdx
	if focused < 0 || focused >= len(m.blocks) {
		focused = render.CurrentBlockIdx(blocks, m.wrappedYOffsetToSource(m.viewport.YOffset))
	}
	guttered, w2s := render.InjectGutterWrapped(rendered, m.blocks, focused, wrapWidth)
	m.wrappedToSrc = w2s
	// Build the inverse: first wrapped line of each source line.
	m.sourceToFirst = make([]int, len(strings.Split(rendered, "\n")))
	for i := 1; i < len(w2s); i++ {
		if w2s[i] != w2s[i-1] && m.sourceToFirst[w2s[i]] == 0 {
			m.sourceToFirst[w2s[i]] = i
		}
	}
	m.viewport.SetContent(guttered)
}

// wrappedYOffsetToSource translates a viewport YOffset (in wrapped
// lines) back to the underlying markdown source-line index that
// CurrentBlockIdx / NavLineIndex expect. Returns 0 when the
// viewport is empty / no wrap map.
func (m *model) wrappedYOffsetToSource(y int) int {
	if len(m.wrappedToSrc) == 0 {
		return 0
	}
	if y < 0 {
		return m.wrappedToSrc[0]
	}
	if y >= len(m.wrappedToSrc) {
		return m.wrappedToSrc[len(m.wrappedToSrc)-1]
	}
	return m.wrappedToSrc[y]
}

// sourceYOffset returns the first wrapped YOffset that belongs to
// the given source line. Used to scroll the viewport to a block
// boundary (e.g. when the cursor moves to a new block).
func (m *model) sourceYOffset(src int) int {
	if src < 0 || src >= len(m.sourceToFirst) {
		return 0
	}
	return m.sourceToFirst[src]
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
	case stateFileNav:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.fileNavView(),
			helpView,
			m.statusLine(),
		))
	case stateFileView:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.fileViewView(),
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

// padRight appends spaces so s reaches exactly width visible cells.
// Returns s unchanged when width is non-positive or s already fills
// the line.
func padRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	pad := width - render.VisibleWidth(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
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
		b.WriteString(padRight(line, m.width))
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

// fileNavView renders the workspace header followed by the
// bubbles filepicker view (current directory listing).
func (m model) fileNavView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	return headerStyle.Render(fmt.Sprintf("workspace — %s", m.filePicker.CurrentDirectory)) + "\n" + m.filePicker.View()
}

// renderFileContent builds the per-line rendered string for the
// file viewer: right-aligned line numbers, a yellow `▍` gutter on
// commented lines, and an inline cyan highlight of the visual
// selection. No header — the header (with optional position
// indicator) lives in fileViewView, outside the viewport so it
// stays visible while the body scrolls.
func (m model) renderFileContent() string {
	if len(m.fileViewer.lines) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		return dimStyle.Render("(empty file)")
	}
	const yellow = "\x1b[38;5;228m"
	width := len(fmt.Sprintf("%d", len(m.fileViewer.lines)))
	var b strings.Builder
	for i, content := range m.fileViewer.lines {
		ln := i + 1
		gutter := " "
		if i < len(m.fileViewer.lineIndex) && m.fileViewer.lineIndex[i].HasComment {
			gutter = yellow + "▍" + resetANSI
		}
		fmt.Fprintf(&b, "%s %*d  %s\n", gutter, width, ln, applySelection(content, ln, m.fileViewer.visual))
	}
	return strings.TrimRight(b.String(), "\n")
}

// fileViewView renders the file-viewer header (path + optional
// `lines N-M of K` position indicator) followed by the file
// viewport's visible window. The header sits outside the viewport
// so the indicator stays visible while the body scrolls.
func (m model) fileViewView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	header := fmt.Sprintf("file — %s", m.fileViewer.path)
	if len(m.fileViewer.lines) > m.fileViewer.viewport.Height {
		top := m.fileViewer.viewport.YOffset + 1
		bot := top + m.fileViewer.viewport.Height - 1
		if bot > len(m.fileViewer.lines) {
			bot = len(m.fileViewer.lines)
		}
		header += fmt.Sprintf("  (lines %d-%d of %d)", top, bot, len(m.fileViewer.lines))
	}
	return headerStyle.Render(header) + "\n" + m.fileViewer.viewport.View()
}

// refreshFileView rebuilds the file viewer's content and hands it
// to the viewport. Called on file-open and after every cursor /
// selection move that affects the rendered gutter or selection
// highlight.
func (m *model) refreshFileView() {
	if m.fileViewer.path == "" {
		return
	}
	m.fileViewer.viewport.SetContent(m.renderFileContent())
}

// applySelection splices cyan ANSI around the byte range of lineNo
// that falls inside sel. Returns content unchanged when sel is
// inactive or this line is outside the selection.
func applySelection(line string, lineNo int, sel fileSelection) string {
	if !sel.Active {
		return line
	}
	la, lc := sel.LineA, sel.LineC
	if la > lc {
		la, lc = lc, la
	}
	if lineNo < la || lineNo > lc {
		return line
	}
	var a, c int
	switch {
	case la == lc:
		a, c = sel.CharA, sel.CharC
	case lineNo == la:
		a, c = sel.CharA, len(line)
	case lineNo == lc:
		a, c = 0, sel.CharC
	default:
		a, c = 0, len(line)
	}
	if a > c {
		a, c = c, a
	}
	if a > len(line) {
		a = len(line)
	}
	if c > len(line) {
		c = len(line)
	}
	if a >= c {
		return line
	}
	return line[:a] + selectionANSI + line[a:c] + resetANSI + line[c:]
}

const (
	selectionANSI = "\x1b[38;5;51m"
	resetANSI     = "\x1b[0m"
)

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
	// Tab chip: always present so the user knows which tab is
	// active. Dim style keeps it subordinate to the pane line.
	switch m.tab {
	case tabFiles:
		rendered += " " + tabChipStyle.Render(" files ")
	default:
		rendered += " " + tabChipStyle.Render(" msg ")
	}
	return padRight(rendered, m.width)
}
