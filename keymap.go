package main

import (
	"github.com/charmbracelet/bubbles/key"
)

// keyMap is the declarative set of keybindings used by pinky. Most
// nav keys are matched by handleNavKey reading the rune directly —
// the bindings here are only consumed by key.Matches() for keys that
// the nav state machine doesn't own (compose, comment-composer, error
// dismissal, include-comments toggle). Each nav binding is a separate
// key.Binding so the help overlay can render one row per key with a
// per-key description (instead of collapsing every nav key into one
// undifferentiated row).
type keyMap struct {
	// Shared
	Help key.Binding // ?

	// Picker
	Up       key.Binding // k, ↑
	Down     key.Binding // j, ↓
	Pick     key.Binding // enter
	QuitPick key.Binding // q

	// Nav — help-overlay only; rune matching lives in handleNavKey.
	NavBlockDown key.Binding // j, ↓
	NavBlockUp   key.Binding // k, ↑
	NavRuneLeft  key.Binding // h
	NavRuneRight key.Binding // l
	NavVisual    key.Binding // v
	NavComment   key.Binding // c
	NavSend      key.Binding // s
	NavCompose   key.Binding // n
	NavRefresh   key.Binding // r
	NavQuit      key.Binding // q

	// Compose
	Newline         key.Binding // enter
	Cancel          key.Binding // esc
	IncludeComments key.Binding // i

	// Comment composer (Enter saves the new comment AND sends every
	// accumulated comment in one batch via the inject pipeline).
	SaveComment key.Binding // enter

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

	// Nav — help-overlay only; rune matching lives in handleNavKey.
	NavBlockDown: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "next block"),
	),
	NavBlockUp: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "previous block"),
	),
	NavRuneLeft: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "rune left"),
	),
	NavRuneRight: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "rune right"),
	),
	NavVisual: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "visual mode"),
	),
	NavComment: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment composer"),
	),
	NavSend: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "send all comments"),
	),
	NavCompose: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "compose mode"),
	),
	NavRefresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "re-poll session"),
	),
	NavQuit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit pinky"),
	),

	// Compose
	Newline: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "newline"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	IncludeComments: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "include comments"),
	),

	// Comment composer (Enter saves the new comment AND sends every
	// accumulated comment in one batch via the inject pipeline).
	SaveComment: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "save & send all"),
	),

	// Error
	QuitError: key.NewBinding(
		key.WithKeys("enter", "esc", "ctrl+c"),
		key.WithHelp("⏎", "dismiss"),
	),
}