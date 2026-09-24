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
//	- file "<path>" (lines X-Y): <text>
//	- file-inline "<excerpt>" (line Z): <text>
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
		marker, excerpt, lineRange := formatEntry(c, blocks)

		b.WriteString("- ")
		b.WriteString(marker)
		b.WriteString(" \"")
		b.WriteString(excerpt)
		b.WriteString("\"")
		b.WriteString(lineRange)
		b.WriteString(": ")
		b.WriteString(c.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

// formatEntry returns the marker, excerpt, and "(lines ...)" /
// "(line ...)" segment for a single comment. Block-kind comments
// pull their excerpt + line range from the rendered message's
// blocks; file-kind comments use the comment's own Path and line
// fields. The lineRange is "" when no range is available.
func formatEntry(c Comment, blocks []Block) (marker, excerpt, lineRange string) {
	inline := c.IsInlineSelection()
	var b strings.Builder
	switch c.Kind {
	case CommentFile:
		if inline {
			marker = "file-inline"
			excerpt = flatten(c.Source)
		} else {
			marker = "file"
			excerpt = c.Path
		}
		if c.LineStart > 0 {
			if c.LineStart == c.LineEnd {
				b.WriteString(" (line ")
				b.WriteString(strconv.Itoa(c.LineStart))
				b.WriteString(")")
			} else {
				b.WriteString(" (lines ")
				b.WriteString(strconv.Itoa(c.LineStart))
				b.WriteString("-")
				b.WriteString(strconv.Itoa(c.LineEnd))
				b.WriteString(")")
			}
		}
	default: // CommentBlock (zero value preserves legacy behaviour)
		if inline {
			marker = "inline"
		} else {
			marker = "block"
		}
		excerpt = c.Source
		if !inline && c.BlockIdx >= 0 && c.BlockIdx < len(blocks) {
			excerpt = blocks[c.BlockIdx].Source
		}
		excerpt = flatten(excerpt)
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
	}
	excerpt = truncate(excerpt, excerptLimit)
	return marker, excerpt, b.String()
}

// flatten trims and collapses internal newlines / tabs so the
// excerpt fits on one line per the appendix contract.
func flatten(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}