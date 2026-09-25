package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/twistedogic/pinky/internal/session"
)

// TestCommitFirstLineVisible: a non-empty poll commits the latest
// assistant text and pins YOffset to top. The first rendered line
// must be the first sentence.
func TestCommitFirstLineVisible(t *testing.T) {
	src := &pollableSource{messages: []session.Message{
		{Role: session.RoleAssistant, Text: "The first sentence is here.\n\nSecond sentence.", Ts: time.Now()},
	}}
	m := newPollModel(src)

	upd, cmd := m.Update(sessionMsg{entries: src.messages})
	mv := upd.(model)
	m = &mv
	if cmd == nil {
		t.Fatal("first poll should re-arm")
	}
	if mv.latest.Text == "" {
		t.Fatal("after poll: latest.Text should be set")
	}
	if mv.viewport.YOffset() != 0 {
		t.Errorf("YOffset should be 0 on commit; got %d", mv.viewport.YOffset())
	}

	view := mv.viewport.View()
	firstRendered := strings.SplitN(view, "\n", 2)[0]
	if !strings.Contains(firstRendered, "first sentence") {
		t.Errorf("first visible line should contain 'first sentence'; got %q", firstRendered)
	}
}

// newPollModelReflow mirrors newPollModel but goes through reflow()
// so the viewport gets a real height (matching the production flow).
func newPollModelReflow(src *pollableSource) *model {
	m := newPollModel(src)
	m.width = 80
	m.height = 24
	m.textarea = textarea.New()
	m.textarea.SetWidth(80)
	m.textarea.SetHeight(composeHeight)
	m.commentTa = textarea.New()
	m.commentTa.SetWidth(80)
	m.commentTa.SetHeight(1)
	m.reflow()
	m.refreshViewport()
	return m
}

// TestCommitFirstLineVisible_AfterReflow: same as above but with a
// real reflow() so the viewport height is the production value.
func TestCommitFirstLineVisible_AfterReflow(t *testing.T) {
	src := &pollableSource{messages: []session.Message{
		{Role: session.RoleAssistant, Text: "The first sentence is here.\n\nSecond sentence.", Ts: time.Now()},
	}}
	m := newPollModelReflow(src)

	upd, _ := m.Update(sessionMsg{entries: src.messages})
	mv := upd.(model)

	view := mv.viewport.View()
	firstRendered := strings.SplitN(view, "\n", 2)[0]
	if !strings.Contains(firstRendered, "first sentence") {
		t.Errorf("first visible line should contain 'first sentence'; got %q\n--- full view ---\n%s",
			firstRendered, view)
	}
	if mv.viewport.YOffset() != 0 {
		t.Errorf("YOffset should be 0; got %d", mv.viewport.YOffset())
	}
}

// TestCommitFirstLineVisible_FromPlaceholder: viewport starts with
// placeholder, transitions to real content. YOffset should land
// at 0 and the first sentence should be visible.
func TestCommitFirstLineVisible_FromPlaceholder(t *testing.T) {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(22))
	vp.SetContent("waiting for agent…")
	if vp.YOffset() != 0 {
		t.Fatalf("setup: placeholder YOffset should be 0; got %d", vp.YOffset())
	}

	vp.SetContent("The first sentence is here.\n\nSecond sentence.")
	if vp.YOffset() != 0 {
		t.Errorf("after SetContent: YOffset should be 0; got %d", vp.YOffset())
	}
	view := vp.View()
	firstRendered := strings.SplitN(view, "\n", 2)[0]
	if !strings.Contains(firstRendered, "first sentence") {
		t.Errorf("first visible line should contain 'first sentence'; got %q", firstRendered)
	}
}

// TestCommitFirstLineVisible_FullAttachFlow drives the full
// attach → WindowSizeMsg → first poll sequence and verifies the
// first rendered line of the viewport is the first sentence.
func TestCommitFirstLineVisible_FullAttachFlow(t *testing.T) {
	src := &pollableSource{messages: []session.Message{
		{Role: session.RoleAssistant, Text: "The first sentence is here.\n\nSecond sentence.", Ts: time.Now()},
	}}
	m := newPollModelReflow(src)

	upd, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mv := upd.(model)
	upd, _ = mv.Update(sessionMsg{entries: src.messages})
	mv = upd.(model)

	view := mv.viewport.View()
	firstRendered := strings.SplitN(view, "\n", 2)[0]
	if !strings.Contains(firstRendered, "first sentence") {
		t.Errorf("first visible line should contain 'first sentence'; got %q", firstRendered)
	}
}

// TestCommitFirstLineVisible_LongMessage: a long message that
// exceeds viewport height — first line must still be visible.
func TestCommitFirstLineVisible_LongMessage(t *testing.T) {
	src := &pollableSource{messages: []session.Message{
		{Role: session.RoleAssistant, Text: longSentence, Ts: time.Now()},
	}}
	m := newPollModelReflow(src)
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mv := upd.(model)
	upd, _ = mv.Update(sessionMsg{entries: src.messages})
	mv = upd.(model)

	if mv.viewport.YOffset() != 0 {
		t.Errorf("YOffset should be 0; got %d", mv.viewport.YOffset())
	}
	view := mv.viewport.View()
	firstRendered := strings.SplitN(view, "\n", 2)[0]
	if !strings.Contains(firstRendered, "first sentence") {
		t.Errorf("long message: first visible line should contain 'first sentence'; got %q", firstRendered)
	}
}

var longSentence = "The first sentence is here.\n\n" + strings.Repeat("Padding line.\n", 30) + "Last line."

// TestRefreshViewport_WordWraps: a long unwrappable line must be
// word-wrapped at viewport.Width - 1 (leaving room for the
// single-cell gutter). Without reflow, the viewport's ansi.Cut
// would clip the right side of the line.
func TestRefreshViewport_WordWraps(t *testing.T) {
	src := &pollableSource{messages: []session.Message{
		{Role: session.RoleAssistant, Text: "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty.", Ts: time.Now()},
	}}
	m := newPollModelReflow(src)
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	mv := upd.(model)
	upd, _ = mv.Update(sessionMsg{entries: src.messages})
	mv = upd.(model)

	view := mv.viewport.View()
	lines := strings.Split(view, "\n")
	// With Width=40 and a one-cell gutter, content wraps to 39 chars.
	// The 20-word paragraph above must occupy several wrapped lines.
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty < 3 {
		t.Errorf("expected word-wrapped paragraph to span multiple lines; got %d non-empty lines:\n%s",
			nonEmpty, view)
	}
	// No visible line should exceed Width (40) cells.
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("line %d width=%d exceeds viewport Width=40: %q", i, w, l)
		}
	}
}

// TestCommitFirstLineVisible_AfterScroll: every commit pins to
// top via the viewport's own GotoTop, regardless of prior scroll
// position. Regression for the "first sentence missing" bug where
// Bubble Tea's SetContent preserves YOffset across calls.
func TestCommitFirstLineVisible_AfterScroll(t *testing.T) {
	src := &pollableSource{messages: []session.Message{
		{Role: session.RoleAssistant, Text: longSentence, Ts: time.Now()},
	}}
	m := newPollModelReflow(src)
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mv := upd.(model)
	upd, _ = mv.Update(sessionMsg{entries: src.messages})
	mv = upd.(model)

	maxOff := mv.viewport.TotalLineCount() - mv.viewport.Height()
	if maxOff < 3 {
		t.Fatalf("test fixture too short; maxOff=%d", maxOff)
	}
	mv.viewport.SetYOffset(maxOff - 2)
	if mv.viewport.YOffset() <= 0 {
		t.Fatalf("setup: should be scrolled; YOffset=%d", mv.viewport.YOffset())
	}

	secondText := "Brand new message: first sentence.\n\n" +
		strings.Repeat("padding line.\n", 30) + "tail."
	src.messages = append(src.messages,
		session.Message{Role: session.RoleAssistant, Text: secondText, Ts: time.Now()})
	src.offset = 1

	upd, _ = mv.Update(sessionMsg{entries: []session.Message{
		{Role: session.RoleAssistant, Text: secondText},
	}})
	mv = upd.(model)

	if mv.viewport.YOffset() != 0 {
		t.Errorf("after second commit: YOffset should be 0; got %d", mv.viewport.YOffset())
	}
	view := mv.viewport.View()
	firstRendered := strings.SplitN(view, "\n", 2)[0]
	if !strings.Contains(firstRendered, "Brand new message") {
		t.Errorf("after second commit: first visible line should contain 'Brand new message'; got %q",
			firstRendered)
	}
}