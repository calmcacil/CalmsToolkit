package components

import (
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/lipgloss"
)

// Header represents the application header component
type Header struct {
	title  string
	style  lipgloss.Style
	colors colors.Colors
}

// NewHeader creates a new header component
func NewHeader(title string, colors colors.Colors) *Header {
	return &Header{
		title:  title,
		colors: colors,
		style: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1).
			Width(80),
	}
}

// SetTitle sets the header title
func (h *Header) SetTitle(title string) {
	h.title = title
}

// SetStyle sets the header style
func (h *Header) SetStyle(style lipgloss.Style) {
	h.style = style
}

// SetWidth sets the header width
func (h *Header) SetWidth(width int) {
	h.style = h.style.Width(width)
}

// View renders the header
func (h *Header) View() string {
	if h.colors.Bold != "" {
		// Use ANSI colors if available
		return h.colors.BoldText(h.title)
	}
	// Use lipgloss styling
	return h.style.Render(h.title)
}
