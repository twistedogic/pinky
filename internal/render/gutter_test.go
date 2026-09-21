package render

import (
	"strings"
	"testing"
)

const (
	gutterCyanANSI   = "\x1b[38;5;51m"
	gutterYellowANSI = "\x1b[38;5;228m"
	gutterReset      = "\x1b[0m"
)

func TestGutter_FocusedBlockCyan(t *testing.T) {
	blocks := []Block{
		{Kind: BlockHeading, StartLine: 0, EndLine: 1},
		{Kind: BlockParagraph, StartLine: 2, EndLine: 3},
	}
	rendered := " h0\n h1\n p2\n p3\n"
	out := InjectGutter(rendered, blocks, 0)
	if !strings.Contains(out, gutterCyanANSI+"▍"+gutterReset+" h0") {
		t.Errorf("expected cyan gutter on first line of focused block; got:\n%s", out)
	}
	if !strings.Contains(out, gutterCyanANSI+"▍"+gutterReset+" h1") {
		t.Errorf("expected cyan gutter on second line of focused block; got:\n%s", out)
	}
}

func TestGutter_CommentedBlockYellow(t *testing.T) {
	blocks := []Block{
		{Kind: BlockHeading, StartLine: 0, EndLine: 0},
		{Kind: BlockParagraph, StartLine: 2, EndLine: 3, HasComment: true},
	}
	rendered := " h0\n\n p2\n p3\n"
	out := InjectGutter(rendered, blocks, -1)
	if !strings.Contains(out, gutterYellowANSI+"▍"+gutterReset+" p2") {
		t.Errorf("expected yellow gutter on commented block; got:\n%s", out)
	}
	if !strings.Contains(out, gutterYellowANSI+"▍"+gutterReset+" p3") {
		t.Errorf("expected yellow gutter on second line of commented block; got:\n%s", out)
	}
}

func TestGutter_GapLineSpace(t *testing.T) {
	blocks := []Block{
		{Kind: BlockHeading, StartLine: 0, EndLine: 0},
		{Kind: BlockParagraph, StartLine: 2, EndLine: 3},
	}
	rendered := " h0\n\n p2\n p3\n"
	out := InjectGutter(rendered, blocks, 0)
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("setup: expected >= 2 lines, got %d", len(lines))
	}
	// Gap line = gutter space + empty body = a single space.
	if lines[1] != " " {
		t.Errorf("gap line should be a single space (gutter only); got %q", lines[1])
	}
}

func TestGutter_FocusedWinsOverCommented(t *testing.T) {
	blocks := []Block{
		{Kind: BlockHeading, StartLine: 0, EndLine: 0},
		{Kind: BlockParagraph, StartLine: 2, EndLine: 3, HasComment: true},
	}
	rendered := " h0\n\n p2\n p3\n"
	out := InjectGutter(rendered, blocks, 1) // focused=1, the commented block
	if !strings.Contains(out, gutterCyanANSI+"▍"+gutterReset+" p2") {
		t.Errorf("focused+commented block should get cyan gutter; got:\n%s", out)
	}
	if strings.Contains(out, gutterYellowANSI+"▍"+gutterReset+" p2") {
		t.Errorf("focused+commented block must NOT also emit yellow gutter; got:\n%s", out)
	}
}

func TestGutter_NoFocusedNoCommentedEmitsOnlySpace(t *testing.T) {
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 1}}
	rendered := " p0\n p1\n"
	out := InjectGutter(rendered, blocks, -1)
	if strings.Contains(out, "▍") {
		t.Errorf("no focused and no commented should produce no ▍; got:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			t.Errorf("line %d should start with a space gutter; got %q", i, line)
		}
	}
}

// TestGutter_SmokeCheck is the one-line visual smoke check from the
// proposal: a focused block whose HasComment is true renders with
// cyan (not yellow) gutter, a non-focused commented block renders
// with yellow gutter, and headings render without `#` prefixes.
// Regression guard for the combined visual contract.
func TestGutter_SmokeCheck(t *testing.T) {
	md := "# A\n\nbody\n\n## B\n\nmore\n"
	rendered, blocks := RenderMessageWithComments(md, 80, []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x"},
	})
	if len(blocks) < 2 {
		t.Fatalf("setup: need >= 2 blocks, got %d", len(blocks))
	}
	if !blocks[0].HasComment {
		t.Fatal("setup: block 0 should have HasComment=true")
	}
	// Focus block 0 (commented): cyan should win over yellow.
	out := InjectGutter(rendered, blocks, 0)
	if !strings.Contains(out, gutterCyanANSI+"▍"+gutterReset) {
		t.Errorf("focused+commented block should still have cyan gutter; got:\n%s", out)
	}
	if strings.Contains(out, gutterYellowANSI+"▍"+gutterReset) {
		t.Errorf("focused+commented block should NOT have yellow gutter (cyan wins); got:\n%s", out)
	}
	// Focus block 1 (uncommented): block 0 (commented, unfocused) should
	// show yellow gutter; block 1 should show cyan.
	out2 := InjectGutter(rendered, blocks, 1)
	if !strings.Contains(out2, gutterYellowANSI+"▍"+gutterReset) {
		t.Errorf("unfocused commented block should still show yellow gutter; got:\n%s", out2)
	}
	// Headings rendered without `#` prefix — strip ANSI and inspect.
	plain := stripANSI(rendered)
	for _, line := range strings.Split(plain, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("heading line should NOT begin with '#'; got %q", line)
		}
	}
}
