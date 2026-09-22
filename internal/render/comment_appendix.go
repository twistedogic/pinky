package render

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// excerptLimit is the cap on the inline excerpt shown in the
// appendix and footnote lines; values past it are truncated with an
// ellipsis.
const excerptLimit = 40

// truncate returns the first n runes of s, appending "…" when the
// input is longer. Used for both the appendix and footnote excerpts.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// FormatCommentsAppendix builds the appendix body string for a
// set of comments. Empty comments slice → empty string.
//
// Layout:
//
//	N comments:
//	- block "<excerpt>" (lines X-Y): <text>
//	- inline "<excerpt>" (line Z): <text>
//
// No leading separator: the caller composes "\n\n---\n" when (and
// only when) there is preceding prose to separate from. The
// comment-only flush path (submitAllComments) calls this function
// directly and gets a body-only payload, so the agent never sees a
// naked "---" as its first line.
func FormatCommentsAppendix(comments []Comment, blocks []Block) string {
	if len(comments) == 0 {
		return ""
	}
	sorted := slices.Clone(comments)
	slices.SortFunc(sorted, func(a, b Comment) int {
		return cmp.Compare(a.CreatedAt.UnixNano(), b.CreatedAt.UnixNano())
	})
	var b strings.Builder
	label := "comments"
	if len(sorted) == 1 {
		label = "comment"
	}
	b.WriteString(strconv.Itoa(len(sorted)))
	b.WriteString(" ")
	b.WriteString(label)
	b.WriteString(":\n")
	for _, c := range sorted {
		inline := c.CharStart >= 0
		marker := "block"
		if inline {
			marker = "inline"
		}
		excerpt := c.Source
		if !inline && c.BlockIdx >= 0 && c.BlockIdx < len(blocks) {
			excerpt = blocks[c.BlockIdx].Source
		}
		// Flatten newlines/tabs so the excerpt stays on one line —
		// the appendix contract is "- marker \"excerpt\" (lines): text\n".
		excerpt = strings.TrimSpace(strings.NewReplacer("\n", " ", "\t", " ").Replace(excerpt))
		excerpt = truncate(excerpt, excerptLimit)

		b.WriteString("- ")
		b.WriteString(marker)
		b.WriteString(" \"")
		b.WriteString(excerpt)
		b.WriteString("\"")
		if c.BlockIdx >= 0 && c.BlockIdx < len(blocks) {
			if inline {
				b.WriteString(" (line ")
				b.WriteString(strconv.Itoa(blocks[c.BlockIdx].StartLine))
				b.WriteString(")")
			} else {
				b.WriteString(" (lines ")
				b.WriteString(strconv.Itoa(blocks[c.BlockIdx].StartLine))
				b.WriteString("-")
				b.WriteString(strconv.Itoa(blocks[c.BlockIdx].EndLine))
				b.WriteString(")")
			}
		}
		b.WriteString(": ")
		b.WriteString(c.Text)
		b.WriteByte('\n')
	}
	return b.String()
}