package queue

import (
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// NewModel creates a new queue remediation model
func NewModel(cfg *config.Config) Model {
	return Model{
		items:          []QueueItem{},
		selected:       0,
		view:           ViewList,
		loading:        true,
		error:          "",
		pendingAction:  "",
		pendingItems:   []int{},
		confirmMessage: "",
		filterStatus:   "",
		showOnlyIssues: true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchQueues(),
	)
}

// Update handles model updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyRunes:
			if len(msg.Runes) > 0 {
				switch msg.Runes[0] {
				case 'r':
					if !m.loading {
						m.loading = true
						m.error = ""
						return m, fetchQueues()
					}
				case 'i':
					if !m.loading && m.view == ViewList {
						m.showOnlyIssues = !m.showOnlyIssues
					}
				}
			}

		case tea.KeyF1:
			if !m.loading && m.view != ViewList {
				m.view = ViewList
				m.pendingAction = ""
				m.pendingItems = []int{}
				m.confirmMessage = ""
			}

		case tea.KeyEnter:
			if !m.loading {
				switch m.view {
				case ViewList:
					if len(m.items) > 0 {
						m.view = ViewDetail
					}
				case ViewDetail:
					// Action selection handled in detail view
				case ViewConfirm:
					// Execute pending action
					return m, executePendingAction(m)
				}
			}

		case tea.KeyEsc:
			if m.view == ViewConfirm {
				m.view = ViewDetail
				m.pendingAction = ""
				m.pendingItems = []int{}
				m.confirmMessage = ""
			}

		}

	case queuesFetchedMsg:
		m.loading = false
		if msg.err != nil {
			m.error = msg.err.Error()
		} else {
			m.items = msg.items
			m.selected = 0
		}

	case actionExecutedMsg:
		m.loading = false
		if msg.err != nil {
			m.error = msg.err.Error()
		} else {
			// Refresh queue after action
			m.loading = true
			m.view = ViewList
			m.pendingAction = ""
			m.pendingItems = []int{}
			m.confirmMessage = ""
			return m, fetchQueues()
		}
	}

	// Handle table navigation
	if m.view == ViewList && !m.loading {
		return m.updateTable(msg)
	}

	return m, nil
}

// updateTable handles table-specific updates
func (m Model) updateTable(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	// Create table model for navigation
	tableModel := m.createTable()
	tableModel, cmd = tableModel.Update(msg)

	// Extract selected row from table
	if len(m.getFilteredItems()) > 0 {
		m.selected = tableModel.Cursor()
	}

	return m, cmd
}

// createTable creates a table model from current items
func (m Model) createTable() table.Model {
	columns := []table.Column{
		{Title: "Instance", Width: 12},
		{Title: "Title", Width: 40},
		{Title: "Status", Width: 12},
		{Title: "Reason", Width: 20},
		{Title: "Action", Width: 10},
	}

	var rows []table.Row
	filteredItems := m.getFilteredItems()

	for i, item := range filteredItems {
		action := m.getRecommendedAction(item)
		reason := m.getReason(item)

		row := table.Row{
			item.InstanceName,
			truncateString(item.Title, 37),
			item.Status,
			truncateString(reason, 17),
			string(action.Action),
		}
		rows = append(rows, row)

		if i == m.selected {
			tableModel := table.New(
				table.WithColumns(columns),
				table.WithRows(rows),
				table.WithFocused(true),
			)
			tableModel.SetCursor(m.selected)
			return tableModel
		}
	}

	return table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)
}

// getFilteredItems returns items based on current filter
func (m Model) getFilteredItems() []QueueItem {
	var filtered []QueueItem

	for _, item := range m.items {
		if m.showOnlyIssues {
			action := m.getRecommendedAction(item)
			if action.Action == ActionMonitor {
				continue
			}
		}

		if m.filterStatus != "" && item.Status != m.filterStatus {
			continue
		}

		filtered = append(filtered, item)
	}

	return filtered
}

// getRecommendedAction returns the recommended action for a queue item
func (m Model) getRecommendedAction(item QueueItem) QueueItemAction {
	if item.Status == "failed" {
		return QueueItemAction{
			Action:    ActionDelete,
			Reason:    ReasonFailed,
			Blocklist: true,
		}
	}

	reason, shouldBlocklist := parseStatusMessages(item.StatusMessages)

	switch reason {
	case "custom_format_no_upgrade":
		return QueueItemAction{
			Action:    ActionDelete,
			Reason:    ReasonCustomFormat,
			Blocklist: shouldBlocklist,
		}
	case "quality_no_upgrade":
		return QueueItemAction{
			Action:    ActionDelete,
			Reason:    ReasonQuality,
			Blocklist: shouldBlocklist,
		}
	case "no_files_found":
		return QueueItemAction{
			Action:    ActionDelete,
			Reason:    ReasonNoFiles,
			Blocklist: shouldBlocklist,
		}
	case "sample_file":
		return QueueItemAction{
			Action:    ActionDelete,
			Reason:    ReasonSample,
			Blocklist: shouldBlocklist,
		}
	case "matched_by_id":
		return QueueItemAction{
			Action:       ActionManualImport,
			Reason:       ReasonIDMatch,
			ManualImport: true,
		}
	}

	if item.TrackedDownloadState == "importBlocked" {
		return QueueItemAction{
			Action:       ActionManualImport,
			Reason:       ReasonImportBlocked,
			ManualImport: true,
		}
	}

	return QueueItemAction{
		Action: ActionMonitor,
		Reason: ReasonDownloading,
	}
}

// getReason returns a human-readable reason for the item's status
func (m Model) getReason(item QueueItem) string {
	if item.Status == "failed" {
		return "download failed"
	}

	reason, _ := parseStatusMessages(item.StatusMessages)

	switch reason {
	case "custom_format_no_upgrade":
		return "custom format not an upgrade"
	case "quality_no_upgrade":
		return "quality not an upgrade"
	case "no_files_found":
		return "no files found"
	case "sample_file":
		return "sample file detected"
	case "matched_by_id":
		return "matched to series by ID"
	case "unknown":
		if item.TrackedDownloadState == "importBlocked" {
			return "import blocked"
		}
		return "downloading normally"
	default:
		if item.TrackedDownloadState == "importBlocked" {
			return "import blocked"
		}
		return "downloading normally"
	}
}

// parseStatusMessages parses status messages to determine the issue
func parseStatusMessages(statusMessages []StatusMessage) (string, bool) {
	var hasQualityCF, hasSample, hasNoFiles, hasIDMatch bool
	var isCustomFormat bool

	for _, sm := range statusMessages {
		for _, msg := range sm.Messages {
			msgLower := strings.ToLower(msg)

			if strings.Contains(msgLower, "custom format upgrade") {
				hasQualityCF = true
				isCustomFormat = true
			}

			if strings.Contains(msgLower, "quality revision") {
				hasQualityCF = true
			}

			if msgLower == "sample" || strings.Contains(msgLower, "sample") {
				hasSample = true
			}

			if strings.Contains(msgLower, "no files found") {
				hasNoFiles = true
			}

			if strings.Contains(msgLower, "matched to series by id") {
				hasIDMatch = true
			}
		}
	}

	if hasQualityCF {
		if isCustomFormat {
			return "custom_format_no_upgrade", true
		}
		return "quality_no_upgrade", true
	}

	if hasIDMatch {
		return "matched_by_id", false
	}

	if hasSample {
		return "sample_file", false
	}

	if hasNoFiles {
		return "no_files_found", false
	}

	return "unknown", false
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
