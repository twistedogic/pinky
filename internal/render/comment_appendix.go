package render

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// excerptLimit is the cap on the inline excerpt shown in the
// appendix; values past it are truncated with an ellipsis.
const excerptLimit = 40

// truncate returns the first n runes of s, appending "…" when the
// input is longer.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// flattenExcerpt prepares a quoted-text excerpt for the appendix:
// internal newlines become a single space, runs of whitespace are
// collapsed to one space, leading / trailing whitespace is trimmed.
// The result is then truncated to excerptLimit chars (with "…") by
// the caller.
func flattenExcerpt(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	// Collapse runs of whitespace to a single space.
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// FormatCommentsAppendix builds the appendix body string for a set of
// comments. Empty comments slice → empty string.
//
// Layout:
//
//	Comments on the message:
//	- comment on "<excerpt>": <text>
//
//	Comments on files:
//	- file "<path>" (lines X-Y): <text>
//	- file-inline "<excerpt>" (line Z): <text>
//
// No leading "---" separator; no count line. Sections with no entries
// are omitted entirely. Two empty sections collapse to a single
// blank line. Entries within each section are chronological (oldest
// first) by CreatedAt.
func FormatCommentsAppendix(comments []Comment) string {
	if len(comments) == 0 {
		return ""
	}
	sorted := slices.Clone(comments)
	slices.SortFunc(sorted, func(a, b Comment) int {
		return cmp.Compare(a.CreatedAt.UnixNano(), b.CreatedAt.UnixNano())
	})

	var msgLines, fileLines []string
	for _, c := range sorted {
		switch c.Kind {
		case CommentMessage:
			msgLines = append(msgLines, formatMessageEntry(c))
		case CommentFile:
			fileLines = append(fileLines, formatFileEntry(c))
		}
	}

	var b strings.Builder
	if len(msgLines) > 0 {
		b.WriteString("Comments on the message:\n")
		for _, l := range msgLines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	if len(fileLines) > 0 {
		if len(msgLines) > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Comments on files:\n")
		for _, l := range fileLines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatMessageEntry renders a single message-kind comment as
// `- comment on "<excerpt>": <text>`. The excerpt is the verbatim
// source slice flattened (newlines → space, whitespace collapsed,
// trimmed) and truncated to excerptLimit chars with "…" appended.
func formatMessageEntry(c Comment) string {
	excerpt := flattenExcerpt(c.Source)
	excerpt = truncate(excerpt, excerptLimit)
	return `- comment on "` + excerpt + `": ` + c.Text
}

// formatFileEntry renders a single file-kind comment in today's
// format: `- file "<path>" (lines X-Y): <text>` for line-range
// comments or `- file-inline "<excerpt>" (line Z): <text>` for
// inline byte-range comments.
func formatFileEntry(c Comment) string {
	inline := c.ByteA > 0 || c.ByteC > 0
	if inline {
		excerpt := flattenExcerpt(c.Source)
		excerpt = truncate(excerpt, excerptLimit)
		if c.LineStart > 0 {
			return `- file-inline "` + excerpt + `" (line ` + strconv.Itoa(c.LineStart) + `): ` + c.Text
		}
		return `- file-inline "` + excerpt + `": ` + c.Text
	}
	if c.LineStart > 0 && c.LineEnd > 0 && c.LineStart != c.LineEnd {
		return `- file "` + c.Path + `" (lines ` + strconv.Itoa(c.LineStart) + `-` + strconv.Itoa(c.LineEnd) + `): ` + c.Text
	}
	if c.LineStart > 0 {
		return `- file "` + c.Path + `" (line ` + strconv.Itoa(c.LineStart) + `): ` + c.Text
	}
	return `- file "` + c.Path + `": ` + c.Text
}