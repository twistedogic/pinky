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
	Help key.Binding // ?

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
	BottomLine key.Binding // G
	Compose    key.Binding // ctrl+n, c
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
	BottomLine: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "bottom"),
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

