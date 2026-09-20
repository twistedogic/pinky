package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/twistedogic/pinky/internal/history"
	"github.com/twistedogic/pinky/internal/inject"
	"github.com/twistedogic/pinky/internal/session"
)

const (
	roleAgent = "agent"
	roleUser  = "user"

	composeHeight = 4
	statusHeight  = 1
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
)

// Messages
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

	// Attached state (populated after picker selection or --target)
	pane    string
	src     session.Source
	hist    *history.History
	entries []entry

	viewport viewport.Model
	textarea textarea.Model

	width, height int

	streaming   bool
	lastChanged time.Time
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

// setAgents populates the picker list. Empty list is a programming error.
func (m *model) setAgents(agents []session.AgentSession) {
	m.agents = agents
	m.cursor = 0
}

// attach opens a session source + history for the given pane and transitions
// to idle state. Used when --target is passed.
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
	seeded, err := history.Load(pane)
	if err != nil {
		_ = src.Close()
		_ = hist.Close()
		return fmt.Errorf("load history: %w", err)
	}

	ta := textarea.New()
	ta.Placeholder = "redirect — Enter newline, Ctrl+S send, Esc cancel"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(composeHeight)

	entries := make([]entry, len(seeded))
	for i, e := range seeded {
		entries[i] = entry{role: e.Role, text: e.Text, ts: e.Ts}
	}

	m.pane = pane
	m.src = src
	m.hist = hist
	m.entries = entries
	m.viewport = viewport.New(40, 20)
	m.textarea = ta
	m.state = stateIdle
	if len(entries) > 0 {
		m.lastChanged = entries[len(entries)-1].ts
	}
	return nil
}

// selectAgent opens the session source for the chosen agent and transitions
// to idle state. Called from the picker on Enter.
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
		for _, e := range msg.entries {
			m.entries = append(m.entries, e)
			if m.hist != nil {
				_ = m.hist.Append(e.role, e.text)
			}
		}
		m.streaming = true
		m.lastChanged = time.Now()
		m.refreshViewport()
		m.viewport.GotoBottom()
		return m, nil
	}

	// Forward unhandled messages to focused component.
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
		switch msg.Type {
		case tea.KeyCtrlN:
			m.state = stateCompose
			m.textarea.Focus()
			m.textarea.Reset()
			m.reflow()
			return m, nil
		case tea.KeyCtrlR:
			m.entries = m.entries[:0]
			m.refreshViewport()
			return m, pollCmd(m.src)
		}
	case stateCompose:
		if msg.Type == tea.KeyCtrlS {
			text := m.textarea.Value()
			if text == "" {
				return m, nil
			}
			if m.hist != nil {
				_ = m.hist.Append(roleUser, text)
			}
			if err := inject.Send(m.pane, text); err != nil {
				m.entries = append(m.entries, entry{
					role: roleUser,
					text: "[send failed: " + err.Error() + "]",
					ts:   time.Now(),
				})
				m.refreshViewport()
				m.viewport.GotoBottom()
				return m, nil
			}
			m.entries = append(m.entries, entry{role: roleUser, text: text, ts: time.Now()})
			m.state = stateIdle
			m.textarea.Blur()
			m.textarea.Reset()
			m.refreshViewport()
			m.viewport.GotoBottom()
			m.reflow()
			return m, nil
		}
		if msg.Type == tea.KeyEsc {
			m.state = stateIdle
			m.textarea.Blur()
			m.textarea.Reset()
			m.reflow()
			return m, nil
		}
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

func (m model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.cursor = len(m.agents) - 1
		}
	case tea.KeyDown:
		if m.cursor < len(m.agents)-1 {
			m.cursor++
		} else {
			m.cursor = 0
		}
	case tea.KeyEnter:
		if err := m.selectAgent(m.cursor); err != nil {
			return m, tea.Quit
		}
		return m, tea.Batch(tickCmd(), pollCmd(m.src))
	case tea.KeyCtrlR:
		// No-op in picker for now; could re-discover later.
	}
	return m, nil
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

func (m *model) refreshViewport() {
	var b strings.Builder
	agentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Italic(true)
	for _, e := range m.entries {
		prefix := "  "
		style := agentStyle
		if e.role == roleUser {
			prefix = "> "
			style = userStyle
		}
		b.WriteString(style.Render(prefix + e.text))
		b.WriteByte('\n')
	}
	m.viewport.SetContent(b.String())
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
	default:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.statusLine(),
		)
	}
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
	return fmt.Sprintf(" %s %s  lines:%d", dot, m.pane, len(m.entries))
}
