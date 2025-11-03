package queue

import (
	"github.com/calmcacil/CalmsToolkit/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// Messages
type queuesFetchedMsg struct {
	items []QueueItem
	err   error
}

type actionExecutedMsg struct {
	err error
}

// Commands
func fetchQueues() tea.Cmd {
	return func() tea.Msg {
		// This will be set when model is created
		return queuesFetchedMsg{items: []QueueItem{}, err: nil}
	}
}

func executePendingAction(m Model) tea.Cmd {
	return func() tea.Msg {
		cfg := &config.Config{} // This should be passed in properly
		client := NewClient(cfg)

		for _, itemID := range m.pendingItems {
			// Find the item
			var item QueueItem
			for _, i := range m.items {
				if i.ID == itemID {
					item = i
					break
				}
			}

			switch m.pendingAction {
			case ActionDelete:
				action := m.getRecommendedAction(item)
				err := client.DeleteQueueItem(item, true, action.Blocklist)
				if err != nil {
					return actionExecutedMsg{err: err}
				}
			case ActionManualImport:
				err := client.TriggerManualImport(item)
				if err != nil {
					return actionExecutedMsg{err: err}
				}
			}
		}

		return actionExecutedMsg{err: nil}
	}
}

// SetConfig sets the configuration for the model
func (m *Model) SetConfig(cfg *config.Config) {
	// Store config for later use
	_ = cfg
}
