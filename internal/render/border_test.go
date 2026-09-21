package render

import (
	"strings"
	"testing"
)

// injectBorderForTest replicates the model's border injection logic so
// the line-rewriting can be tested without a full bubbletea model.
func injectBorderForTest(rendered string, blocks []Block, current int, width int) string {
	if current < 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	border := strings.Repeat("─", width)
	var b strings.Builder
	for i, line := range lines {
		if i == blocks[current].StartLine {
			b.WriteString(border)
			b.WriteByte('\n')
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
	rendered := "h0\nh1\n\ngap\np3\np4\n\ngap\nc6\nc7\n"
	out := injectBorderForTest(rendered, blocks, 1, 10)
	if !strings.Contains(out, "──────────\np3\np4\n──────────") {
		t.Errorf("expected border around block 1, got:\n%s", out)
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
	rendered := "h0\n\np2\n"
	out0 := injectBorderForTest(rendered, blocks, 0, 5)
	out1 := injectBorderForTest(rendered, blocks, 1, 5)
	if out0 == out1 {
		t.Errorf("borders should differ between blocks")
	}
	if !strings.Contains(out0, "─────\nh0\n─────") {
		t.Errorf("block 0 border missing: %q", out0)
	}
	if !strings.Contains(out1, "─────\np2\n─────") {
		t.Errorf("block 1 border missing: %q", out1)
	}
}
