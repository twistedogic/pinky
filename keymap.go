package main

import (
	"github.com/charmbracelet/bubbles/key"
)

// keyMap is the declarative set of keybindings used by pinky. Each
// binding carries the keys it matches and the help text shown in the
// in-TUI help overlay. The bindings are grouped by the model state
// that interprets them.
//
// The vim two-key sequences (gg, ]], [[) are matched by their first
// key here ('g', ']', '['); the actual two-key state machine lives in
// render.VimState and is fed the rune directly when no special-key
// binding matched.
type keyMap struct {
	// Shared
	Quit   key.Binding // q, ctrl+c
	Help   key.Binding // ?

	// Picker
	Up       key.Binding // k, ↑
	Down     key.Binding // j, ↓
	Pick     key.Binding // enter
	QuitPick key.Binding // q (in picker)

	// Idle view navigation
	LineDown    key.Binding // j, ↓
	LineUp      key.Binding // k, ↑
	NextBlock   key.Binding // }
	PrevBlock   key.Binding // {
	TopLine     key.Binding // gg (first key)
	TopSecond  key.Binding // gg (second key)
	BottomLine  key.Binding // G
	NextHeadingFirst  key.Binding // ]] (first key)
	NextHeadingSecond key.Binding // ]] (second key)
	PrevHeadingFirst  key.Binding // [[ (first key)
	PrevHeadingSecond key.Binding // [[ (second key)
	Compose key.Binding // ctrl+n, c
	Refresh key.Binding // ctrl+r
	QuitIdle key.Binding // q (in idle)

	// Compose
	Send   key.Binding // ctrl+s
	Newline key.Binding // enter
	Cancel key.Binding // esc

	// Error
	QuitError key.Binding // any key
}

// defaultKeyMap is the singleton keymap used at runtime.
var defaultKeyMap = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),

	// Picker
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "down"),
	),
	Pick: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "select"),
	),
	QuitPick: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),

	// Idle
	LineDown: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "line down"),
	),
	LineUp: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "line up"),
	),
	NextBlock: key.NewBinding(
		key.WithKeys("}"),
		key.WithHelp("}", "next block"),
	),
	PrevBlock: key.NewBinding(
		key.WithKeys("{"),
		key.WithHelp("{", "prev block"),
	),
	TopLine: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "(first of gg)"),
	),
	TopSecond: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "(second of gg)"),
	),
	BottomLine: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "bottom"),
	),
	NextHeadingFirst: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "(first of ]])"),
	),
	NextHeadingSecond: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "(second of ]])"),
	),
	PrevHeadingFirst: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "(first of [[)"),
	),
	PrevHeadingSecond: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "(second of [[)"),
	),
	Compose: key.NewBinding(
		key.WithKeys("ctrl+n", "c"),
		key.WithHelp("c", "compose"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("^R", "refresh"),
	),
	QuitIdle: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),

	// Compose
	Send: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("^S", "send"),
	),
	Newline: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "newline"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),

	// Error
	QuitError: key.NewBinding(
		key.WithKeys("enter", "esc", "ctrl+c"),
		key.WithHelp("⏎", "dismiss"),
	),
}

// helpVisible controls whether the in-TUI help overlay is shown.
type helpVisible bool

// ShortHelp returns the single-line help for the status bar. The set
// of bindings depends on the current model state.
func (m model) ShortHelp() []key.Binding {
	switch m.state {
	case statePicking:
		return []key.Binding{
			defaultKeyMap.Up, defaultKeyMap.Down, defaultKeyMap.Pick,
			defaultKeyMap.QuitPick, defaultKeyMap.Help,
		}
	case stateIdle:
		return []key.Binding{
			defaultKeyMap.LineDown, defaultKeyMap.LineUp,
			defaultKeyMap.NextBlock, defaultKeyMap.Compose,
			defaultKeyMap.QuitIdle, defaultKeyMap.Help,
		}
	case stateCompose:
		return []key.Binding{
			defaultKeyMap.Send, defaultKeyMap.Cancel, defaultKeyMap.Help,
		}
	case stateError:
		return []key.Binding{defaultKeyMap.QuitError, defaultKeyMap.Help}
	}
	return nil
}

// FullHelp returns the multi-column help for the ? overlay.
func (m model) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{defaultKeyMap.LineDown, defaultKeyMap.LineUp},
		{defaultKeyMap.NextBlock, defaultKeyMap.PrevBlock},
		{defaultKeyMap.BottomLine},
		{defaultKeyMap.Compose, defaultKeyMap.Refresh},
		{defaultKeyMap.Quit, defaultKeyMap.Help},
	}
}
