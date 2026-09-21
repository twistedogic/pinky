package render

import (
	"github.com/charmbracelet/glamour/ansi"
)

// pinkyStyle returns a glamour StyleConfig that uses the same color
// family as pinky's lipgloss palette (212 accent, 250 body, 241 dim,
// 42 user) so the rendered markdown feels native to the TUI.
//
// The hex values approximate the corresponding ANSI 256 colors so the
// chroma syntax highlighter (which expects hex color strings) can
// emit sensible ANSI output at render time.
func pinkyStyle() ansi.StyleConfig {
	const (
		hex212 = "#d75fd7" // pink/magenta (heading accent)
		hex250 = "#bcbcbc" // light gray (body text)
		hex241 = "#626262" // dim gray (secondary)
		hex42  = "#00d75f" // green (literal strings in code)
		hex228 = "#ffdf00" // yellow (h1 highlight)
	)

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       stringPtr(hex250),
			},
			Margin: uintPtr(2),
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       stringPtr(hex212),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Suffix: "",
				Color:  stringPtr(hex228),
				Bold:   boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  stringPtr(hex212),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  stringPtr(hex212),
				Bold:   boolPtr(true),
			},
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr(hex212),
			Underline: boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     stringPtr(hex212),
			Underline: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color: stringPtr(hex241),
		},
		Emph: ansi.StylePrimitive{
			Color:  stringPtr(hex228),
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(hex228),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr(hex250),
				},
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr(hex250),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr(hex212),
					Bold:  boolPtr(true),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr(hex42),
				},
				Comment: ansi.StylePrimitive{
					Color:  stringPtr(hex241),
					Italic: boolPtr(true),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr(hex228),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr(hex250),
				},
			},
		},
	}
}

// Local pointer helpers; glamour/ansi has unexported versions.
func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }

