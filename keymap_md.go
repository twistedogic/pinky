package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// keymapMarkdown returns a markdown document describing the keys
// available in the given model state. Rendered through glamour so
// it shares the pinky style with the agent message view.
//
// Uses a one-line-per-binding layout (key + description) instead of
// tables — tables don't render gracefully when the viewport is
// narrower than the table width.
//
// Each binding's WithHelp() text supplies the key + description,
// so the markdown tracks the keymap.go definitions automatically.
func keymapMarkdown(s state) string {
	var b strings.Builder
	b.WriteString("# pinky keymap\n\n")

	groups := keymapGroupsForState(s)
	for _, g := range groups {
		b.WriteString("**")
		b.WriteString(g.title)
		b.WriteString("**\n\n")
		for _, kb := range g.bindings {
			help := kb.Help()
			b.WriteString("- `")
			b.WriteString(help.Key)
			b.WriteString("` — ")
			b.WriteString(help.Desc)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
	b.WriteString("_press any key to dismiss_\n")
	return b.String()
}

type keymapGroup struct {
	title    string
	bindings []key.Binding
}

func keymapGroupsForState(s state) []keymapGroup {
	switch s {
	case statePicking:
		return []keymapGroup{
			{title: "navigation", bindings: []key.Binding{
				defaultKeyMap.Up, defaultKeyMap.Down,
			}},
			{title: "select", bindings: []key.Binding{
				defaultKeyMap.Pick,
			}},
			{title: "exit", bindings: []key.Binding{
				defaultKeyMap.QuitPick, defaultKeyMap.Help,
			}},
		}
	case stateIdle:
		return []keymapGroup{
			{title: "navigation", bindings: []key.Binding{
				defaultKeyMap.LineDown, defaultKeyMap.LineUp,
				defaultKeyMap.NextBlock, defaultKeyMap.PrevBlock,
				defaultKeyMap.BottomLine,
			}},
			{title: "compose", bindings: []key.Binding{
				defaultKeyMap.Compose,
			}},
			{title: "session", bindings: []key.Binding{
				defaultKeyMap.Refresh,
			}},
			{title: "exit", bindings: []key.Binding{
				defaultKeyMap.QuitIdle, defaultKeyMap.Help,
			}},
		}
	case stateCompose:
		return []keymapGroup{
			{title: "send", bindings: []key.Binding{
				defaultKeyMap.Send,
			}},
			{title: "input", bindings: []key.Binding{
				defaultKeyMap.Newline,
			}},
			{title: "cancel", bindings: []key.Binding{
				defaultKeyMap.Cancel, defaultKeyMap.Help,
			}},
		}
	case stateError:
		return []keymapGroup{
			{title: "dismiss", bindings: []key.Binding{
				defaultKeyMap.QuitError, defaultKeyMap.Help,
			}},
		}
	}
	return nil
}

// Compile-time guard: tea.KeyMsg stays imported for any future
// helper signatures that need it.
var _ tea.KeyMsg

// Compile-time guard: keep key.Binding reachable from this file.
var _ key.Binding

// Compile-time guard for unused-import suppression when groups are
// empty (e.g. an unknown state falls through to nil).
var _ = []keymapGroup(nil)
