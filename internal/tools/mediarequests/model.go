package mediarequests

import (
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the state of the Media Requests TUI
type Model struct {
	// Configuration
	config *config.Config
	colors colors.Colors
	api    *APIClient

	// Workflow state
	step Step

	// Search step
	searchInput textinput.Model
	query       string

	// Select step
	results  []SearchResult
	selected int
	loading  bool
	error    string

	// TV show specific
	tvDetails   *TVDetails
	seasons     interface{} // "all" or []int
	seasonInput textinput.Model
	seasonMode  string // "all", "select", ""

	// Server selection
	serviceInstances []ServiceInstance
	selectedServer   *ServiceInstance
	serviceDetails   *ServiceDetails
	overrides        *RequestOverrides

	// Confirmation step
	confirmed bool

	// Submit step
	submittedRequest *MediaRequest

	// UI state
	width  int
	height int
}

// InitialModel creates the initial model for Media Requests
func InitialModel(cfg *config.Config) Model {
	// Initialize API client
	apiClient := NewAPIClient(cfg.Overseerr.URL, cfg.Overseerr.Token, int(cfg.Timeout.Seconds()))

	// Initialize text inputs
	searchInput := textinput.New()
	searchInput.Placeholder = "Enter movie or TV show name..."
	searchInput.Focus()

	seasonInput := textinput.New()
	seasonInput.Placeholder = "e.g., 1,2,3"

	return Model{
		config:      cfg,
		colors:      colors.New(cfg.NoColor),
		api:         apiClient,
		step:        StepSearch,
		searchInput: searchInput,
		seasonInput: seasonInput,
		selected:    0,
		loading:     false,
		seasonMode:  "",
		confirmed:   false,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Test connection on startup
	return tea.Batch(
		m.testConnection(),
		textinput.Blink,
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			cmds = append(cmds, m.handleEnter())

		case tea.KeyUp:
			if m.step == StepSelect {
				if m.selected > 0 {
					m.selected--
				}
			}

		case tea.KeyDown:
			if m.step == StepSelect {
				if m.selected < len(m.results)-1 {
					m.selected++
				}
			}

		case tea.KeyTab:
			// Cycle through form inputs
			if m.step == StepSearch {
				m.searchInput.Blur()
			}

		case tea.KeyBackspace:
			if m.step == StepSelect && m.selected > 0 {
				m.step = StepSearch
				m.searchInput.Focus()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateComponentWidths()

	case SearchResultsMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error.Error()
		} else {
			m.results = msg.Results
			if len(m.results) > 0 {
				m.step = StepSelect
				m.selected = 0
			} else {
				m.error = "No results found"
			}
		}

	case TVDetailsMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error.Error()
		} else {
			m.tvDetails = msg.Details
			// Handle season selection automatically
			if m.tvDetails != nil {
				m.HandleSeasonSelection()
			}
		}

	case ServiceInstancesMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error.Error()
		} else {
			m.serviceInstances = msg.Instances
			if len(m.serviceInstances) > 0 {
				// Select default server or first one
				for _, server := range m.serviceInstances {
					if server.IsDefault {
						m.selectedServer = &server
						break
					}
				}
				if m.selectedServer == nil {
					m.selectedServer = &m.serviceInstances[0]
				}
				// Handle server selection
				m.HandleServerSelection()
			}
		}

	case ServiceDetailsMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error.Error()
		} else {
			m.serviceDetails = msg.Details
			// Move to confirmation step
			m.HandleServiceDetails(msg.Details)
		}

	case SubmitRequestMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error.Error()
		} else {
			m.submittedRequest = msg.Request
			m.step = StepSubmit
		}
	}

	// Delegate to step-specific handlers
	switch m.step {
	case StepConfirm:
		return m.UpdateWithConfirmation(msg)
	case StepSubmit:
		return m.UpdateWithSubmit(msg)
	}

	// Update text inputs
	if m.step == StepSearch {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
		m.query = m.searchInput.Value()
	}

	if m.step == StepSelect && m.seasonMode == "select" {
		var cmd tea.Cmd
		m.seasonInput, cmd = m.seasonInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the model
func (m Model) View() string {
	if m.width <= 0 {
		m.width = 80
	}

	switch m.step {
	case StepSearch:
		return m.renderSearchView()
	case StepSelect:
		return m.renderSelectView()
	case StepConfirm:
		return m.renderConfirmView()
	case StepSubmit:
		return m.renderSubmitView()
	default:
		return "Unknown step"
	}
}

// Helper methods

func (m *Model) handleEnter() tea.Cmd {
	switch m.step {
	case StepSearch:
		if m.query != "" {
			m.loading = true
			m.error = ""
			return m.searchMedia(m.query)
		}
	case StepSelect:
		if len(m.results) > 0 && m.selected < len(m.results) {
			selectedMedia := m.results[m.selected]

			// Check if already available or requested
			if selectedMedia.MediaInfo != nil {
				status := selectedMedia.MediaInfo.Status
				if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
					m.error = "This media is already available!"
					return nil
				}
				if len(selectedMedia.MediaInfo.Requests) > 0 {
					m.error = "This media has already been requested."
					return nil
				}
			}

			// Handle TV show season selection
			if selectedMedia.MediaType == "tv" {
				m.loading = true
				return m.fetchTVDetails(selectedMedia.ID)
			} else {
				// For movies, go directly to server selection
				return m.fetchServiceInstances("radarr")
			}
		}
	}

	return nil
}

func (m *Model) updateComponentWidths() {
	if m.width > 0 {
		m.searchInput.Width = m.width - 4
		m.seasonInput.Width = m.width - 4
	}
}

func (m *Model) testConnection() tea.Cmd {
	return func() tea.Msg {
		if err := m.api.TestConnection(); err != nil {
			return SearchResultsMsg{Error: err}
		}
		return nil
	}
}

func (m *Model) searchMedia(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.api.SearchMedia(query)
		return SearchResultsMsg{Results: results, Error: err}
	}
}

func (m *Model) fetchTVDetails(tmdbID int) tea.Cmd {
	return func() tea.Msg {
		details, err := m.api.GetTVDetails(tmdbID)
		return TVDetailsMsg{Details: details, Error: err}
	}
}

func (m *Model) fetchServiceInstances(service string) tea.Cmd {
	return func() tea.Msg {
		instances, err := m.api.FetchServiceInstances(service)
		return ServiceInstancesMsg{Instances: instances, Error: err}
	}
}

func (m *Model) submitRequest() tea.Cmd {
	if len(m.results) == 0 || m.selected >= len(m.results) {
		return nil
	}

	selectedMedia := m.results[m.selected]

	return func() tea.Msg {
		request, err := m.api.CreateRequest(selectedMedia, m.seasons, m.overrides)
		return SubmitRequestMsg{Request: request, Error: err}
	}
}
