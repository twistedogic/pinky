package render

import "strings"

// FileLine tracks whether a source line falls inside any file-kind
// comment's line range. Index in the slice matches the 1-based line
// number (slice[0] == line 1). Block-kind comments are ignored.
type FileLine struct {
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
	for _, c := range comments {
		if c.Kind != CommentFile || c.LineStart <= 0 {
			continue
		}
		for ln := c.LineStart; ln <= c.LineEnd && ln <= len(lines); ln++ {
			out[ln-1].HasComment = true
		}
	}
	return out
}
