package render

import "strings"

// FileLine maps a single rendered line index to its 1-based source
// line number and whether any file-kind comment's line range
// covers it (inline char selections are not surfaced as line-level
// highlights in v0).
type FileLine struct {
	LineNo    int
	HasComment bool
}

// MarkLines splits content into source lines and flips HasComment
// on every line covered by a file-kind comment's LineStart..End
// range. content is split on '\n'; a trailing empty line from a
// final newline is dropped. Block-kind comments are ignored.
func MarkLines(content string, comments []Comment) []FileLine {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]FileLine, len(lines))
	covered := make(map[int]bool, len(lines))
	for _, c := range comments {
		if c.Kind != CommentFile || c.LineStart <= 0 {
			continue
		}
		for ln := c.LineStart; ln <= c.LineEnd && ln <= len(lines); ln++ {
			covered[ln] = true
		}
	}
	for i := range out {
		out[i] = FileLine{LineNo: i + 1, HasComment: covered[i+1]}
	}
	return out
}
