package calendar

import (
	"time"

	"github.com/charmbracelet/bubbletea"
)

// Update handles calendar model updates
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case calendarFetchedMsg:
		m.items = msg.items
		m.queueIssues = msg.queueIssues
		m.loading = false
		m.error = ""
		m.lastUpdate = time.Now()
		return m, nil

	case fetchFailedMsg:
		m.loading = false
		m.error = msg.err.Error()
		return m, nil

	case viewModeChangedMsg:
		m.viewMode = msg.viewMode
		m.dateRange = NewDateRange(msg.viewMode, m.currentDate, 0)
		m.loading = true
		return m, m.fetchCalendar()

	case dateNavigatedMsg:
		m.dateRange = msg.dateRange
		m.currentDate = msg.dateRange.Start
		m.loading = true
		return m, m.fetchCalendar()

	case refreshMsg:
		m.loading = true
		return m, m.fetchCalendar()

	default:
		return m, nil
	}
}

// handleKeyMsg handles keyboard input
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyRight:
		// Navigate forward in time
		newRange := m.dateRange.Navigate(1, m.viewMode)
		return m, func() tea.Msg {
			return dateNavigatedMsg{dateRange: newRange}
		}

	case tea.KeyLeft:
		// Navigate backward in time
		newRange := m.dateRange.Navigate(-1, m.viewMode)
		return m, func() tea.Msg {
			return dateNavigatedMsg{dateRange: newRange}
		}

	case tea.KeyTab:
		// Cycle through view modes
		nextMode := ViewMode((int(m.viewMode) + 1) % 3)
		return m, func() tea.Msg {
			return viewModeChangedMsg{viewMode: nextMode}
		}

	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			switch msg.Runes[0] {
			case 'l':
				// Navigate forward in time
				newRange := m.dateRange.Navigate(1, m.viewMode)
				return m, func() tea.Msg {
					return dateNavigatedMsg{dateRange: newRange}
				}

			case 'h':
				// Navigate backward in time
				newRange := m.dateRange.Navigate(-1, m.viewMode)
				return m, func() tea.Msg {
					return dateNavigatedMsg{dateRange: newRange}
				}

			case '1':
				// Switch to day view
				return m, func() tea.Msg {
					return viewModeChangedMsg{viewMode: DayView}
				}

			case '2':
				// Switch to week view
				return m, func() tea.Msg {
					return viewModeChangedMsg{viewMode: WeekView}
				}

			case '3':
				// Switch to month view
				return m, func() tea.Msg {
					return viewModeChangedMsg{viewMode: MonthView}
				}

			case 'r':
				// Refresh data
				return m, m.fetchCalendar()

			case 't':
				// Jump to today
				today := time.Now()
				newRange := NewDateRange(m.viewMode, today, 0)
				return m, func() tea.Msg {
					return dateNavigatedMsg{dateRange: newRange}
				}
			}
		}
	}

	return m, nil
}

// ChangeViewMode changes the view mode
func (m *Model) ChangeViewMode(mode ViewMode) tea.Cmd {
	return func() tea.Msg {
		return viewModeChangedMsg{viewMode: mode}
	}
}

// NavigateDate navigates to a new date range
func (m *Model) NavigateDate(direction int) tea.Cmd {
	newRange := m.dateRange.Navigate(direction, m.viewMode)
	return func() tea.Msg {
		return dateNavigatedMsg{dateRange: newRange}
	}
}

// GoToToday navigates to today's date
func (m *Model) GoToToday() tea.Cmd {
	today := time.Now()
	newRange := NewDateRange(m.viewMode, today, 0)
	return func() tea.Msg {
		return dateNavigatedMsg{dateRange: newRange}
	}
}
