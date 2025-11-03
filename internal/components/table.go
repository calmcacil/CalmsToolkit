package components

import (
	"fmt"
	"strings"

	"github.com/calmcacil/CalmsToolkit/pkg/colors"
)

// Table represents a simple table component
type Table struct {
	headers []string
	rows    [][]string
	colors  colors.Colors
	width   int
}

// NewTable creates a new table
func NewTable(colorScheme colors.Colors, width int) *Table {
	return &Table{
		colors: colorScheme,
		width:  width,
	}
}

// SetHeaders sets the table headers
func (t *Table) SetHeaders(headers []string) {
	t.headers = headers
}

// AddRow adds a row to the table
func (t *Table) AddRow(row []string) {
	t.rows = append(t.rows, row)
}

// Clear clears all rows
func (t *Table) Clear() {
	t.rows = [][]string{}
}

// Render renders the table as a string
func (t *Table) Render() string {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return ""
	}

	var builder strings.Builder

	// Calculate column widths
	colWidths := t.calculateColumnWidths()

	// Render headers
	if len(t.headers) > 0 {
		headerRow := t.renderRow(t.headers, colWidths, true)
		builder.WriteString(headerRow)
		builder.WriteString("\n")

		// Add separator
		separator := t.renderSeparator(colWidths)
		builder.WriteString(separator)
		builder.WriteString("\n")
	}

	// Render rows
	for _, row := range t.rows {
		rowStr := t.renderRow(row, colWidths, false)
		builder.WriteString(rowStr)
		builder.WriteString("\n")
	}

	return builder.String()
}

func (t *Table) calculateColumnWidths() []int {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return []int{}
	}

	// Determine number of columns
	numCols := len(t.headers)
	if numCols == 0 && len(t.rows) > 0 {
		numCols = len(t.rows[0])
	}

	// Initialize with header widths
	colWidths := make([]int, numCols)
	for i, header := range t.headers {
		colWidths[i] = len(header)
	}

	// Update with row widths
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	return colWidths
}

func (t *Table) renderRow(row []string, colWidths []int, isHeader bool) string {
	if len(row) == 0 {
		return ""
	}

	var cells []string
	for i, cell := range row {
		width := 20 // default width
		if i < len(colWidths) {
			width = colWidths[i]
		}

		// Truncate if too long
		if len(cell) > width {
			cell = cell[:width-3] + "..."
		}

		// Pad to width
		cell = fmt.Sprintf("%-*s", width, cell)

		// Apply styling
		if isHeader {
			cell = t.colors.BoldStyle(colors.Cyan).Render(cell)
		}

		cells = append(cells, cell)
	}

	return "│ " + strings.Join(cells, " │ ") + " │"
}

func (t *Table) renderSeparator(colWidths []int) string {
	if len(colWidths) == 0 {
		return ""
	}

	var parts []string
	for _, width := range colWidths {
		parts = append(parts, strings.Repeat("─", width))
	}

	return "├─" + strings.Join(parts, "─┼─") + "─┤"
}
