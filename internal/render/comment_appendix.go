package render

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

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
//
// Excerpt is the first ~40 chars of the block source (block-level)
// or the inline Source (inline), with trailing ellipsis on
// truncation. Comments are emitted in CreatedAt order
// (chronological).
func FormatCommentsAppendix(comments []Comment, blocks []Block) string {
	if len(comments) == 0 {
		return ""
	}
	// Sort by CreatedAt ascending so the appendix reads chronologically.
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
		marker := "block"
		if c.CharStart >= 0 {
			marker = "inline"
		}
		var excerpt string
		if c.CharStart >= 0 {
			excerpt = c.Source
		} else if c.BlockIdx >= 0 && c.BlockIdx < len(blocks) {
			excerpt = blocks[c.BlockIdx].Source
		}
		excerpt = strings.TrimSpace(excerpt)
		// Flatten newlines/tabs so the excerpt stays on one line —
		// the appendix contract is "- marker \"excerpt\" (lines): text\n".
		excerpt = strings.NewReplacer("\n", " ", "\t", " ").Replace(excerpt)
		excerpt = strings.TrimSpace(excerpt)
		if len(excerpt) > 40 {
			excerpt = excerpt[:40] + "…"
		}
		b.WriteString("- ")
		b.WriteString(marker)
		b.WriteString(" \"")
		b.WriteString(excerpt)
		b.WriteString("\"")
		if c.BlockIdx >= 0 && c.BlockIdx < len(blocks) {
			if c.CharStart < 0 {
				b.WriteString(" (lines ")
				b.WriteString(strconv.Itoa(blocks[c.BlockIdx].StartLine))
				b.WriteString("-")
				b.WriteString(strconv.Itoa(blocks[c.BlockIdx].EndLine))
				b.WriteString(")")
			} else {
				b.WriteString(" (line ")
				b.WriteString(strconv.Itoa(blocks[c.BlockIdx].StartLine))
				b.WriteString(")")
			}
		}
		b.WriteString(": ")
		b.WriteString(c.Text)
		b.WriteByte('\n')
	}
	return b.String()
}
