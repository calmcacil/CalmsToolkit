package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	tabStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			Padding(1, 2)

	statusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Width(80)
)

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var b strings.Builder

	// Header with title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Width(m.dimensions.Width).
		Align(lipgloss.Center).
		Render("🎬 CalmsToolkit TUI")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Tab navigation
	tabs := make([]string, len(m.tabs))
	for i, tab := range m.tabs {
		style := tabStyle
		if i == int(m.currentTab) {
			style = activeTabStyle
		}
		tabs[i] = style.Render(fmt.Sprintf("%s %s", tab.Icon(), tab.String()))
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	b.WriteString(lipgloss.NewStyle().Width(m.dimensions.Width).Align(lipgloss.Center).Render(tabRow))
	b.WriteString("\n\n")

	// Content area
	content := m.renderContent()
	b.WriteString(contentStyle.Width(m.dimensions.Width - 4).Height(m.dimensions.Height - 10).Render(content))
	b.WriteString("\n\n")

	// Status bar
	status := m.renderStatus()
	b.WriteString(statusStyle.Width(m.dimensions.Width).Render(status))

	return b.String()
}

func (m model) renderContent() string {
	if m.loading {
		return "⏳ Loading configuration..."
	}

	if m.error != "" {
		return fmt.Sprintf("❌ Error: %s", m.error)
	}

	switch m.currentTab {
	case MediaRequestsTab:
		return m.renderMediaRequestsContent()
	case StreamsTab:
		return m.renderStreamsContent()
	case CalendarTab:
		return m.renderCalendarContent()
	case QueueTab:
		return m.renderQueueContent()
	case ArrFeedTab:
		return m.renderArrFeedContent()
	default:
		return "Unknown tab"
	}
}

func (m model) renderMediaRequestsContent() string {
	return `📋 Media Requests Tool

This tool allows you to:
• Search for movies and TV shows
• Request new content
• View request status
• Manage existing requests

Features coming soon:
• Real-time search
• Request approval workflow
• Status tracking

Press Tab to navigate between tools.`
}

func (m model) renderStreamsContent() string {
	return `🎬 Media Streams Tool

This tool allows you to:
• Monitor active streaming sessions
• View playback history
• Track user activity
• Analyze streaming patterns

Features coming soon:
• Real-time session monitoring
• User analytics
• Bandwidth usage tracking

Press Tab to navigate between tools.`
}

func (m model) renderCalendarContent() string {
	return `📅 Media Calendar Tool

This tool allows you to:
• View upcoming releases
• Track media availability
• Plan viewing schedule
• Monitor release dates

Features coming soon:
• Interactive calendar view
• Release notifications
• Personal watchlist integration

Press Tab to navigate between tools.`
}

func (m model) renderQueueContent() string {
	return `⏳ Queue Remediation Tool

This tool allows you to:
• Monitor download queues
• Identify stuck items
• Retry failed downloads
• Optimize queue performance

Features coming soon:
• Real-time queue monitoring
• Automatic retry logic
• Performance analytics

Press Tab to navigate between tools.`
}

func (m model) renderArrFeedContent() string {
	return `📡 ARR Feed Tool

This tool allows you to:
• Monitor Sonarr/Radarr activity
• Track new releases
• View import history
• Analyze feed patterns

Features coming soon:
• Real-time feed monitoring
• Release filtering
• Import statistics

Press Tab to navigate between tools.`
}

func (m model) renderStatus() string {
	if m.loading {
		return "⏳ Loading..."
	}

	if m.error != "" {
		return fmt.Sprintf("❌ %s", m.error)
	}

	if m.config != nil {
		status := "✅ Ready"
		if m.config.Global.Debug {
			status += " | Debug mode"
		}
		return fmt.Sprintf("%s | %s", status, m.currentTab.String())
	}

	return "Ready"
}
