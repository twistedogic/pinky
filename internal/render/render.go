// Package render formats agent message text and comment annotations
// for the latest-message view. The raw message text is displayed
// verbatim with no styling (no glamour / chroma); comment presence
// is shown as a yellow left-gutter on every source line touched by
// a saved comment's byte range, and the cursor is rendered as an
// inverted block at (lineIdx, charPos) by the caller.
package render

import "time"

// CommentKind discriminates which fields of Comment carry the
// anchor. Message-kind comments anchor to a byte range in the latest
// agent message; file-kind comments anchor to a file path and
// optional byte range in the agent's workspace.
type CommentKind int

const (
	CommentMessage CommentKind = iota // zero value — message-anchored
	CommentFile                       // file-anchored
)

// Comment is a user-authored annotation attached either to a byte
// range in the latest agent message or to a file in the agent's
// workspace.
//
// Message-kind fields (CommentMessage):
//   - ByteA, ByteC: byte offsets into m.latest.Text that cover the
//     commented range. The verbatim slice ByteA:ByteC is stored in
//     Source for the redirect appendix to quote.
//
// File-kind fields (CommentFile):
//   - Path: relative path to the file under the agent pane's cwd.
//   - LineStart, LineEnd: 1-based inclusive line range.
//   - ByteA, ByteC: byte offsets into the file's raw content for
//     inline selections made via visual mode; 0/0 for line-range
//     comments without an inline selection.
//
// Source is the verbatim substring of the anchored range
// (m.latest.Text[ByteA:ByteC] for message-kind, or the file's raw
// content for file-kind inline comments) so the redirect appendix
// can quote the snippet without re-reading the source.
type Comment struct {
	Kind      CommentKind
	ByteA     int
	ByteC     int
	Path      string
	LineStart int
	LineEnd   int
	Source    string
	Text      string
	CreatedAt time.Time
}