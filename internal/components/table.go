package components

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableModel represents a reusable table component
type TableModel struct {
	table.Model
}

// NewTable creates a new table with default styling
func NewTable(columns []table.Column) TableModel {
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	s.Cell = s.Cell.
		Foreground(lipgloss.Color("15")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	t.SetStyles(s)

	return TableModel{Model: t}
}

// SetRows updates the table rows
func (t TableModel) SetRows(rows []table.Row) TableModel {
	t.Model.SetRows(rows)
	return t
}

// Update handles table updates
func (t TableModel) Update(msg tea.Msg) (TableModel, tea.Cmd) {
	var cmd tea.Cmd
	t.Model, cmd = t.Model.Update(msg)
	return t, cmd
}
