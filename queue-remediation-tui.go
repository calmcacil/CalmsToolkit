//go:build queueremediation

package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TUIModel represents the state of the TUI application
type TUIModel struct {
	config        Config
	items         []QueueItem
	currentIndex  int
	loading       bool
	error         string
	status        string
	width         int
	height        int
	quitting      bool
	debugMessages []string
	showDebug     bool
	debugScroll   int
}

// TUI messages for state updates
type itemsLoadedMsg struct {
	items []QueueItem
	err   error
}

type actionExecutedMsg struct {
	success      bool
	err          error
	action       string
	debugLog     []string
	verboseLog   []string
	apiCallCount int
}

type debugLogMsg struct {
	message string
}

// Styles for the TUI components
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Width(80)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3C8F7C")).
			Padding(0, 1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#F25D4C")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#E74C3C")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4A90E2")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#27AE60")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#2C3E50")).
			Padding(0, 1).
			MarginTop(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3498DB")).
			Padding(0, 1).
			MarginTop(1)

	actionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#34495E")).
			Padding(0, 1).
			MarginTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#95A5A6")).
			Padding(0, 1).
			MarginTop(1)

	debugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E67E22")).
			Background(lipgloss.Color("#1C1C1C")).
			Padding(0, 1)

	verboseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3498DB")).
			Padding(0, 1)

	debugHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#E67E22")).
				Padding(0, 1)
)

// InitialModel creates the initial TUI model
func InitialModel(config Config) TUIModel {
	return TUIModel{
		config:        config,
		items:         []QueueItem{},
		currentIndex:  0,
		loading:       true,
		status:        "Loading queue items...",
		width:         80,
		height:        24,
		debugMessages: []string{},
		showDebug:     config.Verbose || config.Debug,
		debugScroll:   0,
	}
}

// Init initializes the TUI application
func (m TUIModel) Init() tea.Cmd {
	return loadItems(m.config)
}

// Update handles TUI updates and user input
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyUp:
			if m.currentIndex > 0 {
				m.currentIndex--
			}

		case tea.KeyDown:
			if m.currentIndex < len(m.items)-1 {
				m.currentIndex++
			}

		case tea.KeyPgUp:
			// Scroll debug log up
			if m.showDebug && m.debugScroll > 0 {
				m.debugScroll--
			}

		case tea.KeyPgDown:
			// Scroll debug log down
			if m.showDebug && len(m.debugMessages) > 10 {
				maxScroll := len(m.debugMessages) - 10
				if m.debugScroll < maxScroll {
					m.debugScroll++
				}
			}

		case tea.KeyEnter:
			if len(m.items) > 0 {
				m.loading = true
				m.status = "Executing suggested action..."
				m.error = ""
				m.debugMessages = []string{} // Clear previous debug logs
				return m, executeSuggestedAction(m.config, m.items[m.currentIndex])
			}

		case tea.KeyRunes:
			if len(msg.Runes) > 0 {
				switch msg.Runes[0] {
				case 'd':
					if len(m.items) > 0 {
						m.loading = true
						m.status = "Deleting queue item..."
						m.error = ""
						m.debugMessages = []string{} // Clear previous debug logs
						return m, executeAction(m.config, m.items[m.currentIndex], "delete")
					}
				case 'm':
					if len(m.items) > 0 {
						m.loading = true
						m.status = "Executing manual import..."
						m.error = ""
						m.debugMessages = []string{} // Clear previous debug logs
						return m, executeAction(m.config, m.items[m.currentIndex], "manual_import")
					}
				case 's':
					if len(m.items) > 0 {
						m.loading = true
						m.status = "Skipping item..."
						m.error = ""
						m.debugMessages = []string{} // Clear previous debug logs
						return m, executeAction(m.config, m.items[m.currentIndex], "monitor")
					}
				case 'r':
					m.loading = true
					m.status = "Refreshing queue items..."
					m.error = ""
					return m, loadItems(m.config)
				case 'q':
					m.quitting = true
					return m, tea.Quit
				case 'D':
					// Toggle debug view
					m.showDebug = !m.showDebug
					if m.showDebug {
						m.status = "Debug view enabled"
					} else {
						m.status = "Debug view disabled"
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case itemsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.error = fmt.Sprintf("Failed to load items: %v", msg.err)
		} else {
			m.items = filterItems(msg.items)
			if len(m.items) == 0 {
				m.status = "No queue items requiring remediation found"
			} else {
				m.status = fmt.Sprintf("Loaded %d items requiring remediation", len(m.items))
			}
		}

	case actionExecutedMsg:
		m.loading = false

		// Append debug and verbose logs to debugMessages
		if len(msg.debugLog) > 0 || len(msg.verboseLog) > 0 {
			m.debugMessages = append(m.debugMessages, msg.verboseLog...)
			m.debugMessages = append(m.debugMessages, msg.debugLog...)
			// Keep only last 500 messages to prevent memory issues
			if len(m.debugMessages) > 500 {
				m.debugMessages = m.debugMessages[len(m.debugMessages)-500:]
			}
		}

		if msg.err != nil {
			m.error = fmt.Sprintf("Action '%s' failed: %v", msg.action, msg.err)
			if msg.apiCallCount > 0 {
				m.error += fmt.Sprintf(" (%d API calls made)", msg.apiCallCount)
			}
		} else {
			m.status = fmt.Sprintf("Successfully executed: %s", msg.action)
			if msg.apiCallCount > 0 {
				m.status += fmt.Sprintf(" (%d API calls)", msg.apiCallCount)
			}
			// Remove the item from the list after successful action
			if m.currentIndex < len(m.items) {
				m.items = append(m.items[:m.currentIndex], m.items[m.currentIndex+1:]...)
				if m.currentIndex >= len(m.items) && m.currentIndex > 0 {
					m.currentIndex--
				}
			}
			if len(m.items) == 0 {
				m.status = "All items processed successfully"
			}
		}

	case debugLogMsg:
		m.debugMessages = append(m.debugMessages, msg.message)
		if len(m.debugMessages) > 500 {
			m.debugMessages = m.debugMessages[len(m.debugMessages)-500:]
		}
	}

	return m, nil
}

// View renders the TUI interface
func (m TUIModel) View() string {
	if m.quitting {
		return ""
	}

	var content strings.Builder

	// Header
	header := headerStyle.Render("Queue Remediation (Manual Mode)")
	content.WriteString(header)
	content.WriteString("\n")

	// Status bar
	if m.loading {
		content.WriteString(statusStyle.Render("Loading..."))
	} else if m.error != "" {
		content.WriteString(errorStyle.Render(m.error))
	} else {
		progress := ""
		if len(m.items) > 0 {
			progress = fmt.Sprintf("Item %d/%d", m.currentIndex+1, len(m.items))
		}
		content.WriteString(statusStyle.Render(fmt.Sprintf("%s | %s", progress, m.status)))
	}
	content.WriteString("\n")

	// Main content area
	if m.loading {
		content.WriteString(infoStyle.Render("Fetching queue items from all instances..."))
	} else if len(m.items) == 0 && m.error == "" {
		content.WriteString(infoStyle.Render("No queue items requiring remediation found."))
	} else if len(m.items) > 0 {
		item := m.items[m.currentIndex]

		// Item header
		instanceName := getInstanceName(m.config, item.InstanceURL, item.InstanceType)
		itemHeader := fmt.Sprintf("%s - %s", instanceName, item.Title)
		if m.currentIndex < len(m.items) {
			content.WriteString(selectedStyle.Render(itemHeader))
		} else {
			content.WriteString(itemStyle.Render(itemHeader))
		}
		content.WriteString("\n")

		// Item details
		details := fmt.Sprintf("Status: %s | State: %s", item.Status, item.TrackedDownloadState)
		if item.DownloadClient != "" {
			details += fmt.Sprintf(" | Client: %s", item.DownloadClient)
		}
		content.WriteString(infoStyle.Render(details))
		content.WriteString("\n\n")

		// Status messages
		if len(item.StatusMessages) > 0 {
			content.WriteString(infoStyle.Render("Status Messages:"))
			content.WriteString("\n")
			for _, sm := range item.StatusMessages {
				for _, msg := range sm.Messages {
					content.WriteString(fmt.Sprintf("  • %s\n", msg))
				}
			}
			content.WriteString("\n")
		}

		// Recommended action
		action, _, _ := mapStatusToAction(item)
		var actionText string
		switch action {
		case "delete":
			actionText = "DELETE"
		case "manual_import":
			actionText = "MANUAL_IMPORT"
		case "monitor":
			actionText = "MONITOR"
		default:
			actionText = "MONITOR"
		}

		reason := getReason(item)
		content.WriteString(infoStyle.Render(fmt.Sprintf("Recommended Action: %s", actionText)))
		content.WriteString("\n")
		content.WriteString(infoStyle.Render(fmt.Sprintf("→ %s", reason)))
		content.WriteString("\n")
	}

	// Debug/Verbose output section (if enabled and has messages)
	if m.showDebug && len(m.debugMessages) > 0 {
		content.WriteString("\n")
		content.WriteString(debugHeaderStyle.Render("Debug Output (PgUp/PgDn to scroll)"))
		content.WriteString("\n")

		// Calculate how many lines we can show
		maxLines := 10
		startIdx := m.debugScroll
		endIdx := startIdx + maxLines
		if endIdx > len(m.debugMessages) {
			endIdx = len(m.debugMessages)
		}

		for i := startIdx; i < endIdx; i++ {
			msg := m.debugMessages[i]
			// Color code based on message type
			if strings.Contains(msg, "[DEBUG]") {
				content.WriteString(debugStyle.Render(msg))
			} else if strings.Contains(msg, "[VERBOSE]") {
				content.WriteString(verboseStyle.Render(msg))
			} else if strings.Contains(msg, "[ERROR]") {
				content.WriteString(errorStyle.Render(msg))
			} else if strings.Contains(msg, "[WARN]") {
				content.WriteString(warningStyle.Render(msg))
			} else {
				content.WriteString(infoStyle.Render(msg))
			}
			content.WriteString("\n")
		}

		if len(m.debugMessages) > maxLines {
			scrollInfo := fmt.Sprintf("Showing %d-%d of %d messages", startIdx+1, endIdx, len(m.debugMessages))
			content.WriteString(helpStyle.Render(scrollInfo))
			content.WriteString("\n")
		}
	}

	// Action panel
	content.WriteString(actionStyle.Render("Actions:"))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("[Enter] Apply Suggested  [d] Delete  [m] Manual Import"))
	content.WriteString("\n")
	debugToggle := "[D] Toggle Debug"
	if m.config.Verbose || m.config.Debug {
		if m.showDebug {
			debugToggle = "[D] Hide Debug ✓"
		} else {
			debugToggle = "[D] Show Debug"
		}
	}
	content.WriteString(helpStyle.Render(fmt.Sprintf("[s] Skip/Monitor  [r] Refresh  %s  [q] Quit", debugToggle)))

	return content.String()
}

// loadItems loads queue items from all instances
func loadItems(config Config) tea.Cmd {
	return func() tea.Msg {
		items, err := fetchAllQueues(config)
		return itemsLoadedMsg{items: items, err: err}
	}
}

// filterItems filters items that need remediation (skipping /torrents/ and normal items)
func filterItems(items []QueueItem) []QueueItem {
	var filtered []QueueItem

	for _, item := range items {
		// Skip items in /torrents/ directory
		if strings.Contains(item.OutputPath, "/torrents/") {
			continue
		}

		// Only include items that need remediation
		action, _, _ := mapStatusToAction(item)
		if action != "monitor" || item.TrackedDownloadState == "importBlocked" {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// executeSuggestedAction executes the suggested action for an item
func executeSuggestedAction(config Config, item QueueItem) tea.Cmd {
	action, _, _ := mapStatusToAction(item)
	return executeAction(config, item, action)
}

// executeAction executes a specific action on an item
func executeAction(config Config, item QueueItem, action string) tea.Cmd {
	return func() tea.Msg {
		var err error
		var debugLog []string
		var verboseLog []string
		apiCallCount := 0

		// Create a custom logger that captures output
		logger := &tuiLogger{
			debugLog:   &debugLog,
			verboseLog: &verboseLog,
			config:     config,
		}

		token, tokenErr := getTokenForInstance(config, item.InstanceURL, item.InstanceType)
		if tokenErr != nil {
			return actionExecutedMsg{
				success:      false,
				err:          tokenErr,
				action:       action,
				debugLog:     debugLog,
				verboseLog:   verboseLog,
				apiCallCount: apiCallCount,
			}
		}

		switch action {
		case "delete":
			_, shouldBlocklist, _ := mapStatusToAction(item)
			logger.logVerbose(fmt.Sprintf("Deleting queue item #%d: %s (blocklist=%v)", item.ID, item.Title, shouldBlocklist))
			apiCallCount++
			err = deleteQueueItemWithLogging(config, logger, item.InstanceURL, token, item.ID, true, shouldBlocklist)

		case "manual_import":
			logger.logVerbose(fmt.Sprintf("Triggering manual import for queue item #%d: %s", item.ID, item.Title))
			logger.logVerbose(fmt.Sprintf("Output path: %s", item.OutputPath))
			logger.logVerbose(fmt.Sprintf("Instance type: %s", item.InstanceType))
			logger.logVerbose(fmt.Sprintf("Use REST API: %v", config.UseRestAPI))

			if item.SeriesID > 0 {
				logger.logVerbose(fmt.Sprintf("Series ID: %d", item.SeriesID))
			}
			if item.MovieID > 0 {
				logger.logVerbose(fmt.Sprintf("Movie ID: %d", item.MovieID))
			}

			count, importErr := triggerManualImportWithLogging(config, logger, item.InstanceURL, token, item.OutputPath, item.InstanceType, config.UseRestAPI, item)
			apiCallCount = count
			err = importErr

		case "monitor":
			// For monitor/skip, we just remove it from the list
			logger.logVerbose(fmt.Sprintf("Monitoring queue item #%d: %s (no action taken)", item.ID, item.Title))
			err = nil
		}

		return actionExecutedMsg{
			success:      err == nil,
			err:          err,
			action:       action,
			debugLog:     debugLog,
			verboseLog:   verboseLog,
			apiCallCount: apiCallCount,
		}
	}
}

// RunTUI starts the TUI application
func RunTUI(config Config) error {
	// Validate configuration
	if err := validateQueueConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Create and run the TUI
	model := InitialModel(config)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}

// tuiLogger captures log messages for display in TUI
type tuiLogger struct {
	debugLog   *[]string
	verboseLog *[]string
	config     Config
}

func (l *tuiLogger) logDebug(msg string) {
	if l.config.Debug {
		formatted := fmt.Sprintf("[DEBUG] %s", msg)
		*l.debugLog = append(*l.debugLog, formatted)
	}
}

func (l *tuiLogger) logVerbose(msg string) {
	if l.config.Verbose || l.config.Debug {
		formatted := fmt.Sprintf("[VERBOSE] %s", msg)
		*l.verboseLog = append(*l.verboseLog, formatted)
	}
}

func (l *tuiLogger) logInfo(msg string) {
	formatted := fmt.Sprintf("[INFO] %s", msg)
	*l.verboseLog = append(*l.verboseLog, formatted)
}

func (l *tuiLogger) logWarn(msg string) {
	formatted := fmt.Sprintf("[WARN] %s", msg)
	*l.verboseLog = append(*l.verboseLog, formatted)
}

func (l *tuiLogger) logError(msg string) {
	formatted := fmt.Sprintf("[ERROR] %s", msg)
	*l.verboseLog = append(*l.verboseLog, formatted)
}
