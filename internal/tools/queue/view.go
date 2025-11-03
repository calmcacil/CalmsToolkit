package queue

import (
	"fmt"
	"strings"

	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/lipgloss"
)

// View renders the model
func (m Model) View() string {
	c := colors.New()

	if m.loading {
		return m.renderLoading(c)
	}

	if m.error != "" {
		return m.renderError(c)
	}

	switch m.view {
	case ViewList:
		return m.renderListView(c)
	case ViewDetail:
		return m.renderDetailView(c)
	case ViewConfirm:
		return m.renderConfirmView(c)
	default:
		return "Invalid view"
	}
}

// renderLoading renders the loading screen
func (m Model) renderLoading(c colors.Colors) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Render("Queue Remediation")

	loading := lipgloss.NewStyle().
		Foreground(lipgloss.Color("36")).
		Render("⏳ Fetching queue data...")

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Press Ctrl+C to quit")

	return lipgloss.Place(
		m.height, m.width,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			"",
			title,
			"",
			loading,
			"",
			help,
		),
	)
}

// renderError renders the error screen
func (m Model) renderError(c colors.Colors) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("9")).
		Render("❌ Error")

	errorMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Width(m.width - 4).
		Render(m.error)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Press R to retry, Ctrl+C to quit")

	return lipgloss.Place(
		m.height, m.width,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			"",
			title,
			"",
			errorMsg,
			"",
			help,
		),
	)
}

// renderListView renders the main list view
func (m Model) renderListView(c colors.Colors) string {
	var content strings.Builder

	// Header
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Render("Queue Remediation")

	filter := "All Items"
	if m.showOnlyIssues {
		filter = "Issues Only"
	}

	filterInfo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("36")).
		Render(fmt.Sprintf("Filter: %s | Items: %d", filter, len(m.getFilteredItems())))

	header := lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", filterInfo)
	content.WriteString(header + "\n\n")

	// Table
	if len(m.items) == 0 {
		noItems := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No queue items found")
		content.WriteString(noItems)
	} else {
		tableModel := m.createTable()
		content.WriteString(tableModel.View())
	}

	// Help
	content.WriteString("\n\n")
	help := m.renderListHelp(c)
	content.WriteString(help)

	return content.String()
}

// renderDetailView renders the detail view for selected item
func (m Model) renderDetailView(c colors.Colors) string {
	if len(m.getFilteredItems()) == 0 {
		return "No items available"
	}

	item := m.getFilteredItems()[m.selected]
	action := m.getRecommendedAction(item)

	var content strings.Builder

	// Header
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Render("Queue Item Details")

	content.WriteString(title + "\n\n")

	// Item details
	details := [][]string{
		{"Instance:", item.InstanceName},
		{"Title:", item.Title},
		{"Status:", item.Status},
		{"Download Client:", item.DownloadClient},
		{"Protocol:", item.Protocol},
		{"Release:", item.ReleaseTitle},
		{"Size:", formatBytes(item.Size)},
		{"Time Left:", item.TimeLeft},
	}

	for _, detail := range details {
		key := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("36")).
			Render(detail[0])

		value := lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Render(detail[1])

		content.WriteString(fmt.Sprintf("%s %s\n", key, value))
	}

	// Status messages
	if len(item.StatusMessages) > 0 {
		content.WriteString("\n")
		statusTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("36")).
			Render("Status Messages:")
		content.WriteString(statusTitle + "\n")

		for _, sm := range item.StatusMessages {
			title := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("11")).
				Render("  " + sm.Title)
			content.WriteString(title + "\n")

			for _, msg := range sm.Messages {
				message := lipgloss.NewStyle().
					Foreground(lipgloss.Color("15")).
					Render("    • " + msg)
				content.WriteString(message + "\n")
			}
		}
	}

	// Recommended action
	content.WriteString("\n")
	actionTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("36")).
		Render("Recommended Action:")
	content.WriteString(actionTitle + "\n")

	actionText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Render(fmt.Sprintf("  %s - %s", action.Action, action.Reason))
	content.WriteString(actionText + "\n")

	// Help
	content.WriteString("\n")
	help := m.renderDetailHelp(c, action)
	content.WriteString(help)

	return content.String()
}

// renderConfirmView renders the confirmation dialog
func (m Model) renderConfirmView(c colors.Colors) string {
	var content strings.Builder

	// Header
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Render("Confirm Action")

	content.WriteString(title + "\n\n")

	// Confirmation message
	message := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Width(m.width - 4).
		Render(m.confirmMessage)
	content.WriteString(message + "\n\n")

	// Options
	confirm := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true).
		Render("Enter")

	cancel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Render("Esc")

	options := fmt.Sprintf("%s to confirm, %s to cancel", confirm, cancel)
	optionsStyled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Render(options)

	content.WriteString(optionsStyled)

	return content.String()
}

// renderListHelp renders help text for list view
func (m Model) renderListHelp(c colors.Colors) string {
	helpItems := []string{
		"↑↓/j/k: Navigate",
		"Enter: Details",
		"R: Refresh",
		"I: Toggle Issues Only",
		"Ctrl+C: Quit",
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Join(helpItems, " | "))
}

// renderDetailHelp renders help text for detail view
func (m Model) renderDetailHelp(c colors.Colors, action QueueItemAction) string {
	var helpItems []string

	helpItems = append(helpItems, "F1: Back to List")

	if action.Action == ActionDelete {
		helpItems = append(helpItems, "D: Delete Item")
	}
	if action.Action == ActionManualImport {
		helpItems = append(helpItems, "M: Manual Import")
	}

	helpItems = append(helpItems, "Ctrl+C: Quit")

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Join(helpItems, " | "))
}

// formatBytes formats bytes into human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
