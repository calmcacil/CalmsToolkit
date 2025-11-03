package app

import (
	"github.com/calmcacil/CalmsToolkit/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

type Tab int

const (
	MediaRequestsTab Tab = iota
	StreamsTab
	CalendarTab
	QueueTab
	ArrFeedTab
)

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

func (t Tab) Icon() string {
	switch t {
	case MediaRequestsTab:
		return "📋"
	case StreamsTab:
		return "🎬"
	case CalendarTab:
		return "📅"
	case QueueTab:
		return "⏳"
	case ArrFeedTab:
		return "📡"
	default:
		return "❓"
	}
}

type model struct {
	// Navigation
	currentTab Tab
	tabs       []Tab

	// Global state
	config  *config.Config
	loading bool
	error   string
	quit    bool

	// UI state
	ready      bool
	dimensions tea.WindowSizeMsg

	// Tool states (placeholders for now)
	mediaRequests interface{}
	streams       interface{}
	calendar      interface{}
	queue         interface{}
	arrFeed       interface{}
}

func InitialModel() tea.Model {
	tabs := []Tab{
		MediaRequestsTab,
		StreamsTab,
		CalendarTab,
		QueueTab,
		ArrFeedTab,
	}

	return model{
		currentTab: MediaRequestsTab,
		tabs:       tabs,
		loading:    true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadConfig,
		tea.WindowSize(),
	)
}

type configLoadedMsg struct {
	config *config.Config
	err    error
}

func loadConfig() tea.Msg {
	cfg, err := config.LoadConfig()
	return configLoadedMsg{config: cfg, err: err}
}
