package render

import "strings"

// MarkLines splits content into source lines and returns a
// `[]bool` parallel to the lines. The bool is true for any line
// covered by a file-kind comment's LineStart..End range.
// Block-kind comments are ignored. Index in the slice matches the
// 1-based line number (slice[0] == line 1).
func MarkLines(content string, comments []Comment) []bool {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]bool, len(lines))
	for _, c := range comments {
		if c.Kind != CommentFile || c.LineStart <= 0 {
			continue
		}
		for ln := c.LineStart; ln <= c.LineEnd && ln <= len(lines); ln++ {
			out[ln-1] = true
		}
	}
	return out
}
