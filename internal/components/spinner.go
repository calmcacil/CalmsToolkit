package components

import (
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Spinner wraps the bubbles spinner with our color scheme
type Spinner struct {
	spinner spinner.Model
	colors  colors.Colors
}

// NewSpinner creates a new spinner with the given color scheme
func NewSpinner(c colors.Colors) Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = c.Style(colors.Cyan)
	return Spinner{
		spinner: s,
		colors:  c,
	}
}

// Update handles spinner updates
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

// View returns the spinner view
func (s Spinner) View() string {
	return s.spinner.View()
}

// Start returns a command to start the spinner
func (s Spinner) Start() tea.Cmd {
	return s.spinner.Tick
}

// Stop stops the spinner
func (s Spinner) Stop() {
	// spinner.Model doesn't have Hide method, just set it to not show
	s.spinner.Spinner = spinner.Line
}
