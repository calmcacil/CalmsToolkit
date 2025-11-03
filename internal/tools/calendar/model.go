package calendar

import (
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/bubbletea"
)

// Model represents the calendar TUI model
type Model struct {
	// Configuration
	config *config.Config
	colors colors.Colors

	// API client
	apiClient *APIClient

	// View state
	viewMode    ViewMode
	dateRange   DateRange
	currentDate time.Time

	// Data
	items       []CalendarItem
	queueIssues []QueueIssue
	loading     bool
	error       string

	// UI state
	width      int
	height     int
	ready      bool
	lastUpdate time.Time
}

// NewModel creates a new calendar model
func NewModel(cfg *config.Config) *Model {
	apiClient := NewAPIClient(
		cfg.SonarrURLs,
		cfg.SonarrTokens,
		cfg.RadarrURLs,
		cfg.RadarrTokens,
		cfg.Timeout,
		cfg.Debug,
	)

	now := time.Now()
	dateRange := NewDateRange(DayView, now, 0)

	return &Model{
		config:      cfg,
		colors:      colors.New(cfg.NoColor),
		apiClient:   apiClient,
		viewMode:    DayView,
		dateRange:   dateRange,
		currentDate: now,
		loading:     true,
	}
}

// Init initializes the calendar model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchCalendar(),
		tea.WindowSize(),
	)
}

// fetchCalendar returns a command to fetch calendar data
func (m *Model) fetchCalendar() tea.Cmd {
	return func() tea.Msg {
		items, queueIssues, err := m.apiClient.FetchCalendar(m.dateRange)
		if err != nil {
			return fetchFailedMsg{err: err}
		}
		return calendarFetchedMsg{
			items:       items,
			queueIssues: queueIssues,
		}
	}
}

// Messages
type calendarFetchedMsg struct {
	items       []CalendarItem
	queueIssues []QueueIssue
}

type fetchFailedMsg struct {
	err error
}

type viewModeChangedMsg struct {
	viewMode ViewMode
}

type dateNavigatedMsg struct {
	dateRange DateRange
}

type refreshMsg struct{}

// Refresh returns a command to refresh calendar data
func Refresh() tea.Cmd {
	return func() tea.Msg {
		return refreshMsg{}
	}
}
