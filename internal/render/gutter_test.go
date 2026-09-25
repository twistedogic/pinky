package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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
	out, _ := InjectGutterWrapped(rendered, blocks, 0, 0)
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
	out, _ := InjectGutterWrapped(rendered, blocks, -1, 0)
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
	out, _ := InjectGutterWrapped(rendered, blocks, 0, 0)
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
	out, _ := InjectGutterWrapped(rendered, blocks, 1, 0) // focused=1, the commented block
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
	out, _ := InjectGutterWrapped(rendered, blocks, -1, 0)
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

// TestGutter_SmokeCheck exercises the combined visual contract: a
// focused+commented block emits cyan gutter (not yellow), and an
// unfocused commented block emits yellow gutter.
func TestGutter_SmokeCheck(t *testing.T) {
	md := "# A\n\nbody\n\n## B\n\nmore\n"
	rendered, blocks := RenderMessageWithComments(md, []Comment{
		{BlockIdx: 0, CharStart: -1, CharEnd: -1, Text: "x"},
	})
	if len(blocks) < 2 {
		t.Fatalf("setup: need >= 2 blocks, got %d", len(blocks))
	}
	if !blocks[0].HasComment {
		t.Fatal("setup: block 0 should have HasComment=true")
	}
	// Focus block 0 (commented): cyan should win over yellow.
	out, _ := InjectGutterWrapped(rendered, blocks, 0, 0)
	if !strings.Contains(out, gutterCyanANSI+"▍"+gutterReset) {
		t.Errorf("focused+commented block should still have cyan gutter; got:\n%s", out)
	}
	if strings.Contains(out, gutterYellowANSI+"▍"+gutterReset) {
		t.Errorf("focused+commented block should NOT have yellow gutter (cyan wins); got:\n%s", out)
	}
	// Focus block 1 (uncommented): block 0 (commented, unfocused) should
	// show yellow gutter; block 1 should show cyan.
	out2, _ := InjectGutterWrapped(rendered, blocks, 1, 0)
	if !strings.Contains(out2, gutterYellowANSI+"▍"+gutterReset) {
		t.Errorf("unfocused commented block should still show yellow gutter; got:\n%s", out2)
	}
}

// TestInjectGutterWrapped_FocusedCarriesToWrappedLines: when a
// source line word-wraps into multiple visual lines, EVERY wrapped
// line must carry the focused block's cyan gutter. The wrapped
// line index → source line index map must agree with line counts.
func TestInjectGutterWrapped_FocusedCarriesToWrappedLines(t *testing.T) {
	rendered := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon"
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 0, HasComment: false}}
	out, w2s := InjectGutterWrapped(rendered, blocks, 0, 10)
	lines := strings.Split(out, "\n")
	if len(lines) != len(w2s) {
		t.Fatalf("output line count %d != wrapped→source map length %d", len(lines), len(w2s))
	}
	// Every wrapped line of source line 0 must carry the cyan
	// gutter (since focused=0 maps to the single block).
	for i, line := range lines {
		if !strings.Contains(line, gutterCyanANSI+"▍"+gutterReset) {
			t.Errorf("wrapped line %d (source=%d) missing cyan gutter; got %q",
				i, w2s[i], line)
		}
	}
	// The map must point every wrapped line at source line 0.
	for i, src := range w2s {
		if src != 0 {
			t.Errorf("wrapped line %d mapped to source %d, want 0", i, src)
		}
	}
	// And the wrap actually fired: many wrapped lines for one source.
	if len(lines) < 3 {
		t.Errorf("expected word wrap to expand one line to many; got %d", len(lines))
	}
}

// TestInjectGutterWrapped_YellowCarriesToWrappedLines: a commented
// block's yellow gutter must appear on every wrapped line, even
// though the block is unfocused.
func TestInjectGutterWrapped_YellowCarriesToWrappedLines(t *testing.T) {
	rendered := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen"
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 0, HasComment: true}}
	out, _ := InjectGutterWrapped(rendered, blocks, -1, 8)
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if !strings.Contains(line, gutterYellowANSI+"▍"+gutterReset) {
			t.Errorf("wrapped line %d missing yellow gutter; got %q", i, line)
		}
	}
}

// TestInjectGutterWrapped_LineWidthWithinBudget: after wrapping
// every visual line must fit within wrapWidth (no truncation).
func TestInjectGutterWrapped_LineWidthWithinBudget(t *testing.T) {
	rendered := "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor"
	blocks := []Block{{Kind: BlockParagraph, StartLine: 0, EndLine: 0, HasComment: false}}
	out, _ := InjectGutterWrapped(rendered, blocks, -1, 12)
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > 12 {
			t.Errorf("wrapped line %d width=%d exceeds budget=12: %q", i, w, line)
		}
	}
}
