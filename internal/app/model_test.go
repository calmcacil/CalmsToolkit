package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
)

func TestInitialModel(t *testing.T) {
	model := InitialModel()

	// Test that model is properly initialized
	if model.config == nil {
		t.Error("Expected config to be non-nil")
	}
	if model.tabs == nil {
		t.Error("Expected tabs to be non-nil")
	}
	if model.header == nil {
		t.Error("Expected header to be non-nil")
	}
	if model.statusBar == nil {
		t.Error("Expected statusBar to be non-nil")
	}
	if model.ready {
		t.Error("Expected ready to be false initially")
	}
	if model.quitting {
		t.Error("Expected quitting to be false initially")
	}
}

func TestTabString(t *testing.T) {
	tests := []struct {
		tab      Tab
		expected string
	}{
		{MediaRequestsTab, "Media Requests"},
		{StreamsTab, "Streams"},
		{CalendarTab, "Calendar"},
		{QueueTab, "Queue"},
		{ArrFeedTab, "ARR Feed"},
		{Tab(999), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.tab.String(); got != tt.expected {
			t.Errorf("Expected %q, got %q", tt.expected, got)
		}
	}
}

func TestModelUpdate(t *testing.T) {
	model := InitialModel()

	// Test quit key
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if appModel, ok := newModel.(Model); ok && !appModel.quitting {
		t.Error("Expected quitting to be true after Ctrl+C")
	}
	if cmd == nil {
		t.Error("Expected quit command")
	}

	// Test tab navigation
	model = InitialModel()
	newModel, cmd = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if appModel, ok := newModel.(Model); ok && appModel.tabs.GetActive() != 1 {
		t.Errorf("Expected active tab to be 1, got %d", appModel.tabs.GetActive())
	}
}

func TestModelView(t *testing.T) {
	model := InitialModel()
	view := model.View()

	// View should contain header
	if view == "" {
		t.Error("Expected view to be non-empty")
	}

	// Should contain tab names
	if !strings.Contains(view, "Media Requests") {
		t.Error("Expected view to contain 'Media Requests'")
	}
	if !strings.Contains(view, "Streams") {
		t.Error("Expected view to contain 'Streams'")
	}
}

func TestUpdateHeader(t *testing.T) {
	model := InitialModel()

	// Change active tab
	model.tabs.SetActive(2)
	model.updateHeader()

	// Header should be updated
	headerView := model.header.View()
	if !strings.Contains(headerView, "Calendar") {
		t.Error("Expected header to contain 'Calendar'")
	}
}
