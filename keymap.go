package main

import (
	"github.com/charmbracelet/bubbles/key"
)

// keyMap is the declarative set of keybindings used by pinky. Most
// nav keys are matched by handleNavKey reading the rune directly —
// the bindings here are only consumed by key.Matches() for keys that
// the nav state machine doesn't own (compose, comment-composer, error
// dismissal, include-comments toggle). The Nav binding group exists
// purely to populate the FullHelp view.
type keyMap struct {
	// Shared
	Help key.Binding // ?

	// Picker
	Up       key.Binding // k, ↑
	Down     key.Binding // j, ↓
	Pick     key.Binding // enter
	QuitPick key.Binding // q

	// Nav (single-letter surface per design D2; group rendered for
	// the help overlay only — handleNavKey matches runes directly).
	NavGroup key.Binding // j k h l v Esc c s q r n

	// Compose
	Newline         key.Binding // enter
	Cancel          key.Binding // esc
	IncludeComments key.Binding // ctrl+i

	// Comment composer (uses Ctrl+S for save, Esc for cancel; the
	// composer is a multi-line textarea so `s` is a literal there).
	SaveComment key.Binding // ctrl+s

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
	NavGroup: key.NewBinding(
		key.WithKeys("j", "k", "h", "l", "v", "esc", "c", "s", "q", "r", "n"),
		key.WithHelp("nav", "j k h l v c s q r n"),
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
		key.WithKeys("ctrl+i"),
		key.WithHelp("^I", "include comments"),
	),

	// Comment composer
	SaveComment: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("^S", "save"),
	),

	// Error
	QuitError: key.NewBinding(
		key.WithKeys("enter", "esc", "ctrl+c"),
		key.WithHelp("⏎", "dismiss"),
	),
}
