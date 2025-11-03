package app

import (
	"github.com/calmcacil/CalmsToolkit/internal/components"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the main application model
type Model struct {
	config    *config.Config
	colors    colors.Colors
	tabs      *components.Tabs
	header    *components.Header
	statusBar *components.StatusBar
	ready     bool
	quitting  bool
	lastError string
}

// Tab represents the available tabs
type Tab int

const (
	MediaRequestsTab Tab = iota
	StreamsTab
	CalendarTab
	QueueTab
	ArrFeedTab
)

// String returns the string representation of a tab
func (t Tab) String() string {
	switch t {
	case MediaRequestsTab:
		return "Media Requests"
	case StreamsTab:
		return "Streams"
	case CalendarTab:
		return "Calendar"
	case QueueTab:
		return "Queue"
	case ArrFeedTab:
		return "ARR Feed"
	default:
		return "Unknown"
	}
}

// InitialModel creates the initial application model
func InitialModel() Model {
	cfg, err := config.LoadConfig()
	if err != nil {
		// For now, create a default config if loading fails
		cfg = &config.Config{
			Global: config.GlobalConfig{
				NoColor: false,
				Timeout: 30 * 1000000000, // 30 seconds in nanoseconds
				Debug:   false,
			},
		}
	}

	// Sanitize the configuration
	cfg.Sanitize()

	colors := colors.NewWithOverride(cfg.Global.NoColor)

	return Model{
		config:    cfg,
		colors:    colors,
		tabs:      components.NewTabs(colors),
		header:    components.NewHeader("CalmsToolkit TUI", colors),
		statusBar: components.NewStatusBar(colors),
		ready:     false,
		quitting:  false,
		lastError: "",
	}
}

// Init initializes the application model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles application updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyTab, tea.KeyRight:
			m.tabs.Next()
			m.updateHeader()
			return m, nil

		case tea.KeyLeft:
			m.tabs.Previous()
			m.updateHeader()
			return m, nil

		default:
			// Let tabs handle navigation
			cmd := m.tabs.Update(msg)
			if cmd != nil {
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)

	default:
		// Update components
		cmd := m.tabs.Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

// View renders the application
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// Update status based on current state
	if m.lastError != "" {
		m.statusBar.SetError(m.lastError)
	} else if !m.ready {
		m.statusBar.SetLoading(true)
	} else {
		m.statusBar.SetStatus("Ready")
	}

	// Build the view
	content := ""
	content += m.header.View() + "\n\n"
	content += m.tabs.View() + "\n\n"

	// Add content for the active tab
	activeTab := m.tabs.GetActive()
	switch Tab(activeTab) {
	case MediaRequestsTab:
		content += m.renderMediaRequestsContent()
	case StreamsTab:
		content += m.renderStreamsContent()
	case CalendarTab:
		content += m.renderCalendarContent()
	case QueueTab:
		content += m.renderQueueContent()
	case ArrFeedTab:
		content += m.renderArrFeedContent()
	}

	content += "\n\n" + m.statusBar.View()

	return content
}

// updateHeader updates the header based on the active tab
func (m *Model) updateHeader() {
	activeTab := m.tabs.GetActiveTab()
	m.header.SetTitle("CalmsToolkit TUI - " + activeTab.Name)
}

// handleWindowSize handles window size changes
func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) {
	m.ready = true

	// Update component widths
	m.tabs.SetWidth(msg.Width)
	m.header.SetWidth(msg.Width)
	m.statusBar.SetWidth(msg.Width)
}

// renderMediaRequestsContent renders the media requests tab content
func (m Model) renderMediaRequestsContent() string {
	return m.colors.CyanText("Media Requests tool will be implemented in Phase 2.\n\n") +
		m.colors.GrayText("This tab will allow you to:\n") +
		m.colors.GrayText("• Search for movies and TV shows\n") +
		m.colors.GrayText("• Submit media requests\n") +
		m.colors.GrayText("• View request status\n") +
		m.colors.GrayText("• Manage existing requests")
}

// renderStreamsContent renders the streams tab content
func (m Model) renderStreamsContent() string {
	return m.colors.CyanText("Media Streams tool will be implemented in Phase 2.\n\n") +
		m.colors.GrayText("This tab will allow you to:\n") +
		m.colors.GrayText("• Monitor active streaming sessions\n") +
		m.colors.GrayText("• View playback history\n") +
		m.colors.GrayText("• Track user activity\n") +
		m.colors.GrayText("• Monitor bandwidth usage")
}

// renderCalendarContent renders the calendar tab content
func (m Model) renderCalendarContent() string {
	return m.colors.CyanText("Media Calendar tool will be implemented in Phase 2.\n\n") +
		m.colors.GrayText("This tab will allow you to:\n") +
		m.colors.GrayText("• View upcoming episode releases\n") +
		m.colors.GrayText("• Track movie release dates\n") +
		m.colors.GrayText("• Browse by date/week/month\n") +
		m.colors.GrayText("• Filter by series or status")
}

// renderQueueContent renders the queue tab content
func (m Model) renderQueueContent() string {
	return m.colors.CyanText("Queue Remediation tool will be implemented in Phase 2.\n\n") +
		m.colors.GrayText("This tab will allow you to:\n") +
		m.colors.GrayText("• View download queues\n") +
		m.colors.GrayText("• Identify stuck items\n") +
		m.colors.GrayText("• Retry failed downloads\n") +
		m.colors.GrayText("• Monitor queue health")
}

// renderArrFeedContent renders the ARR feed tab content
func (m Model) renderArrFeedContent() string {
	return m.colors.CyanText("ARR Feed tool will be implemented in Phase 3.\n\n") +
		m.colors.GrayText("This tab will allow you to:\n") +
		m.colors.GrayText("• View recent additions\n") +
		m.colors.GrayText("• Monitor library changes\n") +
		m.colors.GrayText("• Track import status\n") +
		m.colors.GrayText("• Browse by media type")
}
