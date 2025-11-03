package components

import (
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/lipgloss"
)

// StatusBar represents the status bar component
type StatusBar struct {
	status  string
	loading bool
	error   string
	width   int
	style   lipgloss.Style
	colors  colors.Colors
}

// NewStatusBar creates a new status bar component
func NewStatusBar(colors colors.Colors) *StatusBar {
	return &StatusBar{
		colors: colors,
		style: lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15")).
			Width(50).
			Align(lipgloss.Center),
	}
}

// SetStatus sets the status message
func (s *StatusBar) SetStatus(status string) {
	s.status = status
	s.error = ""
	s.loading = false
}

// SetLoading sets the loading state
func (s *StatusBar) SetLoading(loading bool) {
	s.loading = loading
	if loading {
		s.status = "Loading..."
		s.error = ""
	}
}

// SetError sets the error message
func (s *StatusBar) SetError(err string) {
	s.error = err
	s.status = ""
	s.loading = false
}

// SetWidth sets the status bar width
func (s *StatusBar) SetWidth(width int) {
	s.width = width
	s.style = s.style.Width(width)
}

// SetStyle sets the status bar style
func (s *StatusBar) SetStyle(style lipgloss.Style) {
	s.style = style
}

// View renders the status bar
func (s *StatusBar) View() string {
	var message string

	if s.error != "" {
		if s.colors.Red != "" {
			message = "❌ " + s.colors.RedText(s.error)
		} else {
			message = "❌ " + s.error
		}
	} else if s.loading {
		if s.colors.Yellow != "" {
			message = "⏳ " + s.colors.YellowText("Loading...")
		} else {
			message = "⏳ Loading..."
		}
	} else if s.status != "" {
		if s.colors.Green != "" {
			message = "✓ " + s.colors.GreenText(s.status)
		} else {
			message = "✓ " + s.status
		}
	} else {
		message = "Ready"
	}

	if s.colors.Reset != "" {
		// Use ANSI colors if available
		return message + s.colors.Reset
	}
	// Use lipgloss styling
	return s.style.Render(message)
}
