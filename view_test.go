package main

import (
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
// cwd, the header collapses to a single padded line that contains
// the abbreviated cwd and the pending count "0".
func TestHeader_RendersSingleLineWhenFits(t *testing.T) {
	m := newModel()
	m.fileRoot = "/Users/a012/Dev/pinky"
	m.comments = nil
	m.width = 80

	got := m.headerView()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("header should render single line at width 80; got %d lines:\n%s", len(lines), got)
	}
	line := lines[0]
	if !strings.Contains(line, "Dev/pinky") {
		t.Errorf("header should contain abbreviated cwd; got %q", line)
	}
	if !strings.Contains(line, "0") {
		t.Errorf("header should contain pending count 0; got %q", line)
	}
	if w := ansi.StringWidth(line); w != 80 {
		t.Errorf("header line width = %d want 80\n  line: %q", w, line)
	}
}

// TestView_HeaderPresentInAttachedStates: View() in stateNav puts
// the header as the first rendered line at width 80. The header
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
	if len(lines) == 0 {
		t.Fatal("View().Content is empty")
	}
	first := lines[0]
	if !strings.Contains(first, "Dev/pinky") {
		t.Errorf("first line should contain cwd; got %q", first)
	}
	if strings.Contains(first, "%12") {
		t.Errorf("first line should not contain pane id (moved to status line); got %q", first)
	}
	if w := ansi.StringWidth(first); w != m.width {
		t.Errorf("first line width = %d want %d\n  line: %q", w, m.width, first)
	}
}

// TestReflow_ShrinksViewportByHeaderHeight: when the header wraps
// to 2 lines (forced by a single-segment cwd longer than width),
// reflow() subtracts the full headerHeight from vpHeight.
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

	if m.headerHeight != 2 {
		t.Errorf("headerHeight = %d want 2 (forced wrap)", m.headerHeight)
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
		{BlockIdx: 0, Text: "c1", CreatedAt: time.Now()},
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
		{BlockIdx: 0, Text: "c1", CreatedAt: time.Now()},
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
