package queue

import (
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	cfg := &config.Config{}
	model := NewModel(cfg)

	if model.view != ViewList {
		t.Errorf("Expected initial view to be ViewList, got %v", model.view)
	}

	if !model.loading {
		t.Error("Expected initial loading state to be true")
	}

	if !model.showOnlyIssues {
		t.Error("Expected showOnlyIssues to be true by default")
	}
}

func TestModelUpdate(t *testing.T) {
	cfg := &config.Config{}
	model := NewModel(cfg)

	// Test Ctrl+C quit
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Expected quit command on Ctrl+C")
	}

	// Test refresh key
	model.loading = false
	updatedModel, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	queueModel := updatedModel.(Model)
	if !queueModel.loading {
		t.Error("Expected loading to be true after refresh")
	}

	// Test toggle issues filter
	model.loading = false
	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	queueModel = updatedModel.(Model)
	if queueModel.showOnlyIssues == model.showOnlyIssues {
		t.Error("Expected showOnlyIssues to toggle")
	}
}

func TestGetRecommendedAction(t *testing.T) {
	cfg := &config.Config{}
	model := NewModel(cfg)

	tests := []struct {
		name     string
		item     QueueItem
		expected QueueAction
	}{
		{
			name: "failed item should be deleted",
			item: QueueItem{
				Status: "failed",
			},
			expected: ActionDelete,
		},
		{
			name: "import blocked should be manual import",
			item: QueueItem{
				Status:               "completed",
				TrackedDownloadState: "importBlocked",
			},
			expected: ActionManualImport,
		},
		{
			name: "normal download should be monitored",
			item: QueueItem{
				Status:               "downloading",
				TrackedDownloadState: "downloading",
			},
			expected: ActionMonitor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := model.getRecommendedAction(tt.item)
			if action.Action != tt.expected {
				t.Errorf("Expected action %s, got %s", tt.expected, action.Action)
			}
		})
	}
}

func TestParseStatusMessages(t *testing.T) {
	tests := []struct {
		name           string
		statusMessages []StatusMessage
		expectedReason string
		expectedBlock  bool
	}{
		{
			name: "custom format upgrade",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Not a Custom Format upgrade"},
				},
			},
			expectedReason: "custom_format_no_upgrade",
			expectedBlock:  true,
		},
		{
			name: "sample file",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"sample"},
				},
			},
			expectedReason: "sample_file",
			expectedBlock:  false,
		},
		{
			name: "no files found",
			statusMessages: []StatusMessage{
				{
					Title:    "Error",
					Messages: []string{"No files found are eligible for import"},
				},
			},
			expectedReason: "no_files_found",
			expectedBlock:  false,
		},
		{
			name: "unknown",
			statusMessages: []StatusMessage{
				{
					Title:    "Info",
					Messages: []string{"Some unknown message"},
				},
			},
			expectedReason: "unknown",
			expectedBlock:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, blocklist := parseStatusMessages(tt.statusMessages)
			if reason != tt.expectedReason {
				t.Errorf("Expected reason %s, got %s", tt.expectedReason, reason)
			}
			if blocklist != tt.expectedBlock {
				t.Errorf("Expected blocklist %v, got %v", tt.expectedBlock, blocklist)
			}
		})
	}
}

func TestGetFilteredItems(t *testing.T) {
	cfg := &config.Config{}
	model := NewModel(cfg)

	model.items = []QueueItem{
		{ID: 1, Status: "downloading", TrackedDownloadState: "downloading"},
		{ID: 2, Status: "failed", TrackedDownloadState: "failed"},
		{ID: 3, Status: "warning", TrackedDownloadState: "importBlocked", StatusMessages: []StatusMessage{{Messages: []string{"No files found"}}}},
	}

	// Test show only issues (default)
	filtered := model.getFilteredItems()
	if len(filtered) != 2 {
		t.Errorf("Expected 2 items when showing issues only, got %d", len(filtered))
	}

	// Test show all
	model.showOnlyIssues = false
	filtered = model.getFilteredItems()
	if len(filtered) != 3 {
		t.Errorf("Expected 3 items when showing all, got %d", len(filtered))
	}

	// Test status filter
	model.filterStatus = "failed"
	filtered = model.getFilteredItems()
	if len(filtered) != 1 {
		t.Errorf("Expected 1 item when filtering by failed status, got %d", len(filtered))
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{
			input:    "short",
			maxLen:   10,
			expected: "short",
		},
		{
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			input:    "exactlength",
			maxLen:   11,
			expected: "exactlength",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
