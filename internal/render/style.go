package render

import (
	"github.com/charmbracelet/glamour/ansi"
)

// pinkyStyle returns a glamour StyleConfig aligned with pinky's
// lipgloss palette (51 cyan selection/headings, 228 yellow comments,
// 250 body, 241 dim, 42 user, 212 picker accent). Headings use
// weight+color only (no `#` prefix); Document.Margin=1 leaves room
// for the model's left gutter.
func pinkyStyle() ansi.StyleConfig {
	const (
		hex51  = "#00d7d7" // cyan (selection + h1/h2)
		hex87  = "#5fffff" // softer cyan (h3)
		hex212 = "#d75fd7" // pink/magenta (picker header accent only)
		hex250 = "#bcbcbc" // light gray (body text)
		hex241 = "#626262" // dim gray (secondary)
		hex42  = "#00d75f" // green (literal strings in code)
		hex228 = "#ffdf00" // yellow (h1 highlight + comment gutter)
	)

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       stringPtr(hex250),
			},
			Margin: uintPtr(1),
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
				Prefix:    "",
				Suffix:    "",
				Color:     stringPtr(hex51),
				Bold:      boolPtr(true),
				Underline: boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "",
				Color:  stringPtr(hex51),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "",
				Color:  stringPtr(hex87),
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
