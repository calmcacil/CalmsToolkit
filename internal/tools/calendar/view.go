package calendar

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/pkg/colors"
)

// View renders the calendar model
func (m *Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var builder strings.Builder

	// Render header
	builder.WriteString(m.renderHeader())
	builder.WriteString("\n\n")

	// Render queue warnings if any
	if len(m.queueIssues) > 0 {
		builder.WriteString(m.renderQueueWarnings())
		builder.WriteString("\n")
	}

	// Render loading or error state
	if m.loading {
		builder.WriteString(m.colors.BoldStyle(colors.Yellow).Render("⏳ Loading calendar data..."))
		return builder.String()
	}

	if m.error != "" {
		builder.WriteString(m.colors.BoldStyle(colors.Red).Render("❌ Error: " + m.error))
		return builder.String()
	}

	// Render calendar content
	builder.WriteString(m.renderCalendar())

	// Render footer
	builder.WriteString("\n\n")
	builder.WriteString(m.renderFooter())

	return builder.String()
}

// renderHeader renders the calendar header
func (m *Model) renderHeader() string {
	title := fmt.Sprintf("📅 Media Calendar - %s View", m.viewMode.String())
	dateRange := fmt.Sprintf("%s - %s",
		m.dateRange.Start.Format("Jan 02, 2006"),
		m.dateRange.End.AddDate(0, 0, -1).Format("Jan 02, 2006"))

	titleStyle := m.colors.BoldStyle(colors.Cyan)
	dateStyle := m.colors.Style(colors.Gray)

	return fmt.Sprintf("%s  %s", titleStyle.Render(title), dateStyle.Render(dateRange))
}

// renderQueueWarnings renders queue issue warnings
func (m *Model) renderQueueWarnings() string {
	totalIssues := 0
	for _, issue := range m.queueIssues {
		totalIssues += issue.Count
	}

	warningStyle := m.colors.BoldStyle(colors.Red)
	serviceStyle := m.colors.Style(colors.Yellow)
	urlStyle := m.colors.Style(colors.Blue)

	var builder strings.Builder
	builder.WriteString(warningStyle.Render(fmt.Sprintf("⚠️  WARNING: %d items require manual intervention", totalIssues)))
	builder.WriteString("\n")

	for _, issue := range m.queueIssues {
		builder.WriteString(fmt.Sprintf("   %s: %s",
			serviceStyle.Render(issue.ServiceName),
			urlStyle.Render(issue.URL)))
		builder.WriteString("\n")
	}

	return builder.String()
}

// renderCalendar renders the main calendar content
func (m *Model) renderCalendar() string {
	if len(m.items) == 0 {
		return m.colors.Style(colors.Green).Render("No scheduled releases")
	}

	// Group items by day
	dayGroups := make(map[string][]CalendarItem)
	for _, item := range m.items {
		dayKey := item.AirTime.Format("2006-01-02")
		dayGroups[dayKey] = append(dayGroups[dayKey], item)
	}

	// Sort days
	var sortedDays []string
	for dayKey := range dayGroups {
		sortedDays = append(sortedDays, dayKey)
	}
	sort.Strings(sortedDays)

	var builder strings.Builder

	// Render each day
	for _, dayKey := range sortedDays {
		dayItems := dayGroups[dayKey]
		date, _ := time.Parse("2006-01-02", dayKey)

		builder.WriteString(m.renderDay(date, dayItems))
		builder.WriteString("\n")
	}

	return builder.String()
}

// renderDay renders a single day's items
func (m *Model) renderDay(date time.Time, items []CalendarItem) string {
	dateStyle := m.colors.BoldStyle(colors.Cyan)
	separatorStyle := m.colors.Style(colors.Cyan)

	var builder strings.Builder

	// Day header
	builder.WriteString(dateStyle.Render(date.Format("Mon, Jan 02, 2006")))
	builder.WriteString("\n")
	builder.WriteString(separatorStyle.Render(strings.Repeat("─", 40)))
	builder.WriteString("\n")

	// Sort items by time, then type, then season/episode
	sort.Slice(items, func(i, j int) bool {
		if !items[i].AirTime.Equal(items[j].AirTime) {
			return items[i].AirTime.Before(items[j].AirTime)
		}
		if items[i].Type != items[j].Type {
			return items[i].Type == "episode"
		}
		if items[i].Type == "episode" {
			if items[i].Season != items[j].Season {
				return items[i].Season < items[j].Season
			}
			return items[i].Episode < items[j].Episode
		}
		return false
	})

	// Group episodes by show
	showEpisodes := make(map[string][]CalendarItem)
	var movies []CalendarItem

	for _, item := range items {
		if item.Type == "episode" {
			showEpisodes[item.ShowTitle] = append(showEpisodes[item.ShowTitle], item)
		} else {
			movies = append(movies, item)
		}
	}

	// Render episodes
	for _, episodes := range showEpisodes {
		maxDisplay := 3
		for i, ep := range episodes {
			if i >= maxDisplay {
				remaining := len(episodes) - maxDisplay
				builder.WriteString(fmt.Sprintf("  %s+ %d more episodes%s\n",
					m.colors.Style(colors.Cyan).Render(""),
					remaining,
					m.colors.Style(colors.Cyan).Render("")))
				break
			}

			builder.WriteString(m.renderEpisode(ep))
		}
	}

	// Render movies
	for _, movie := range movies {
		builder.WriteString(m.renderMovie(movie))
	}

	return builder.String()
}

// renderEpisode renders a single episode
func (m *Model) renderEpisode(ep CalendarItem) string {
	statusColor := m.getStatusColor(ep)
	timeStr := ep.AirTime.Format("15:04")

	timeStyle := m.colors.BoldStyle(colors.Gray)
	showStyle := m.colors.BoldStyle(statusColor)
	titleStyle := m.colors.Style(statusColor)

	var builder strings.Builder

	// Line 1: "HH:MM SHOWNAME - S##E##"
	builder.WriteString(fmt.Sprintf("  %s %s%s - S%02dE%02d%s\n",
		timeStyle.Render(timeStr),
		showStyle.Render(ep.ShowTitle),
		titleStyle.Render(""),
		ep.Season,
		ep.Episode,
		titleStyle.Render("")))

	// Line 2: "       EPISODE_TITLE"
	builder.WriteString(fmt.Sprintf("         %s\n",
		titleStyle.Render(ep.Title)))

	return builder.String()
}

// renderMovie renders a single movie
func (m *Model) renderMovie(movie CalendarItem) string {
	statusColor := m.getStatusColor(movie)
	timeStr := movie.AirTime.Format("15:04")

	timeStyle := m.colors.BoldStyle(colors.Gray)
	titleStyle := m.colors.Style(statusColor)

	return fmt.Sprintf("  %s %s (%d)\n",
		timeStyle.Render(timeStr),
		titleStyle.Render(movie.Title),
		movie.Year)
}

// getStatusColor returns the appropriate color for an item's status
func (m *Model) getStatusColor(item CalendarItem) string {
	now := time.Now()

	if item.HasFile {
		return colors.Green
	}

	if item.AirTime.Before(now) {
		return colors.Red
	}

	if item.IsPremiere {
		return colors.Orange
	}

	return colors.Blue
}

// renderFooter renders the footer with help text
func (m *Model) renderFooter() string {
	helpStyle := m.colors.Style(colors.Gray)
	keys := []string{
		"←/h: Previous",
		"→/l: Next",
		"1/2/3: Day/Week/Month",
		"Tab: Cycle views",
		"t: Today",
		"r: Refresh",
		"q: Quit",
	}

	return helpStyle.Render(strings.Join(keys, " • "))
}
