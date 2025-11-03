package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit
		case tea.KeyTab:
			m.currentTab = Tab((int(m.currentTab) + 1) % len(m.tabs))
			return m, nil
		case tea.KeyShiftTab:
			m.currentTab = Tab((int(m.currentTab) - 1 + len(m.tabs)) % len(m.tabs))
			return m, nil
		case tea.KeyRight:
			m.currentTab = Tab((int(m.currentTab) + 1) % len(m.tabs))
			return m, nil
		case tea.KeyLeft:
			m.currentTab = Tab((int(m.currentTab) - 1 + len(m.tabs)) % len(m.tabs))
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.dimensions = msg
		m.ready = true
		return m, nil

	case configLoadedMsg:
		if msg.err != nil {
			m.error = msg.err.Error()
			m.loading = false
		} else {
			m.config = msg.config
			m.loading = false
		}
		return m, nil
	}

	return m, nil
}
