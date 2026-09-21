package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// borderStyleForTest mirrors the production borderStyle so the test
// can color the heavy borders identically.
var borderStyleForTest = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

// injectBorderForTest replicates the model's border injection logic
// so the line-rewriting can be tested without a full bubbletea model.
func injectBorderForTest(rendered string, blocks []Block, current int, width int) string {
	if current < 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	border := borderStyleForTest.Render(strings.Repeat("━", width))
	leftBar := borderStyleForTest.Render("┃")
	var b strings.Builder
	for i, line := range lines {
		if i == blocks[current].StartLine {
			b.WriteString(border)
			b.WriteByte('\n')
		}
		if i >= blocks[current].StartLine && i <= blocks[current].EndLine {
			if !HasCommentGutter(line) {
				line = leftBar + " " + TrimLeadingVisible(line, 2)
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if i == blocks[current].EndLine {
			b.WriteString(border)
			b.WriteByte('\n')
		}
	}
	return b.String()
}



func TestBorder_AppearsAboveAndBelowCurrentBlock(t *testing.T) {
	// Three blocks at lines 0-1, 4-5, 8-9 (with 1-line gaps from glamour).
	blocks := []Block{
		{Kind: BlockHeading, StartLine: 0, EndLine: 1},
		{Kind: BlockParagraph, StartLine: 4, EndLine: 5},
		{Kind: BlockCode, StartLine: 8, EndLine: 9},
	}
	// Each line has glamour's 2-space margin.
	rendered := "  h0\n  h1\n\n  gap\n  p3\n  p4\n\n  gap\n  c6\n  c7\n"
	out := injectBorderForTest(rendered, blocks, 1, 10)
	plain := stripANSIForBorderTest(out)
	if !strings.Contains(plain, "━━━━━━━━━━\n┃ p3\n┃ p4\n━━━━━━━━━━") {
		t.Errorf("expected heavy border+left bar around block 1, got:\n%s", plain)
	}
}

func TestBorder_UsesHeavyHorizontalCharacter(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 0}}
	rendered := "body\n"
	out := injectBorderForTest(rendered, blocks, 0, 5)
	if !strings.Contains(out, "━━━━━") {
		t.Errorf("expected heavy horizontal border (━); got:\n%s", out)
	}
	// Regression guard: light horizontal (─) should NOT be used.
	if strings.Contains(out, "─────") {
		t.Errorf("light horizontal (─) should no longer be used; got:\n%s", out)
	}
}

func TestBorder_AddsLeftVerticalBarToFocusedBlock(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 1}}
	// Each line has glamour's 2-space margin.
	rendered := "  p0\n  p1\n"
	out := injectBorderForTest(rendered, blocks, 0, 5)
	plain := stripANSIForBorderTest(out)
	if !strings.Contains(plain, "┃ p0") {
		t.Errorf("expected ┃ prefix on line 0; got:\n%s", plain)
	}
	if !strings.Contains(plain, "┃ p1") {
		t.Errorf("expected ┃ prefix on line 1; got:\n%s", plain)
	}
}

func TestBorder_LeftBarPreservesCommentGutter(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 1}}
	// Line 0 has the comment gutter marker (▸); line 1 is plain.
	rendered := "\x1b[38;5;228m▸\x1b[0m content\n  more\n"
	out := injectBorderForTest(rendered, blocks, 0, 10)
	plain := stripANSIForBorderTest(out)
	// Line 0: gutter kept, no left bar.
	if !strings.Contains(plain, "▸ content") {
		t.Errorf("expected comment gutter preserved on line 0; got:\n%s", plain)
	}
	// Line 1: left bar added (no gutter).
	if !strings.Contains(plain, "┃ more") {
		t.Errorf("expected left bar on line 1; got:\n%s", plain)
	}
}

func TestBorder_NoCurrentReturnsOriginal(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 1}}
	rendered := "line0\nline1\n"
	out := injectBorderForTest(rendered, blocks, -1, 5)
	if out != rendered {
		t.Errorf("expected unchanged, got %q", out)
	}
}

func TestBorder_MovesWithCurrent(t *testing.T) {
	blocks := []Block{
		{Kind: BlockHeading, StartLine: 0, EndLine: 0},
		{Kind: BlockParagraph, StartLine: 2, EndLine: 2},
	}
	// Each line has glamour's 2-space margin.
	rendered := "  h0\n\n  p2\n"
	out0 := injectBorderForTest(rendered, blocks, 0, 5)
	out1 := injectBorderForTest(rendered, blocks, 1, 5)
	if out0 == out1 {
		t.Errorf("borders should differ between blocks")
	}
	if !strings.Contains(out0, "━━━━━\n┃ h0\n━━━━━") {
		t.Errorf("block 0 border+bar missing: %q", out0)
	}
	if !strings.Contains(out1, "━━━━━\n┃ p2\n━━━━━") {
		t.Errorf("block 1 border+bar missing: %q", out1)
	}
}

func TestTrimLeadingVisible(t *testing.T) {
	// No ANSI: drops first n chars.
	if got := TrimLeadingVisible("hello", 2); got != "llo" {
		t.Errorf("plain trim: got %q want %q", got, "llo")
	}
	// With ANSI prefix: drops first n visible chars but keeps escape codes.
	if got := TrimLeadingVisible("\x1b[31mab", 2); got != "\x1b[31m" {
		t.Errorf("ANSI trim: got %q want %q", got, "\x1b[31m")
	}
	// n > visible length: returns empty (escape codes preserved).
	if got := TrimLeadingVisible("\x1b[31mab", 5); got != "\x1b[31m" {
		t.Errorf("over-trim: got %q want %q", got, "\x1b[31m")
	}
}

func TestHasCommentGutter(t *testing.T) {
	cases := map[string]bool{
		"\x1b[38;5;228m▸\x1b[0m content": true,
		"\x1b[38;5;228m•\x1b[0m content": true,
		"  content":                       false,
		"\x1b[31mred text":                 false,
	}
	for in, want := range cases {
		if got := HasCommentGutter(in); got != want {
			t.Errorf("HasCommentGutter(%q) = %v want %v", in, got, want)
		}
	}
}

// stripANSIForBorderTest removes ANSI escape codes for plain-text assertions.
func stripANSIForBorderTest(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
