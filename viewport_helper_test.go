package main

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
)

func newViewport80x20() viewport.Model {
	return viewport.New(80, 20)
}

// newTestTextarea builds a textarea with the blink/cursor machinery
// initialized so calling Focus() in tests doesn't panic.
func newTestTextarea() textarea.Model {
	ta := textarea.New()
	ta.SetHeight(composeHeight)
	ta.SetWidth(80)
	ta.Focus() // initialize the cursor; tests will Blur() if needed
	return ta
}
