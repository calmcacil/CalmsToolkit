package colors

import "github.com/charmbracelet/lipgloss"

// Colors wraps lipgloss colors for consistent theming
type Colors struct {
	Reset   lipgloss.TerminalColor
	Red     lipgloss.TerminalColor
	Green   lipgloss.TerminalColor
	Yellow  lipgloss.TerminalColor
	Blue    lipgloss.TerminalColor
	Magenta lipgloss.TerminalColor
	Cyan    lipgloss.TerminalColor
	Gray    lipgloss.TerminalColor
	Bold    lipgloss.TerminalColor
}

// New creates a new color scheme, respecting NO_COLOR environment variable
func New(noColor bool) Colors {
	if noColor {
		return Colors{
			Reset:   lipgloss.NoColor{},
			Red:     lipgloss.NoColor{},
			Green:   lipgloss.NoColor{},
			Yellow:  lipgloss.NoColor{},
			Blue:    lipgloss.NoColor{},
			Magenta: lipgloss.NoColor{},
			Cyan:    lipgloss.NoColor{},
			Gray:    lipgloss.NoColor{},
			Bold:    lipgloss.NoColor{},
		}
	}

	return Colors{
		Reset:   lipgloss.AdaptiveColor{Light: "0", Dark: "0"},
		Red:     lipgloss.AdaptiveColor{Light: "1", Dark: "9"},
		Green:   lipgloss.AdaptiveColor{Light: "2", Dark: "10"},
		Yellow:  lipgloss.AdaptiveColor{Light: "3", Dark: "11"},
		Blue:    lipgloss.AdaptiveColor{Light: "4", Dark: "12"},
		Magenta: lipgloss.AdaptiveColor{Light: "5", Dark: "13"},
		Cyan:    lipgloss.AdaptiveColor{Light: "6", Dark: "14"},
		Gray:    lipgloss.AdaptiveColor{Light: "7", Dark: "8"},
		Bold:    lipgloss.AdaptiveColor{Light: "8", Dark: "7"},
	}
}
