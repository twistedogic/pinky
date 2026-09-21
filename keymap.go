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

	// Comments (idle view)
	Mark         key.Binding // m
	Visual       key.Binding // V
	EditComment  key.Binding // e
	DeleteComment key.Binding // d
	NextComment  key.Binding // n
	PrevComment  key.Binding // N
	SubmitComments key.Binding // s

	// Visual mode (active after V; bindings dispatched via
	// render.VisualState.Handle, surfaced here for the help overlay).
	VisualDown      key.Binding // j
	VisualUp        key.Binding // k
	VisualNextBlock key.Binding // }
	VisualPrevBlock key.Binding // {
	VisualOpen      key.Binding // c
	VisualExit      key.Binding // esc

	// Compose
	Send   key.Binding // ctrl+s
	Newline key.Binding // enter
	Cancel key.Binding // esc
	IncludeComments key.Binding // ctrl+i

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

	// Comments
	Mark: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "mark block"),
	),
	Visual: key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("V", "visual mode"),
	),
	EditComment: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit comment"),
	),
	DeleteComment: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete comment"),
	),
	NextComment: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "next comment"),
	),
	PrevComment: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "prev comment"),
	),
	SubmitComments: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "submit all comments"),
	),

	// Visual mode (documentation only — actual dispatch goes through
	// render.VisualState.Handle)
	VisualDown: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "cursor down"),
	),
	VisualUp: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "cursor up"),
	),
	VisualNextBlock: key.NewBinding(
		key.WithKeys("}"),
		key.WithHelp("}", "next block"),
	),
	VisualPrevBlock: key.NewBinding(
		key.WithKeys("{"),
		key.WithHelp("{", "prev block"),
	),
	VisualOpen: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment"),
	),
	VisualExit: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "exit visual"),
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
	IncludeComments: key.NewBinding(
		key.WithKeys("ctrl+i"),
		key.WithHelp("^I", "include comments"),
	),

	// Error
	QuitError: key.NewBinding(
		key.WithKeys("enter", "esc", "ctrl+c"),
		key.WithHelp("⏎", "dismiss"),
	),
}

