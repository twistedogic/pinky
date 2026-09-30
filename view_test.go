package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/twistedogic/pinky/internal/render"
	"github.com/twistedogic/pinky/internal/session"
)

func TestView_PadsPickerToFullWidth(t *testing.T) {
	m := newModel()
	m.agents = []session.AgentSession{
		{Session: "work", Window: "1", Pane: "1", PaneID: "%12", Agent: "codex"},
	}
	m.pickCursor = 0
	m.width = 80
	m.height = 24

	view := m.View()
	for _, line := range strings.Split(view.Content, "\n") {
		if w := ansi.StringWidth(line); w != m.width {
			t.Errorf("line width = %d want %d\n  line: %q", w, m.width, line)
		}
	}
}

func TestView_PadsStatusLineToFullWidth(t *testing.T) {
	m := newModel()
	m.state = stateNav
	m.pane = "%12"
	m.width = 80
	m.height = 24
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	view := m.View()
	lines := strings.Split(view.Content, "\n")
	last := lines[len(lines)-1]
	if w := ansi.StringWidth(last); w != m.width {
		t.Errorf("status line width = %d want %d\n  line: %q", w, m.width, last)
	}
}

// TestHeader_RendersSingleLineWhenFits: at width 80 with a normal
// cwd, the header collapses to a tab row + a single padded cwd
// line that contains the abbreviated cwd and the pending count "0".
func TestHeader_RendersSingleLineWhenFits(t *testing.T) {
	m := newModel()
	m.fileRoot = "/Users/a012/Dev/pinky"
	m.comments = nil
	m.width = 80

	got := m.headerView()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("header should render tab row + cwd row at width 80; got %d lines:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], "Message") || !strings.Contains(lines[0], "Files") {
		t.Errorf("first line should be the tab row; got %q", lines[0])
	}
	cwdLine := lines[1]
	if !strings.Contains(cwdLine, "Dev/pinky") {
		t.Errorf("second line should contain abbreviated cwd; got %q", cwdLine)
	}
	if !strings.Contains(cwdLine, "0") {
		t.Errorf("second line should contain pending count 0; got %q", cwdLine)
	}
	if w := ansi.StringWidth(cwdLine); w != 80 {
		t.Errorf("cwd line width = %d want 80\n  line: %q", w, cwdLine)
	}
}

// TestView_HeaderPresentInAttachedStates: View() in stateNav puts
// the tab row above the cwd header at width 80. The cwd line
// must NOT carry the streaming dot or the pane id — those moved
// to the slimmed status line.
func TestView_HeaderPresentInAttachedStates(t *testing.T) {
	m := newModel()
	m.state = stateNav
	m.pane = "%12"
	m.fileRoot = "/Users/a012/Dev/pinky"
	m.width = 80
	m.height = 24
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	view := m.View()
	lines := strings.Split(view.Content, "\n")
	if len(lines) < 2 {
		t.Fatalf("View().Content should have tab row + cwd row; got %d lines", len(lines))
	}
	tabLine := lines[0]
	if !strings.Contains(ansi.Strip(tabLine), "Message") {
		t.Errorf("first line should be the tab row; got %q", tabLine)
	}
	cwdLine := lines[1]
	if !strings.Contains(cwdLine, "Dev/pinky") {
		t.Errorf("second line should contain cwd; got %q", cwdLine)
	}
	if strings.Contains(cwdLine, "%12") {
		t.Errorf("cwd line should not contain pane id (moved to status line); got %q", cwdLine)
	}
	if w := ansi.StringWidth(cwdLine); w != m.width {
		t.Errorf("cwd line width = %d want %d\n  line: %q", w, m.width, cwdLine)
	}
}

// TestReflow_ShrinksViewportByHeaderHeight: when the cwd header
// wraps to 2 lines (forced by a single-segment cwd longer than
// width), reflow() accounts for the tab row + the 2 cwd lines
// when computing vpHeight.
func TestReflow_ShrinksViewportByHeaderHeight(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateCompose
	m.pane = "%12"
	// Single trailing segment > width 40 — wraps even at maxSegs=1
	// because splitOnSlash still has the leading "~/".
	m.fileRoot = "/a/supercalifragilisticexpialidocious-and-then-some"
	m.width = 40
	m.height = 24
	m.reflow()

	if m.headerHeight != 3 {
		t.Errorf("headerHeight = %d want 3 (tab row + 2-line cwd)", m.headerHeight)
	}
	want := 24 - statusHeight - m.helpHeight() - m.headerHeight - composeHeight - 1
	if m.viewport.Height() != want {
		t.Errorf("viewport.Height() = %d want %d", m.viewport.Height(), want)
	}
}

// TestView_StateCompose_ShowsIncludeChip: the slimmed status line
// still carries the [I] include N comments chip when there are
// pending comments and the toggle is on.
func TestView_StateCompose_ShowsIncludeChip(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateCompose
	m.enterCompose()
	m.comments = []render.Comment{
		{ByteA: 0, ByteC: 0, Text: "c1", CreatedAt: time.Now()},
	}
	m.includeComments = true
	m.width = 80
	m.height = 24
	m.reflow()

	lines := strings.Split(m.View().Content, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "[I] include") {
		t.Errorf("status line should contain [I] include chip; got %q", last)
	}
	if !strings.Contains(last, "1 comments") {
		t.Errorf("status line should include the pending count in the chip; got %q", last)
	}
}

// TestStatusLine_SlimmedOmitsPaneAndCommentCount: the pane id and
// pending comment count moved from the status line to the header.
func TestStatusLine_SlimmedOmitsPaneAndCommentCount(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.pane = "%12"
	m.comments = []render.Comment{
		{ByteA: 0, ByteC: 0, Text: "c1", CreatedAt: time.Now()},
	}
	m.width = 80

	line := m.statusLine()
	if strings.Contains(line, "%12") {
		t.Errorf("status line should not contain pane id; got %q", line)
	}
	if strings.Contains(line, "1 comments") {
		t.Errorf("status line should not contain pending comment count (moved to header); got %q", line)
	}
}

// tabHeaderFirstLine extracts the first non-empty rendered line of
// the view. Used by the tab-header tests to check the row above
// the cwd header.
func tabHeaderFirstLine(t *testing.T, m model) string {
	t.Helper()
	view := m.View()
	lines := strings.Split(view.Content, "\n")
	for _, line := range lines {
		if strings.TrimSpace(ansi.Strip(line)) != "" {
			return line
		}
	}
	return ""
}

// TestTabHeader_PresentInAttachedStates: every attached state
// shows a tab row above the cwd header containing both labels.
func TestTabHeader_PresentInAttachedStates(t *testing.T) {
	attached := []state{
		stateNav, stateCompose, stateCommentComposer,
		stateFileNav, stateFileView, stateLSPPicker,
	}
	for _, st := range attached {
		t.Run(stateName(st), func(t *testing.T) {
			m := newIdleModelForKeymap(t)
			m.state = st
			m.tab = tabMessage
			m.reflow()

			first := tabHeaderFirstLine(t, m)
			stripped := ansi.Strip(first)
			if !strings.Contains(stripped, "Message") {
				t.Errorf("first rendered line should contain 'Message'; got %q", first)
			}
			if !strings.Contains(stripped, "Files") {
				t.Errorf("first rendered line should contain 'Files'; got %q", first)
			}
		})
	}
}

// TestTabHeader_AbsentInPickerAndError: the picker and the error
// screen skip the tab row (no agent yet, no meaningful tab state).
func TestTabHeader_AbsentInPickerAndError(t *testing.T) {
	for _, st := range []state{statePicking, stateError} {
		t.Run(stateName(st), func(t *testing.T) {
			m := newIdleModelForKeymap(t)
			m.state = st
			if st == stateError {
				m.err = errors.New("test")
			}
			m.reflow()

			view := m.View()
			stripped := ansi.Strip(view.Content)
			// The tab header always contains both "Message" and
			// "Files" on the SAME line. For picker / error the
			// string should not contain both labels on any line.
			for _, line := range strings.Split(stripped, "\n") {
				if strings.Contains(line, "Message") && strings.Contains(line, "Files") {
					t.Errorf("%s should not show the tab header; got line %q",
						stateName(st), line)
				}
			}
		})
	}
}

// TestTabHeader_ActiveCellSwapsWithMTab: each render must contain
// the active cell's combined "fg + bg" ANSI escape sequence and
// that escape must be immediately followed by the active label.
func TestTabHeader_ActiveCellSwapsWithMTab(t *testing.T) {
	render := func(tab tab) string {
		m := newIdleModelForKeymap(t)
		m.tab = tab
		m.reflow()
		return tabHeaderFirstLine(t, m)
	}
	// The combined escape emitted for an active cell with both
	// fg and bg set (lipgloss emits fg first, then bg).
	const activeCombined = "\x1b[38;5;232;48;5;51m"

	for _, c := range []struct {
		name   string
		view   string
		active string
	}{
		{"tabMessage", render(tabMessage), "Message"},
		{"tabFiles", render(tabFiles), "Files"},
	} {
		idx := strings.Index(c.view, activeCombined)
		if idx < 0 {
			t.Errorf("%s render missing active combined escape %q; got %q",
				c.name, activeCombined, c.view)
			continue
		}
		after := c.view[idx+len(activeCombined):]
		if !strings.HasPrefix(after, c.active) {
			t.Errorf("%s active combined escape should be immediately followed by %q; got %q",
				c.name, c.active, after)
		}
	}
}

// TestTabHeader_FoldsIntoHeaderHeight: headerHeight accounts for
// the tab row on top of the (1- or 2-line) cwd header.
func TestTabHeader_FoldsIntoHeaderHeight(t *testing.T) {
	// 1-line cwd + tab row.
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.tab = tabMessage
	m.fileRoot = "/Users/a012/Dev/pinky"
	m.width = 80
	m.height = 24
	m.reflow()

	if m.headerHeight != 2 {
		t.Errorf("1-line cwd + tab row: headerHeight = %d, want 2", m.headerHeight)
	}

	// 2-line cwd + tab row: a single-segment cwd that won't fit
	// at width 24 forces the header to split across lines.
	m.fileRoot = "/totallyflatsingle-segmentnameofdreadfulhort"
	m.width = 24
	m.reflow()

	if m.headerHeight != 3 {
		t.Errorf("2-line cwd + tab row: headerHeight = %d, want 3", m.headerHeight)
	}
}

// TestStatusLine_DropsTabChip: the status line no longer carries
// the tab chip (tab-header spec moved the indicator up top).
func TestStatusLine_DropsTabChip(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.tab = tabFiles
	m.width = 80

	line := ansi.Strip(m.statusLine())
	// The streaming dot remains; the chip is gone.
	if !strings.Contains(line, "●") && !strings.Contains(line, "·") {
		t.Errorf("status line should still contain the streaming dot; got %q", line)
	}
	if strings.Contains(line, "files") {
		t.Errorf("status line should not contain 'files' tab chip; got %q", line)
	}
	if strings.Contains(line, "msg") {
		t.Errorf("status line should not contain 'msg' tab chip; got %q", line)
	}
}

// stateName returns a stable, human-readable name for a state
// constant. Test-only.
func stateName(s state) string {
	switch s {
	case statePicking:
		return "statePicking"
	case stateNav:
		return "stateNav"
	case stateCompose:
		return "stateCompose"
	case stateCommentComposer:
		return "stateCommentComposer"
	case stateFileNav:
		return "stateFileNav"
	case stateFileView:
		return "stateFileView"
	case stateLSPPicker:
		return "stateLSPPicker"
	case stateError:
		return "stateError"
	default:
		return "unknown"
	}
}
