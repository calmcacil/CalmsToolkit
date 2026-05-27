package colors

import "fmt"

// Colorizer provides no-color awareness for ANSI codes.
type Colorizer struct {
	NoColor bool
}

func New(noColor bool) *Colorizer {
	return &Colorizer{NoColor: noColor}
}

func (c *Colorizer) Apply(code string) string {
	if c.NoColor {
		return ""
	}
	return code
}

// Standard ANSI colour constants (kept for backwards compatibility).
const (
	Reset   = "\033[0m"
	Red     = "\033[0;31m"
	Green   = "\033[0;32m"
	Yellow  = "\033[0;33m"
	Blue    = "\033[0;34m"
	Magenta = "\033[0;35m"
	Cyan    = "\033[0;36m"
	Gray    = "\033[0;90m"
	Bold    = "\033[1m"
	Orange  = "\033[38;5;208m"
)

// Cursor / screen control sequences (not themeable).
const (
	ClearScreen = "\033[2J"
	HomeCursor  = "\033[H"
	HideCursor  = "\033[?25l"
	ShowCursor  = "\033[?25h"
)

// --- Theme / Palette system ------------------------------------------------

// Palette maps semantic colour roles to ANSI escape codes.
type Palette struct {
	Reset string
	Bold  string

	// Status indicators
	Success  string // available, imported, healthy, direct play
	Error    string // missing, failed, transcoding
	Warning  string // warning, deleted, searching
	Info     string // upcoming, renamed
	Accent   string // totals, progress percentage, grabbed
	Premiere string // premiere events
	Subdued  string // ended, ignored, historical

	// Server identity
	ServerPlex     string
	ServerJellyfin string

	// Metrics
	Bandwidth string

	// Feed-specific action colours
	Grabbed string
	Renamed string

	// Calendar-specific UI elements
	QueueWarning string // warning banner
	QueueLink    string // issue link in banner
	NoReleases   string // empty state
	Overflow     string // "N more" indicator
	DayHeader    string // date heading
}

// Theme is a named colour theme.
type Theme string

const (
	ThemeDefault         Theme = "default"
	ThemeCatppuccinMocha Theme = "catppuccin-mocha"
	ThemeCatppuccinLatte Theme = "catppuccin-latte"
)

// ValidThemes returns all recognised theme names.
func ValidThemes() []string {
	return []string{
		string(ThemeDefault),
		string(ThemeCatppuccinMocha),
		string(ThemeCatppuccinLatte),
	}
}

// GetPalette resolves a theme name to a Palette.
// Unrecognised names fall back to the default palette.
func GetPalette(name string) *Palette {
	switch Theme(name) {
	case ThemeCatppuccinMocha:
		return catppuccinMocha()
	case ThemeCatppuccinLatte:
		return catppuccinLatte()
	default:
		return defaultPalette()
	}
}

// hexTrueColor converts "#RRGGBB" to a 24-bit ANSI foreground code.
func hexTrueColor(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return Reset
	}
	r := parseHex(hex[1:3])
	g := parseHex(hex[3:5])
	b := parseHex(hex[5:7])
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

func parseHex(s string) uint8 {
	var v uint8
	for i := 0; i < 2; i++ {
		v <<= 4
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v |= c - '0'
		case c >= 'a' && c <= 'f':
			v |= c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v |= c - 'A' + 10
		}
	}
	return v
}

// --- Built-in palettes -----------------------------------------------------

func defaultPalette() *Palette {
	return &Palette{
		Reset:   Reset,
		Bold:    Bold,
		Success: Green,
		Error:   Red,
		Warning: Yellow,
		Info:    Blue,
		Accent:  Cyan,

		Premiere: Orange,
		Subdued:  Gray,

		ServerPlex:     Yellow,
		ServerJellyfin: Magenta,

		Bandwidth: Magenta,

		Grabbed: Cyan,
		Renamed: Blue,

		QueueWarning: Bold + Red,
		QueueLink:    Yellow,
		NoReleases:   Green,
		Overflow:     Cyan,
		DayHeader:    Bold,
	}
}

func catppuccinMocha() *Palette {
	return &Palette{
		Reset:   Reset,
		Bold:    Bold,
		Success: hexTrueColor("#a6e3a1"),
		Error:   hexTrueColor("#f38ba8"),
		Warning: hexTrueColor("#f9e2af"),
		Info:    hexTrueColor("#89b4fa"),
		Accent:  hexTrueColor("#94e2d5"),

		Premiere: hexTrueColor("#fab387"),
		Subdued:  hexTrueColor("#a6adc8"),

		ServerPlex:     hexTrueColor("#f9e2af"),
		ServerJellyfin: hexTrueColor("#cba6f7"),

		Bandwidth: hexTrueColor("#cba6f7"),

		Grabbed: hexTrueColor("#94e2d5"),
		Renamed: hexTrueColor("#89b4fa"),

		QueueWarning: Bold + hexTrueColor("#f38ba8"),
		QueueLink:    hexTrueColor("#f9e2af"),
		NoReleases:   hexTrueColor("#a6e3a1"),
		Overflow:     hexTrueColor("#94e2d5"),
		DayHeader:    Bold,
	}
}

func catppuccinLatte() *Palette {
	return &Palette{
		Reset:   Reset,
		Bold:    Bold,
		Success: hexTrueColor("#40a02b"),
		Error:   hexTrueColor("#d20f39"),
		Warning: hexTrueColor("#df8e1d"),
		Info:    hexTrueColor("#1e66f5"),
		Accent:  hexTrueColor("#179299"),

		Premiere: hexTrueColor("#fe640b"),
		Subdued:  hexTrueColor("#6c6f85"),

		ServerPlex:     hexTrueColor("#df8e1d"),
		ServerJellyfin: hexTrueColor("#8839ef"),

		Bandwidth: hexTrueColor("#8839ef"),

		Grabbed: hexTrueColor("#179299"),
		Renamed: hexTrueColor("#1e66f5"),

		QueueWarning: Bold + hexTrueColor("#d20f39"),
		QueueLink:    hexTrueColor("#df8e1d"),
		NoReleases:   hexTrueColor("#40a02b"),
		Overflow:     hexTrueColor("#179299"),
		DayHeader:    Bold,
	}
}
