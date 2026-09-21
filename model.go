package main

import (
	"fmt"
	"strings"
	"time"

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

// attach opens a session source + history for the given pane and
// transitions to idle state. History is opened for append-only writing
// but is NOT loaded back into the view (the view starts fresh).
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

	m.pane = pane
	m.src = src
	m.hist = hist
	m.viewport = viewport.New(40, 20)
	m.textarea = ta
	m.state = stateIdle
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

	m.pane = "(explicit)"
	m.src = src
	m.hist = nil
	m.viewport = viewport.New(40, 20)
	m.textarea = ta
	m.state = stateIdle
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
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	switch m.state {
	case statePicking:
		return m.handlePickerKey(msg)
	case stateIdle:
		return m.handleIdleKey(msg)
	case stateCompose:
		return m.handleComposeKey(msg)
	case stateError:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.moveCursor(-1)
	case tea.KeyDown:
		m.moveCursor(+1)
	case tea.KeyEnter:
		if err := m.selectAgent(m.cursor); err != nil {
			m.state = stateError
			m.err = err
			return m, nil
		}
		m.refreshViewport()
		return m, tea.Batch(tickCmd(), pollCmd(m.src))
	}
	// Vim-style aliases (j/k) for picker navigation.
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'j':
			m.moveCursor(+1)
		case 'k':
			m.moveCursor(-1)
		}
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
	switch msg.Type {
	case tea.KeyCtrlN:
		m.state = stateCompose
		m.vim = render.VimState{} // clear any pending two-key state
		m.textarea.Focus()
		m.textarea.Reset()
		m.reflow()
		return m, nil
	case tea.KeyCtrlR:
		return m, pollCmd(m.src)
	case tea.KeyUp:
		m.viewport.LineUp(1)
		return m, nil
	case tea.KeyDown:
		m.viewport.LineDown(1)
		return m, nil
	case tea.KeyPgUp:
		m.viewport.GotoTop()
		return m, nil
	case tea.KeyPgDown:
		m.applyAction(render.VimNextBlock)
		return m, nil
	}

	// Vim navigation.
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
	switch msg.Type {
	case tea.KeyCtrlS:
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
	case tea.KeyEsc:
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
		return m.pickerView()
	case stateCompose:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.textarea.View(),
			m.statusLine(),
		)
	case stateError:
		return m.errorView()
	default:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.statusLine(),
		)
	}
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
	return fmt.Sprintf(" %s %s", dot, m.pane)
}
