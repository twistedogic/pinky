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
// internal whitespace collapses to single spaces, leading and
// trailing whitespace is dropped.
//
// ponytail: strings.Fields + Join handles every form of unicode
// whitespace (including \n, \t, \r, NBSP, etc.) in one stdlib call.
func flattenExcerpt(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
// are omitted entirely. Entries within each section are chronological
// (oldest first) by CreatedAt.
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
			excerpt := truncate(flattenExcerpt(c.Source), excerptLimit)
			msgLines = append(msgLines,
				`- comment on "`+excerpt+`": `+c.Text)
		case CommentFile:
			inline := c.ByteA > 0 || c.ByteC > 0
			switch {
			case inline && c.LineStart > 0:
				excerpt := truncate(flattenExcerpt(c.Source), excerptLimit)
				fileLines = append(fileLines,
					`- file-inline "`+excerpt+`" (line `+strconv.Itoa(c.LineStart)+`): `+c.Text)
			case inline:
				excerpt := truncate(flattenExcerpt(c.Source), excerptLimit)
				fileLines = append(fileLines,
					`- file-inline "`+excerpt+`": `+c.Text)
			case c.LineStart > 0 && c.LineEnd > 0 && c.LineStart != c.LineEnd:
				fileLines = append(fileLines,
					`- file "`+c.Path+`" (lines `+strconv.Itoa(c.LineStart)+`-`+strconv.Itoa(c.LineEnd)+`): `+c.Text)
			case c.LineStart > 0:
				fileLines = append(fileLines,
					`- file "`+c.Path+`" (line `+strconv.Itoa(c.LineStart)+`): `+c.Text)
			default:
				fileLines = append(fileLines,
					`- file "`+c.Path+`": `+c.Text)
			}
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
