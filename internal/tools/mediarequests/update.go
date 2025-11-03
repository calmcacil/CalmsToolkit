package mediarequests

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// UpdateWithConfirmation handles the confirmation step logic
func (m Model) UpdateWithConfirmation(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			// Submit the request
			m.loading = true
			m.error = ""
			return m, m.submitRequest()

		case "n", "N":
			// Go back to selection
			m.step = StepSelect
			return m, nil
		}
	}

	return m, nil
}

// UpdateWithSubmit handles the submit completion logic
func (m Model) UpdateWithSubmit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		// Any key continues
		return m, tea.Quit
	}

	return m, nil
}

// HandleSeasonSelection handles TV show season selection
func (m *Model) HandleSeasonSelection() tea.Cmd {
	if m.tvDetails == nil {
		return nil
	}

	// Default to all seasons for simplicity
	m.seasons = "all"

	// Fetch service instances for TV shows (Sonarr)
	return m.fetchServiceInstances("sonarr")
}

// HandleServerSelection handles server and root folder selection
func (m *Model) HandleServerSelection() tea.Cmd {
	if len(m.serviceInstances) == 0 {
		return nil
	}

	// For simplicity, use the first/default server
	if m.selectedServer == nil {
		m.selectedServer = &m.serviceInstances[0]
	}

	// Create overrides
	m.overrides = &RequestOverrides{
		ServerID:   m.selectedServer.ID,
		ServerName: m.selectedServer.Name,
	}

	// Fetch service details for root folder selection
	service := "radarr"
	if len(m.results) > 0 && m.selected < len(m.results) {
		if m.results[m.selected].MediaType == "tv" {
			service = "sonarr"
		}
	}

	return m.fetchServiceDetails(service, m.selectedServer.ID)
}

// HandleServiceDetails handles service details response
func (m *Model) HandleServiceDetails(details *ServiceDetails) {
	if details == nil {
		return
	}

	// For simplicity, use the first root folder if available
	if len(details.RootFolders) > 0 {
		if m.overrides == nil {
			m.overrides = &RequestOverrides{}
		}
		m.overrides.RootFolder = details.RootFolders[0].Path
	}

	// Move to confirmation step
	m.step = StepConfirm
}

// fetchServiceDetails is a helper method to fetch service details
func (m *Model) fetchServiceDetails(service string, id int) tea.Cmd {
	return func() tea.Msg {
		details, err := m.api.FetchServiceDetails(service, id)
		return ServiceDetailsMsg{Details: details, Error: err}
	}
}

// ValidateSelection validates the current selection
func (m *Model) ValidateSelection() error {
	if len(m.results) == 0 {
		return fmt.Errorf("no results available")
	}

	if m.selected < 0 || m.selected >= len(m.results) {
		return fmt.Errorf("invalid selection")
	}

	selectedMedia := m.results[m.selected]

	// Check if already available or requested
	if selectedMedia.MediaInfo != nil {
		status := selectedMedia.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			return fmt.Errorf("this media is already available")
		}
		if len(selectedMedia.MediaInfo.Requests) > 0 {
			return fmt.Errorf("this media has already been requested")
		}
	}

	return nil
}

// ResetError clears any current error
func (m *Model) ResetError() {
	m.error = ""
}

// SetError sets an error message
func (m *Model) SetError(err error) {
	if err != nil {
		m.error = err.Error()
	} else {
		m.error = ""
	}
}

// IsLoading returns true if currently loading
func (m Model) IsLoading() bool {
	return m.loading
}

// SetLoading sets the loading state
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
	if loading {
		m.error = ""
	}
}

// GetSelectedMedia returns the currently selected media
func (m Model) GetSelectedMedia() *SearchResult {
	if len(m.results) == 0 || m.selected < 0 || m.selected >= len(m.results) {
		return nil
	}
	return &m.results[m.selected]
}

// FormatMediaTitle formats a media title with year
func (m Model) FormatMediaTitle(result SearchResult) string {
	title := GetTitle(result)
	year := GetYear(result)

	if year != "" {
		return fmt.Sprintf("%s (%s)", title, year)
	}
	return title
}

// FormatMediaType formats media type with icon
func (m Model) FormatMediaType(mediaType string) string {
	switch strings.ToLower(mediaType) {
	case "movie":
		return "🎬 Movie"
	case "tv":
		return "📺 TV Show"
	default:
		return strings.Title(mediaType)
	}
}

// TruncateText truncates text to fit within width
func (m Model) TruncateText(text string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	return text[:maxWidth-3] + "..."
}
