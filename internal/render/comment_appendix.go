package render

import "strings"

// FormatCommentsAppendix builds the redirect appendix string for a
// set of comments. Empty comments slice → empty string.
//
// Layout per design D8:
//
//	---
//	N comments:
//	- block "<excerpt>" (lines X-Y): <text>
//	- inline "<excerpt>" (line Z): <text>
//
// Excerpt is the first ~40 chars of the block source (block-level)
// or the inline Source (inline), with trailing ellipsis on
// truncation. Comments are emitted in CreatedAt order
// (chronological). When the include flag is on but len(comments)==0,
// the caller gets back "" so the redirect is sent as plain text.
func FormatCommentsAppendix(comments []Comment, blocks []Block) string {
	if len(comments) == 0 {
		return ""
	}
	// Sort by CreatedAt ascending so the appendix reads chronologically.
	sorted := make([]Comment, len(comments))
	copy(sorted, comments)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1].CreatedAt.After(sorted[j].CreatedAt); j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	var b strings.Builder
	b.WriteString("\n\n---\n")
	b.WriteString(itoa(len(sorted)))
	b.WriteString(" comments:\n")
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
				b.WriteString(itoa(blocks[c.BlockIdx].StartLine))
				b.WriteString("-")
				b.WriteString(itoa(blocks[c.BlockIdx].EndLine))
				b.WriteString(")")
			} else {
				b.WriteString(" (line ")
				b.WriteString(itoa(blocks[c.BlockIdx].StartLine))
				b.WriteString(")")
			}
		}
		b.WriteString(": ")
		b.WriteString(c.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

// itoa avoids importing strconv in this hot path; small int
// formatting is fine without it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
