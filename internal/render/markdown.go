package render

import (
	"github.com/charmbracelet/glamour"
)

// hoverStylesheet is the JSON style config for glamour. Sized for
// the hover modal's small viewport: cyan (51) headers that match
// pinky's hover accent, 228-on-236 code blocks (matches the status
// bar palette), 0 document margin so the body fills the modal.
//
//   ponytail: hard-coded JSON — glamour stylesheets are JSON and
//   have to live somewhere; one tiny stylesheet beats dragging
//   chroma + glamour's standard style through the modal.
var hoverStylesheet = []byte(`{
  "document": {"block_prefix": "", "block_suffix": "", "margin": 0},
  "heading": {"block_suffix": "\n"},
  "h1": {"prefix": "# ", "color": "51", "bold": true},
  "h2": {"prefix": "## ", "color": "51", "bold": true},
  "h3": {"prefix": "### ", "color": "51", "bold": true},
  "h4": {"prefix": "#### ", "color": "51", "bold": true},
  "h5": {"prefix": "##### ", "color": "51", "bold": true},
  "h6": {"prefix": "###### ", "color": "51", "bold": true},
  "strong": {"bold": true},
  "emph": {"italic": true},
  "strikethrough": {"block_prefix": "~~", "block_suffix": "~~"},
  "code": {"color": "228", "background_color": "236"},
  "code_block": {"color": "228", "background_color": "236", "margin": 0},
  "list": {"level_indent": 2},
  "item": {"block_prefix": "• "},
  "link": {"color": "39", "underline": true}
}`)

// RenderMarkdown renders content as ANSI-styled markdown suitable
// for the hover modal. width caps each line at width visible cells
// (glamour's word wrap). On any error (parse, render), returns the
// input verbatim so the modal has something to show rather than
// rendering nothing.
func RenderMarkdown(content string, width int) string {
	w := width
	if w < 8 {
		w = 8
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes(hoverStylesheet),
		glamour.WithWordWrap(w),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}