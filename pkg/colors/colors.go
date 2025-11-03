package colors

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors wraps lipgloss colors for consistent theming
type Colors struct {
	noColor bool
}

// New creates a new color scheme, optionally disabling colors
func New(noColor bool) Colors {
	return Colors{noColor: noColor}
}

// Style returns a lipgloss.Style with the given foreground color
func (c Colors) Style(color string) lipgloss.Style {
	if c.noColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

// BoldStyle returns a bold lipgloss.Style with the given foreground color
func (c Colors) BoldStyle(color string) lipgloss.Style {
	if c.noColor {
		return lipgloss.NewStyle().Bold(true)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
}

// Color constants
const (
	Red     = "196"
	Green   = "46"
	Yellow  = "226"
	Blue    = "21"
	Magenta = "201"
	Cyan    = "51"
	Gray    = "245"
	Orange  = "208"
)
