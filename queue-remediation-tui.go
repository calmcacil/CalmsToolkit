//go:build queueremediation

package main

import (
	"errors"
	"fmt"
	"sort"
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
	verboseDebug  bool
	debugScroll   int
	episodeCache  map[int]EpisodeResource
	seriesCache   map[int]SeriesResource
	mappingInfo   map[int]mappingInfo
}

type mappingInfo struct {
	Episode    EpisodeResource
	Series     SeriesResource
	TitleMatch TitleMatchResult
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
		verboseDebug:  false,
		debugScroll:   0,
		episodeCache:  make(map[int]EpisodeResource),
		seriesCache:   make(map[int]SeriesResource),
		mappingInfo:   make(map[int]mappingInfo),
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
				case 'v':
					if m.config.Debug || m.config.Verbose {
						m.verboseDebug = !m.verboseDebug
						if m.verboseDebug {
							m.status = "Verbose debug enabled"
						} else {
							m.status = "Verbose debug disabled"
						}
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
			filteredItems, report := filterItemsWithReport(m.config, msg.items)
			m.items = filteredItems
			if len(m.items) == 0 {
				m.status = "No queue items requiring remediation found"
			} else {
				m.status = fmt.Sprintf("Loaded %d items requiring remediation", len(m.items))
			}

			// Pre-fetch mapping details for metadata mapping pending items (Sonarr)
			for _, item := range m.items {
				if getReason(item) == "metadata mapping pending" && item.InstanceType == "sonarr" && item.EpisodeID > 0 {
					m.ensureMappingInfo(item)
				}
			}

			if m.config.Debug {
				summaryLogs, verboseLogs := buildSummaryBlock(report)
				m.debugMessages = append(m.debugMessages, summaryLogs...)

				if m.verboseDebug {
					m.debugMessages = append(m.debugMessages, verboseLogs...)
				}

				if len(m.debugMessages) > 500 {
					m.debugMessages = m.debugMessages[len(m.debugMessages)-500:]
				}
			}
		}

	case actionExecutedMsg:
		m.loading = false

		if len(msg.debugLog) > 0 || len(msg.verboseLog) > 0 {
			if m.verboseDebug {
				m.debugMessages = append(m.debugMessages, msg.debugLog...)
				m.debugMessages = append(m.debugMessages, msg.verboseLog...)
			} else {
				m.debugMessages = append(m.debugMessages, msg.debugLog...)
			}

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

		reason := getReason(item)

		// Show proposed mapping for Sonarr items when mapping details help manual import
		if (reason == "metadata mapping pending" || reason == "matched to series by ID") && item.InstanceType == "sonarr" && item.EpisodeID > 0 {
			m.ensureMappingInfo(item)
			label := "Proposed Mapping:"
			if reason == "matched to series by ID" {
				label = "Current Mapping:"
			}
			content.WriteString(infoStyle.Render(label))
			content.WriteString("\n")
			if info, ok := m.mappingInfo[item.EpisodeID]; ok {
				if info.Series.Title != "" {
					content.WriteString(fmt.Sprintf("  • Series: %s\n", info.Series.Title))
				}
				content.WriteString(fmt.Sprintf("  • Episode: S%02dE%02d - %s\n", info.Episode.SeasonNumber, info.Episode.EpisodeNumber, info.Episode.Title))
				if info.Episode.AbsoluteEpisodeNumber > 0 {
					content.WriteString(fmt.Sprintf("  • Absolute #: %d\n", info.Episode.AbsoluteEpisodeNumber))
				}
				if info.Episode.AirDateUtc != "" {
					content.WriteString(fmt.Sprintf("  • Air date (UTC): %s\n", info.Episode.AirDateUtc))
				}
				content.WriteString(fmt.Sprintf("  • Title match: %.1f%% (%s)\n", info.TitleMatch.Similarity, info.TitleMatch.Reason))
			} else {
				content.WriteString("  • Mapping details unavailable\n")
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

	verboseToggle := ""
	if m.config.Verbose || m.config.Debug {
		if m.verboseDebug {
			verboseToggle = "[v] Verbose Debug ✓"
		} else {
			verboseToggle = "[v] Verbose Debug"
		}
	}

	toggles := []string{debugToggle}
	if verboseToggle != "" {
		toggles = append(toggles, verboseToggle)
	}
	content.WriteString(helpStyle.Render(fmt.Sprintf("[s] Skip/Monitor  [r] Refresh  %s  [q] Quit", strings.Join(toggles, "  "))))

	return content.String()
}

// loadItems loads queue items from all instances
func loadItems(config Config) tea.Cmd {
	return func() tea.Msg {
		items, err := fetchAllQueues(config)
		return itemsLoadedMsg{items: items, err: err}
	}
}

// InstanceFilterReport captures per-instance filtering data
type InstanceFilterReport struct {
	InstanceName    string
	TotalItems      int
	FilteredItems   int
	SkippedTorrents int
	SkippedMonitor  int
	ItemsByReason   map[string][]QueueItem
}

// FilterReport aggregates filtering results across instances
type FilterReport struct {
	TotalItems    int
	FilteredItems int
	Instances     []InstanceFilterReport
}

// filterItemsWithReport filters items that need remediation and builds a report
func filterItemsWithReport(config Config, items []QueueItem) ([]QueueItem, FilterReport) {
	var filtered []QueueItem
	report := FilterReport{TotalItems: len(items)}
	instanceReports := make(map[string]*InstanceFilterReport)

	for _, item := range items {
		instanceName := getInstanceName(config, item.InstanceURL, item.InstanceType)
		if _, ok := instanceReports[instanceName]; !ok {
			instanceReports[instanceName] = &InstanceFilterReport{
				InstanceName:  instanceName,
				ItemsByReason: make(map[string][]QueueItem),
			}
		}

		instReport := instanceReports[instanceName]
		instReport.TotalItems++

		if strings.Contains(item.OutputPath, "/torrents/") {
			instReport.SkippedTorrents++
			continue
		}

		include := needsRemediation(item)
		if include {
			filtered = append(filtered, item)
			instReport.FilteredItems++
			reason := getReason(item)
			instReport.ItemsByReason[reason] = append(instReport.ItemsByReason[reason], item)
		} else {
			instReport.SkippedMonitor++
		}
	}

	for _, inst := range instanceReports {
		report.Instances = append(report.Instances, *inst)
	}

	sort.Slice(report.Instances, func(i, j int) bool {
		return report.Instances[i].InstanceName < report.Instances[j].InstanceName
	})

	report.FilteredItems = len(filtered)
	return filtered, report
}

func buildSummaryBlock(report FilterReport) ([]string, []string) {
	var summary []string
	var verbose []string

	summary = append(summary, "[DEBUG] === Filter Summary ===")
	summary = append(summary, fmt.Sprintf("[DEBUG] Filtered %d/%d items requiring remediation", report.FilteredItems, report.TotalItems))

	for _, inst := range report.Instances {
		summary = append(summary, fmt.Sprintf("[DEBUG] -- %s --", inst.InstanceName))
		summary = append(summary, fmt.Sprintf("[DEBUG] %s: %d/%d items require remediation", inst.InstanceName, inst.FilteredItems, inst.TotalItems))

		if inst.SkippedTorrents > 0 {
			summary = append(summary, fmt.Sprintf("[DEBUG] %s: skipped %d torrent-path items", inst.InstanceName, inst.SkippedTorrents))
		}
		if inst.SkippedMonitor > 0 {
			summary = append(summary, fmt.Sprintf("[DEBUG] %s: %d items monitored/ignored", inst.InstanceName, inst.SkippedMonitor))
		}

		var reasons []string
		for reason := range inst.ItemsByReason {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)

		for _, reason := range reasons {
			items := inst.ItemsByReason[reason]
			summary = append(summary, fmt.Sprintf("[DEBUG]   %s: %d", reason, len(items)))

			if len(items) == 0 {
				continue
			}

			sampleCount := len(items)
			if sampleCount > 5 {
				sampleCount = 5
			}

			var titles []string
			for i := 0; i < sampleCount; i++ {
				titles = append(titles, truncateTitle(items[i].Title))
			}
			verbose = append(verbose, fmt.Sprintf("[VERBOSE] %s %s: %s", inst.InstanceName, reason, strings.Join(titles, " | ")))
			if len(items) > sampleCount {
				verbose = append(verbose, fmt.Sprintf("[VERBOSE] %s %s: %d more suppressed", inst.InstanceName, reason, len(items)-sampleCount))
			}
		}
	}

	if len(report.Instances) > 0 {
		summary = append(summary, "[DEBUG] ------------------------")
	}

	return summary, verbose
}

func (m *TUIModel) ensureMappingInfo(item QueueItem) {
	if item.InstanceType != "sonarr" || item.EpisodeID == 0 {
		return
	}

	if m.mappingInfo == nil {
		m.mappingInfo = make(map[int]mappingInfo)
	}
	if m.episodeCache == nil {
		m.episodeCache = make(map[int]EpisodeResource)
	}
	if m.seriesCache == nil {
		m.seriesCache = make(map[int]SeriesResource)
	}

	if _, ok := m.mappingInfo[item.EpisodeID]; ok {
		return
	}

	token, err := getTokenForInstance(m.config, item.InstanceURL, item.InstanceType)
	if err != nil {
		return
	}

	episode, err := fetchEpisodeDetails(m.config, item.InstanceURL, token, item.EpisodeID)
	if err != nil || episode == nil {
		return
	}

	m.episodeCache[item.EpisodeID] = *episode

	var series SeriesResource
	if episode.SeriesID != 0 {
		if cached, ok := m.seriesCache[episode.SeriesID]; ok {
			series = cached
		} else {
			seriesResp, serr := fetchSeriesDetails(m.config, item.InstanceURL, token, episode.SeriesID)
			if serr == nil && seriesResp != nil {
				series = *seriesResp
				m.seriesCache[episode.SeriesID] = series
			}
		}
	}

	titleMatch := validateTitleMatch(item.Title, episode.Title)
	m.mappingInfo[item.EpisodeID] = mappingInfo{
		Episode:    *episode,
		Series:     series,
		TitleMatch: titleMatch,
	}
}

func truncateTitle(title string) string {
	const maxLength = 40
	runes := []rune(title)
	if len(runes) <= maxLength {
		return title
	}

	return string(runes[:maxLength-1]) + "…"
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
			reason := getReason(item)
			useRest := config.UseRestAPI
			// XEM/TBA mapping pending requires manual UI-equivalent command flow (no REST manual import)
			if reason == "metadata mapping pending" {
				useRest = false
			}

			logger.logVerbose(fmt.Sprintf("Triggering manual import for queue item #%d: %s", item.ID, item.Title))
			logger.logVerbose(fmt.Sprintf("Output path: %s", item.OutputPath))
			logger.logVerbose(fmt.Sprintf("Instance type: %s", item.InstanceType))
			logger.logVerbose(fmt.Sprintf("Use REST API: %v", useRest))
			logger.logVerbose(fmt.Sprintf("Reason: %s", reason))

			if item.SeriesID > 0 {
				logger.logVerbose(fmt.Sprintf("Series ID: %d", item.SeriesID))
			}
			if item.MovieID > 0 {
				logger.logVerbose(fmt.Sprintf("Movie ID: %d", item.MovieID))
			}

			count, importErr := triggerManualImportWithLogging(config, logger, item.InstanceURL, token, item.OutputPath, item.InstanceType, useRest, item)
			apiCallCount = count
			if errors.Is(importErr, errManualImportNoMapping) {
				logger.logWarn(fmt.Sprintf("Mapping failed; deleting queue item %d", item.ID))
				apiCallCount++
				err = deleteQueueItemWithLogging(config, logger, item.InstanceURL, token, item.ID, true, false)
				action = "delete"
				break
			}
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
