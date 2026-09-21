package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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

	roleAgent = "agent"
	roleUser  = "user"
)

type entry struct {
	role string
	text string
	ts   time.Time
}

type state int

const (
	statePicking state = iota
	stateIdle
	stateCompose
	stateError
)

type tickMsg time.Time
type sessionMsg struct {
	entries []entry
	err     error
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
	latest   entry
	blocks   []render.Block
	renderer *glamour.TermRenderer
	rendW    int

	viewport viewport.Model
	textarea textarea.Model

	width, height int

	streaming   bool
	lastChanged time.Time

	vim render.VimState

	// Error state: set when initialization or attach fails. The TUI
	// shows the message and exits on any key press.
	err error

	// help toggles the in-TUI help overlay (rendered via help.Model).
	help bool
}

// newModel returns a picker model. Callers must either call setAgents
// (then run the picker) or call attach (skip the picker, jump to running).
func newModel() model {
	vp := viewport.New(40, 20)
	return model{
		state:    statePicking,
		viewport: vp,
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

// attach opens a session source + history for the given pane and
// transitions to idle state. History is opened for append-only writing
// but is NOT loaded back into the view (the view starts fresh).
//
// Always seeds the viewport via refreshViewport so the placeholder
// (or the latest message if one was already known) renders immediately
// — same behavior whether the user picked the session from the picker
// or jumped straight to it via --target.
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
	return nil
}

// attachWithFile is the --session-file escape hatch: skip pane
// discovery and use the given JSONL path directly. Used when the
// auto-discovery (PI_SESSION_FILE / lsof) can't find the file.
// ponytail: no pane → no inject.Send target; compose still works
// locally but the redirect has nowhere to go.
func (m *model) attachWithFile(path string) error {
	src, err := session.OpenFile(path)
	if err != nil {
		return err
	}

	ta := textarea.New()
	ta.Placeholder = "redirect — Enter newline, Ctrl+S send, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)

	w, h := m.viewportSize()

	m.pane = "(explicit)"
	m.src = src
	m.hist = nil
	m.viewport = viewport.New(w, h)
	m.textarea = ta
	m.state = stateIdle
	m.refreshViewport()
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
		return tea.Batch(tickCmd(), pollCmd(m.src))
	}
	return nil
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func pollCmd(src session.Source) tea.Cmd {
	return func() tea.Msg {
		msgs, err := src.NewMessages()
		if err != nil {
			return sessionMsg{err: err}
		}
		var entries []entry
		for _, m := range msgs {
			role := roleAgent
			if m.Role == session.RoleUser {
				role = roleUser
			}
			entries = append(entries, entry{role: role, text: m.Text, ts: m.Ts})
		}
		return sessionMsg{entries: entries}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.reflow()
		// Force renderer rebuild on width change.
		m.renderer = nil
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		return m, tea.Batch(pollCmd(m.src), tickCmd())

	case sessionMsg:
		if msg.err != nil {
			return m, tickCmd()
		}
		if len(msg.entries) == 0 {
			m.streaming = false
			return m, nil
		}
		// Find the latest assistant message in this poll. A user redirect
		// arriving after the agent's last reply should NOT replace the
		// view — we want the agent's most recent text, not the user's own.
		var last *entry
		for i := range msg.entries {
			if msg.entries[i].role == roleAgent {
				last = &msg.entries[i]
			}
		}
		if last != nil {
			m.latest = *last
		}
		if m.hist != nil && last != nil {
			_ = m.hist.Append(last.role, last.text)
		}
		m.streaming = true
		m.lastChanged = time.Now()
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
	// Ctrl+C and the help toggle are global across every state.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if key.Matches(msg, defaultKeyMap.Help) {
		m.help = !m.help
		if m.help {
			m.showHelpMarkdown()
		} else {
			m.refreshViewport()
		}
		return m, nil
	}
	// In help mode, any other key dismisses the help and is
	// reprocessed (so the user can hit a navigation key without
	// having to press ? first).
	if m.help {
		m.help = false
		m.refreshViewport()
		return m.Update(msg)
	}

	switch m.state {
	case statePicking:
		return m.handlePickerKey(msg)
	case stateIdle:
		return m.handleIdleKey(msg)
	case stateCompose:
		return m.handleComposeKey(msg)
	case stateError:
		// Any key dismisses the error and quits.
		if key.Matches(msg, defaultKeyMap.QuitError) {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

// showHelpMarkdown renders the per-state keymap as markdown via the
// existing glamour pipeline and pushes it into the viewport.
func (m *model) showHelpMarkdown() {
	r := m.getRenderer()
	if r == nil {
		m.viewport.SetContent(keymapMarkdown(m.state))
		return
	}
	rendered, _ := render.RenderMessage(keymapMarkdown(m.state), m.width)
	m.viewport.SetContent(rendered)
}

// enterCompose transitions to compose mode with the textarea reset
// and focused. Shared by Ctrl+N and the `c` alias.
func (m *model) enterCompose() {
	m.state = stateCompose
	m.vim = render.VimState{}
	m.textarea.Focus()
	m.textarea.Reset()
	m.reflow()
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
		return m, tea.Batch(tickCmd(), pollCmd(m.src))
	case key.Matches(msg, defaultKeyMap.QuitPick):
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) moveCursor(delta int) {
	if len(m.agents) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = len(m.agents) - 1
	} else if m.cursor >= len(m.agents) {
		m.cursor = 0
	}
}

func (m model) handleIdleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, defaultKeyMap.Send):
		text := m.textarea.Value()
		if text == "" {
			return m, nil
		}
		if m.hist != nil {
			_ = m.hist.Append(roleUser, text)
		}
		if err := inject.Send(m.pane, text); err != nil {
			// ponytail: surface a transient placeholder rather than
			// permanently mutating view state; revisit if it bites.
			m.latest = entry{
				role: roleUser,
				text: "[send failed: " + err.Error() + "]",
				ts:   time.Now(),
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
	vpHeight := m.height - statusHeight
	if m.state == stateCompose {
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

// getRenderer returns a cached glamour renderer for the current width,
// rebuilding it if the width changed.
func (m *model) getRenderer() *glamour.TermRenderer {
	if m.renderer == nil || m.rendW != m.width {
		r, err := render.NewRenderer(m.width)
		if err != nil {
			return nil
		}
		m.renderer = r
		m.rendW = m.width
	}
	return m.renderer
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

func (m *model) refreshViewport() {
	if m.latest.text == "" {
		m.blocks = nil
		m.viewport.SetContent(placeholderStyle.Render(placeholderText))
		return
	}
	r := m.getRenderer()
	if r == nil {
		// Render failed (very narrow terminal): fall back to plain text.
		m.viewport.SetContent(m.latest.text)
		return
	}
	rendered, blocks := render.RenderMessage(m.latest.text, m.width)
	m.blocks = blocks
	m.viewport.SetContent(m.injectBorder(rendered))
}

// injectBorder wraps the rendered output with horizontal border lines
// above and below the currently-focused block. The viewport content is
// rewritten line-by-line so the border sits at exact line indices.
func (m *model) injectBorder(rendered string) string {
	idx := render.CurrentBlockIdx(m.blocks, m.viewport.YOffset)
	if idx < 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	border := borderStyle.Render(strings.Repeat("─", m.width))
	var b strings.Builder
	for i, line := range lines {
		if i == m.blocks[idx].StartLine {
			b.WriteString(border)
			b.WriteByte('\n')
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if i == m.blocks[idx].EndLine {
			b.WriteString(border)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (m model) View() string {
	switch m.state {
	case statePicking:
		return m.fillWidth(m.pickerView())
	case stateCompose:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.textarea.View(),
			m.statusLine(),
		))
	case stateError:
		return m.fillWidth(m.errorView())
	default:
		return m.fillWidth(lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.statusLine(),
		))
	}
}

// visibleWidth returns the visible (ANSI-stripped) width of s. Used
// to pad lines to the terminal width; lipgloss.Width is off-by-one
// when styling adds background-color padding (e.g. statusBarStyle).
func visibleWidth(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		if r == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
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
		if pad := m.width - visibleWidth(line); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	return b.String()
}

// errorView renders a single centered error message. The TUI shows
// this when attach or session discovery fails; any key press quits.
func (m model) errorView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	return headerStyle.Render("pinky: error") + "\n\n" +
		bodyStyle.Render(m.err.Error()) + "\n\n" +
		hintStyle.Render("press any key to quit")
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
	b.WriteByte('\n')
	b.WriteString(dimStyle.Render("↑/↓ navigate  Enter select  Ctrl+C quit"))
	return b.String()
}

func (m model) statusLine() string {
	dot := "·"
	if m.streaming {
		dot = "●"
	}
	text := fmt.Sprintf("%s %s", dot, m.pane)
	rendered := statusBarStyle.Render(text)
	if m.width <= 0 {
		return rendered
	}
	if pad := m.width - visibleWidth(rendered); pad > 0 {
		rendered += strings.Repeat(" ", pad)
	}
	return rendered
}
