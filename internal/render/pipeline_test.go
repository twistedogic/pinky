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

// visibleLen keeps the historical name; returns the rune count of
// the ANSI-stripped string.
func visibleLen(s string) int {
	return len([]rune(stripANSI(s)))
}

// TestPipeline_HeadingParaCode exercises parse → slice against a
// representative agent message. Raw rendering: every block's source
// appears verbatim in the output.
func TestPipeline_HeadingParaCode(t *testing.T) {
	md := "# Setup\n\nRead the file and parse it.\n\n```\nfunc main() {}\n```\n"
	rendered, blocks := renderBlocks(md)
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

// TestPipeline_EmptyMessage: empty input renders to empty output
// with zero blocks.
func TestPipeline_EmptyMessage(t *testing.T) {
	rendered, blocks := renderBlocks("")
	if rendered != "" {
		t.Errorf("empty message should render empty, got %q", rendered)
	}
	if len(blocks) != 0 {
		t.Errorf("empty message should have no blocks, got %d", len(blocks))
	}
}

// TestPipeline_BlockBoundariesNonOverlapping: contiguous blocks
// have non-overlapping StartLine..EndLine ranges.
func TestPipeline_BlockBoundariesNonOverlapping(t *testing.T) {
	md := "# A\n\npara one\n\n## B\n\npara two\n\n- one\n- two\n- three\n"
	_, blocks := renderBlocks(md)
	for i := 0; i < len(blocks)-1; i++ {
		if blocks[i].EndLine >= blocks[i+1].StartLine {
			t.Errorf("block %d (lines %d-%d) overlaps block %d (lines %d-%d)",
				i, blocks[i].StartLine, blocks[i].EndLine,
				i+1, blocks[i+1].StartLine, blocks[i+1].EndLine)
		}
	}
}