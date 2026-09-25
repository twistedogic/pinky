package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFileView_ResizeKeepsCursorVisible: moving the cursor deep
// into a large file and then shrinking the viewport must keep
// the cursor's line on screen. Covers both the WindowSizeMsg
// path (terminal resize) and the help-toggle path (`?` expands
// the help footer, shrinking the file viewport).
func TestFileView_ResizeKeepsCursorVisible(t *testing.T) {
	t.Run("window-resize-shrinks", func(t *testing.T) {
		for _, h := range []int{6, 8, 12, 20} {
			root, rel := longFileFixture(t, 200)
			m := openFileInFixture(t, root, rel)
			upd, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
			mv := upd.(model)
			m = &mv

			for i := 0; i < 100; i++ {
				upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
				mv = upd.(model)
				m = &mv
			}
			cursor := m.fileViewer.cursor
			upd, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: h})
			mv = upd.(model)
			m = &mv
			top := m.fileViewer.viewport.YOffset
			bot := top + m.fileViewer.viewport.Height - 1
			if cursor-1 < top || cursor-1 > bot {
				t.Errorf("h=%d cursor=%d outside viewport [%d..%d]",
					h, cursor, top, bot)
			}
		}
	})

	t.Run("help-toggle-shrinks", func(t *testing.T) {
		root, rel := longFileFixture(t, 200)
		m := openFileInFixture(t, root, rel)
		upd, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
		mv := upd.(model)
		m = &mv

		for i := 0; i < 100; i++ {
			upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
			mv = upd.(model)
			m = &mv
		}
		cursor := m.fileViewer.cursor
		upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		mv = upd.(model)
		m = &mv
		top := m.fileViewer.viewport.YOffset
		bot := top + m.fileViewer.viewport.Height - 1
		if cursor-1 < top || cursor-1 > bot {
			t.Errorf("cursor=%d outside viewport [%d..%d] after help toggle",
				cursor, top, bot)
		}
	})
}
