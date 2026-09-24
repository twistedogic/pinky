package render

import (
	"fmt"
	"strings"
)

// FileLine maps a single rendered line index to its 1-based source
// line number and the set of comments that anchor it (line-range
// only; inline char selections are not surfaced as line-level
// highlights in v0).
type FileLine struct {
	LineNo    int  // 1-based source line number
	HasComment bool // any file-kind comment's line range covers this line
}

// RenderFile renders content with a right-aligned 1-based line
// number column and a one-character left gutter. Lines that carry
// at least one file-kind comment from comments are highlighted
// with a yellow gutter. Returns the rendered string and a parallel
// []FileLine slice the caller can use for cursor / hit-test logic.
//
// content is split on '\n'; trailing empty line from a final
// newline is dropped (matches the message viewer's behaviour).
// comments that are not file-kind are ignored.
func RenderFile(content string, comments []Comment) (string, []FileLine) {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	marked := markLines(len(lines), comments)
	width := lineNumberWidth(len(lines))
	const yellow = "\x1b[38;5;228m"
	const reset = "\x1b[0m"
	var b strings.Builder
	for i, line := range lines {
		idx := i + 1 // 1-based
		gutter := " "
		if marked[i].HasComment {
			gutter = yellow + "▍" + reset
		}
		fmt.Fprintf(&b, "%s %*d  %s\n", gutter, width, idx, line)
	}
	return strings.TrimRight(b.String(), "\n"), marked
}

// markLines builds a parallel []FileLine and flips HasComment on
// any line covered by a file-kind comment's LineStart..LineEnd
// range. Build a covered-set once instead of nested-loop scanning.
func markLines(n int, comments []Comment) []FileLine {
	out := make([]FileLine, n)
	for i := range out {
		out[i] = FileLine{LineNo: i + 1}
	}
	covered := make(map[int]bool, n)
	for _, c := range comments {
		if c.Kind != CommentFile || c.LineStart <= 0 {
			continue
		}
		for ln := c.LineStart; ln <= c.LineEnd && ln <= n; ln++ {
			covered[ln] = true
		}
	}
	for i := range out {
		out[i].HasComment = covered[out[i].LineNo]
	}
	return out
}

// lineNumberWidth returns the minimum width needed to render any
// line number up to n (inclusive).
func lineNumberWidth(n int) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for x := n; x > 0; x /= 10 {
		w++
	}
	return w
}
