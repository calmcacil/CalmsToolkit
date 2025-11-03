package mediarequests

import (
	"fmt"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/components"
	"github.com/charmbracelet/lipgloss"
)

// renderSearchView renders the search interface
func (m Model) renderSearchView() string {
	var content strings.Builder

	// Header
	header := components.NewHeader("Media Requests - Search", m.colors)
	header.SetWidth(m.width)
	content.WriteString(header.View())
	content.WriteString("\n\n")

	// Search input
	searchStyle := lipgloss.NewStyle().
		Width(m.width-4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 1)

	content.WriteString(searchStyle.Render(m.searchInput.View()))
	content.WriteString("\n\n")

	// Instructions
	instructions := []string{
		"Type to search for movies or TV shows",
		"Press Enter to search",
		"Press Esc or Ctrl+C to quit",
	}

	instStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	for _, inst := range instructions {
		content.WriteString(instStyle.Render("• " + inst))
		content.WriteString("\n")
	}

	// Error or loading
	if m.error != "" {
		content.WriteString("\n")
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		content.WriteString(errorStyle.Render("❌ " + m.error))
	}

	if m.loading {
		content.WriteString("\n")
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
		content.WriteString(loadingStyle.Render("⏳ Searching..."))
	}

	return content.String()
}

// renderSelectView renders the results selection interface
func (m Model) renderSelectView() string {
	var content strings.Builder

	// Header
	header := components.NewHeader("Media Requests - Select", m.colors)
	header.SetWidth(m.width)
	content.WriteString(header.View())
	content.WriteString("\n\n")

	if len(m.results) == 0 {
		noResultsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true).
			Align(lipgloss.Center)
		content.WriteString(noResultsStyle.Render("No results found"))
		return content.String()
	}

	// Results
	displayLimit := 10
	if len(m.results) < displayLimit {
		displayLimit = len(m.results)
	}

	for i := 0; i < displayLimit; i++ {
		result := m.results[i]
		selected := i == m.selected

		// Result styling
		var resultStyle lipgloss.Style
		if selected {
			resultStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("4")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1).
				Width(m.width - 4)
		} else {
			resultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Padding(0, 1).
				Width(m.width - 4)
		}

		// Build result line
		title := GetTitle(result)
		year := GetYear(result)
		typeIcon := "🎬"
		if result.MediaType == "tv" {
			typeIcon = "📺"
		}

		line := fmt.Sprintf("%s %d. %s", typeIcon, i+1, title)
		if year != "" {
			line += fmt.Sprintf(" (%s)", year)
		}

		// Add status indicators
		if result.MediaInfo != nil {
			status := result.MediaInfo.Status
			if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
				line += " [AVAILABLE]"
			} else if len(result.MediaInfo.Requests) > 0 {
				line += " [REQUESTED]"
			}
		}

		content.WriteString(resultStyle.Render(line))
		content.WriteString("\n")

		// Add overview for selected item
		if selected && result.Overview != "" {
			overviewStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Italic(true).
				MarginLeft(4).
				Width(m.width - 8)

			overview := result.Overview
			if len(overview) > 100 {
				overview = overview[:97] + "..."
			}
			content.WriteString(overviewStyle.Render(overview))
			content.WriteString("\n")
		}

		if selected {
			content.WriteString("\n")
		}
	}

	// Instructions
	instructions := []string{
		"↑/↓ or K/J to navigate",
		"Enter to select",
		"Backspace to go back",
		"Esc or Ctrl+C to quit",
	}

	instStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	for _, inst := range instructions {
		content.WriteString(instStyle.Render("• " + inst))
		content.WriteString("\n")
	}

	// Error or loading
	if m.error != "" {
		content.WriteString("\n")
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		content.WriteString(errorStyle.Render("❌ " + m.error))
	}

	if m.loading {
		content.WriteString("\n")
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
		content.WriteString(loadingStyle.Render("⏳ Loading..."))
	}

	return content.String()
}

// renderConfirmView renders the confirmation interface
func (m Model) renderConfirmView() string {
	var content strings.Builder

	// Header
	header := components.NewHeader("Media Requests - Confirm", m.colors)
	header.SetWidth(m.width)
	content.WriteString(header.View())
	content.WriteString("\n\n")

	if len(m.results) == 0 || m.selected >= len(m.results) {
		content.WriteString("No media selected")
		return content.String()
	}

	selectedMedia := m.results[m.selected]
	title := GetTitle(selectedMedia)
	year := GetYear(selectedMedia)

	// Media details
	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	content.WriteString(detailStyle.Render("Media: " + title))
	if year != "" {
		content.WriteString(" (" + year + ")")
	}
	content.WriteString("\n")

	content.WriteString(detailStyle.Render("Type: " + strings.Title(selectedMedia.MediaType)))
	content.WriteString("\n")

	// TV show seasons
	if selectedMedia.MediaType == "tv" && m.seasons != nil {
		if m.seasons == "all" {
			content.WriteString(detailStyle.Render("Seasons: All"))
		} else if seasonList, ok := m.seasons.([]int); ok {
			content.WriteString(fmt.Sprintf("Seasons: %v", seasonList))
		}
		content.WriteString("\n")
	}

	// Server overrides
	if m.overrides != nil {
		if m.overrides.ServerName != "" {
			content.WriteString(detailStyle.Render("Server: " + m.overrides.ServerName))
			content.WriteString("\n")
		}
		if m.overrides.RootFolder != "" {
			content.WriteString(detailStyle.Render("Root Folder: " + m.overrides.RootFolder))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")

	// Confirmation prompt
	confirmStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true)
	content.WriteString(confirmStyle.Render("Submit this request? (y/n)"))

	// Instructions
	instructions := []string{
		"Y to submit request",
		"N to cancel",
		"Esc or Ctrl+C to quit",
	}

	instStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	content.WriteString("\n\n")
	for _, inst := range instructions {
		content.WriteString(instStyle.Render("• " + inst))
		content.WriteString("\n")
	}

	// Error or loading
	if m.error != "" {
		content.WriteString("\n")
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		content.WriteString(errorStyle.Render("❌ " + m.error))
	}

	if m.loading {
		content.WriteString("\n")
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
		content.WriteString(loadingStyle.Render("⏳ Submitting..."))
	}

	return content.String()
}

// renderSubmitView renders the success/failure interface
func (m Model) renderSubmitView() string {
	var content strings.Builder

	// Header
	header := components.NewHeader("Media Requests - Complete", m.colors)
	header.SetWidth(m.width)
	content.WriteString(header.View())
	content.WriteString("\n\n")

	if m.submittedRequest != nil {
		// Success
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true)
		content.WriteString(successStyle.Render("✓ Request submitted successfully!"))
		content.WriteString("\n\n")

		detailStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

		content.WriteString(detailStyle.Render(fmt.Sprintf("Request ID: %d", m.submittedRequest.ID)))
		content.WriteString("\n")
		content.WriteString(detailStyle.Render(fmt.Sprintf("Status: %s", GetStatusText(m.submittedRequest.Status))))
		content.WriteString("\n")
	} else if m.error != "" {
		// Error
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		content.WriteString(errorStyle.Render("❌ Request failed"))
		content.WriteString("\n\n")
		content.WriteString(errorStyle.Render(m.error))
		content.WriteString("\n")
	}

	// Instructions
	instructions := []string{
		"Press any key to continue",
		"Esc or Ctrl+C to quit",
	}

	instStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	content.WriteString("\n")
	for _, inst := range instructions {
		content.WriteString(instStyle.Render("• " + inst))
		content.WriteString("\n")
	}

	return content.String()
}
