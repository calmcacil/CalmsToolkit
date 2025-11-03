package components

import (
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Table represents a table component with consistent styling
type Table struct {
	model  table.Model
	colors colors.Colors
}

// NewTable creates a new table component
func NewTable(colors colors.Colors) Table {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Background(lipgloss.Color("240"))

	t := table.New(
		table.WithColumns([]table.Column{}),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(7),
		table.WithWidth(80),
	)

	t.SetStyles(table.Styles{
		Selected: selectedStyle,
		Header:   headerStyle,
		Cell:     lipgloss.NewStyle().Padding(0, 1),
	})

	return Table{
		model:  t,
		colors: colors,
	}
}

// SetColumns sets table columns
func (t *Table) SetColumns(columns []table.Column) {
	t.model.SetColumns(columns)
}

// SetRows sets table rows
func (t *Table) SetRows(rows []table.Row) {
	t.model.SetRows(rows)
}

// AddRow adds a single row to the table
func (t *Table) AddRow(row table.Row) {
	currentRows := t.model.Rows()
	t.model.SetRows(append(currentRows, row))
}

// ClearRows clears all rows from the table
func (t *Table) ClearRows() {
	t.model.SetRows([]table.Row{})
}

// SetWidth sets table width
func (t *Table) SetWidth(width int) {
	t.model.SetWidth(width)
}

// SetHeight sets table height
func (t *Table) SetHeight(height int) {
	t.model.SetHeight(height)
}

// GetSelected returns the currently selected row index
func (t *Table) GetSelected() int {
	return t.model.Cursor()
}

// GetSelectedRow returns the currently selected row
func (t *Table) GetSelectedRow() table.Row {
	rows := t.model.Rows()
	if len(rows) == 0 {
		return table.Row{}
	}
	cursor := t.model.Cursor()
	if cursor < 0 || cursor >= len(rows) {
		return table.Row{}
	}
	return rows[cursor]
}

// Update handles table updates
func (t *Table) Update(msg tea.Msg) (Table, tea.Cmd) {
	var cmd tea.Cmd
	t.model, cmd = t.model.Update(msg)
	return *t, cmd
}

// View renders the table
func (t *Table) View() string {
	return t.model.View()
}
