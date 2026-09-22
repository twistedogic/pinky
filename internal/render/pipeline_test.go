package render

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func visibleLen(s string) int {
	return len([]rune(stripANSI(s)))
}

// TestPipeline_HeadingParaCode exercises parse → render → border
// injection against a representative agent message.
func TestPipeline_HeadingParaCode(t *testing.T) {
	md := "# Setup\n\nRead the file and parse it.\n\n```\nfunc main() {}\n```\n"
	rendered, blocks := renderBlocks(md, 80)
	if len(blocks) != 3 {
		t.Fatalf("want 3 blocks, got %d", len(blocks))
	}
	if rendered == "" {
		t.Fatal("rendered output is empty")
	}
	plain := stripANSI(rendered)
	for _, want := range []string{"Setup", "Read the file", "func main"} {
		if !strings.Contains(plain, want) {
			t.Errorf("rendered missing %q", want)
		}
	}
}

// TestPipeline_FillsWidth verifies the rendered markdown content
// uses the requested width, not width-2 (the dark preset's margin).
// Regression guard for the glamour margin compensation in NewRenderer.
func TestPipeline_FillsWidth(t *testing.T) {
	for _, w := range []int{40, 80, 120} {
		rendered, _ := renderBlocks("# Title\n\nBody paragraph.\n", w)
		for i, line := range strings.Split(rendered, "\n") {
			if vw := visibleLen(line); vw > 0 && vw != w {
				t.Errorf("width=%d: line %d width=%d (want %d)\n  line: %q",
					w, i, vw, w, line)
			}
		}
	}
}

func TestPipeline_EmptyMessage(t *testing.T) {
	rendered, blocks := renderBlocks("", 80)
	if rendered != "" {
		t.Errorf("empty message should render empty, got %q", rendered)
	}
	if len(blocks) != 0 {
		t.Errorf("empty message should have no blocks, got %d", len(blocks))
	}
}

func TestPipeline_BlockBoundariesNonOverlapping(t *testing.T) {
	md := "# A\n\npara one\n\n## B\n\npara two\n\n- one\n- two\n- three\n"
	_, blocks := renderBlocks(md, 80)
	for i := 0; i < len(blocks)-1; i++ {
		if blocks[i].EndLine >= blocks[i+1].StartLine {
			t.Errorf("block %d (lines %d-%d) overlaps block %d (lines %d-%d)",
				i, blocks[i].StartLine, blocks[i].EndLine,
				i+1, blocks[i+1].StartLine, blocks[i+1].EndLine)
		}
	}
}
