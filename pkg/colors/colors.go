package colors

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors wraps lipgloss colors for consistent theming
type Colors struct {
	noColor bool
	bold    bool
}

// New creates a new color scheme, optionally disabling colors
func New(noColor bool) Colors {
	return Colors{
		noColor: noColor,
		bold:    !noColor,
	}
}

// Style returns a lipgloss.Style with the configured color
func (c Colors) Style(color string) lipgloss.Style {
	if c.noColor {
		return lipgloss.NewStyle()
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	if c.bold {
		style = style.Bold(true)
	}
	return style
}

// ServerColor returns the appropriate color for a server type
func (c Colors) ServerColor(server string) string {
	if c.noColor {
		return ""
	}
	switch server {
	case "plex":
		return "3" // Yellow
	case "jellyfin":
		return "5" // Magenta
	default:
		return "6" // Cyan
	}
}

// StatusColor returns green for direct play, red for transcoding
func (c Colors) StatusColor(transcoding bool) string {
	if c.noColor {
		return ""
	}
	if transcoding {
		return "1" // Red
	}
	return "2" // Green
}

// Color constants
const (
	Red     = "1"
	Green   = "2"
	Yellow  = "3"
	Blue    = "4"
	Magenta = "5"
	Cyan    = "6"
	Gray    = "8"
	Reset   = ""
	Bold    = "bold"
)
