package components

import (
	"strings"

	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/lipgloss"
)

// StatusBar represents a status bar component
type StatusBar struct {
	status  string
	loading bool
	error   string
	width   int
	colors  colors.Colors
}

// NewStatusBar creates a new status bar
func NewStatusBar(c colors.Colors) *StatusBar {
	return &StatusBar{
		colors: c,
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
}

// SetError sets the error message
func (s *StatusBar) SetError(err string) {
	s.error = err
	s.loading = false
}

// SetWidth sets the width of the status bar
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// View renders the status bar
func (s *StatusBar) View() string {
	if s.width <= 0 {
		s.width = 50
	}

	var content string
	var style lipgloss.Style

	baseStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("15")).
		Width(s.width).
		Align(lipgloss.Center)

	if s.error != "" {
		content = "❌ " + s.error
		style = baseStyle.Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15"))
	} else if s.loading {
		content = "⏳ Loading..."
		style = baseStyle.Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
	} else if s.status != "" {
		content = s.status
		style = baseStyle
	} else {
		content = "Ready"
		style = baseStyle
	}

	return style.Render(content)
}

// Header represents a header component
type Header struct {
	title  string
	width  int
	colors colors.Colors
}

// NewHeader creates a new header
func NewHeader(title string, c colors.Colors) *Header {
	return &Header{
		title:  title,
		colors: c,
	}
}

// SetWidth sets the width of the header
func (h *Header) SetWidth(width int) {
	h.width = width
}

// View renders the header
func (h *Header) View() string {
	if h.width <= 0 {
		h.width = 50
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 1).
		Width(h.width - 4). // Account for border padding
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))

	return border.Render(titleStyle.Render(h.title))
}

// Table represents a simple table component
type Table struct {
	headers []string
	rows    [][]string
	width   int
	colors  colors.Colors
}

// NewTable creates a new table
func NewTable(headers []string, c colors.Colors) *Table {
	return &Table{
		headers: headers,
		colors:  c,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(row []string) {
	t.rows = append(t.rows, row)
}

// Clear clears all rows
func (t *Table) Clear() {
	t.rows = [][]string{}
}

// SetWidth sets the width of the table
func (t *Table) SetWidth(width int) {
	t.width = width
}

// View renders the table
func (t *Table) View() string {
	if len(t.headers) == 0 {
		return ""
	}

	var lines []string

	// Calculate column widths (simple approach)
	maxWidth := t.width
	if maxWidth <= 0 {
		maxWidth = 80
	}

	colWidth := maxWidth / len(t.headers)
	if colWidth < 10 {
		colWidth = 10
	}

	// Render headers
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))

	var headerCells []string
	for _, header := range t.headers {
		cell := lipgloss.NewStyle().
			Width(colWidth).
			MaxWidth(colWidth).
			Align(lipgloss.Left).
			Render(truncate(header, colWidth-2))
		headerCells = append(headerCells, headerStyle.Render(cell))
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, headerCells...))

	// Add separator
	separator := strings.Repeat("─", maxWidth)
	lines = append(lines, separator)

	// Render rows
	for _, row := range t.rows {
		var cells []string
		for i, cell := range row {
			if i >= len(t.headers) {
				break
			}
			cellStyle := lipgloss.NewStyle().
				Width(colWidth).
				MaxWidth(colWidth).
				Align(lipgloss.Left)
			cells = append(cells, cellStyle.Render(truncate(cell, colWidth-2)))
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, cells...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// truncate truncates a string to the specified length, adding "..." if needed
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return strings.Repeat(".", length)
	}
	return s[:length-3] + "..."
}
