package main

import (
	"charm.land/bubbles/v2/key"
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
	Tab  key.Binding // tab — toggle message / file review tab

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
	NavWordRight key.Binding // w
	NavWordLeft  key.Binding // b
	NavVisual    key.Binding // v
	NavComment   key.Binding // c
	NavSend      key.Binding // s
	NavCompose   key.Binding // n
	NavRefresh   key.Binding // r
	NavQuit      key.Binding // q

	// File nav (stateFileNav).
	FileNavUp      key.Binding // k, ↑
	FileNavDown    key.Binding // j, ↓
	FileNavCollapse key.Binding // h
	FileNavExpand   key.Binding // l
	FileNavOpen    key.Binding // enter
	FileNavComment key.Binding // c
	FileNavSearch  key.Binding // /
	FileNavBack    key.Binding // esc

	// File view (stateFileView).
	FileViewUp         key.Binding // k, ↑
	FileViewDown       key.Binding // j, ↓
	FileViewLeft       key.Binding // h
	FileViewRight      key.Binding // l
	FileViewWordRight  key.Binding // w
	FileViewWordLeft   key.Binding // b
	FileViewVisual     key.Binding // v
	FileViewComment    key.Binding // c
	FileViewDefinition key.Binding // d
	FileViewReferences key.Binding // R
	FileViewHover      key.Binding // K
	FileViewBack       key.Binding // esc

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
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch tab"),
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
	NavWordRight: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "word forward"),
	),
	NavWordLeft: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "word back"),
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

	// File nav
	FileNavUp: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "up"),
	),
	FileNavDown: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "down"),
	),
	FileNavCollapse: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "collapse / parent"),
	),
	FileNavExpand: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "expand / child"),
	),
	FileNavOpen: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "open / toggle"),
	),
	FileNavComment: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment file"),
	),
	FileNavSearch: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	FileNavBack: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),

	// File view
	FileViewUp: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "up"),
	),
	FileViewDown: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "down"),
	),
	FileViewLeft: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "rune left"),
	),
	FileViewRight: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "rune right"),
	),
	FileViewWordRight: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "word forward"),
	),
	FileViewWordLeft: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "word back"),
	),
	FileViewVisual: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "visual"),
	),
	FileViewComment: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment line"),
	),
	FileViewBack: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	FileViewDefinition: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "def word"),
	),
	FileViewReferences: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refs word"),
	),
	FileViewHover: key.NewBinding(
		key.WithKeys("K"),
		key.WithHelp("K", "hover word"),
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

	// Comment composer (Enter saves the new comment and returns to
	// nav without sending; `s` in nav flushes the accumulated batch).
	SaveComment: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "save"),
	),

	// Error
	QuitError: key.NewBinding(
		key.WithKeys("enter", "esc", "ctrl+c"),
		key.WithHelp("⏎", "dismiss"),
	),
}