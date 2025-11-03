package components

import (
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single tab
type Tab struct {
	ID   int
	Name string
	Icon string
}

// Tabs represents a tab navigation component
type Tabs struct {
	tabs        []Tab
	active      int
	width       int
	style       lipgloss.Style
	activeStyle lipgloss.Style
	colors      colors.Colors
}

// NewTabs creates a new tabs component
func NewTabs(colors colors.Colors) *Tabs {
	return &Tabs{
		tabs: []Tab{
			{ID: 0, Name: "Media Requests", Icon: "📺"},
			{ID: 1, Name: "Streams", Icon: "🎬"},
			{ID: 2, Name: "Calendar", Icon: "📅"},
			{ID: 3, Name: "Queue", Icon: "⚙️"},
			{ID: 4, Name: "ARR Feed", Icon: "📡"},
		},
		active: 0,
		colors: colors,
		style: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("240")),
		activeStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Background(lipgloss.Color("240")),
	}
}

// SetActive sets the active tab
func (t *Tabs) SetActive(active int) {
	if active >= 0 && active < len(t.tabs) {
		t.active = active
	}
}

// GetActive returns the active tab index
func (t *Tabs) GetActive() int {
	return t.active
}

// GetActiveTab returns the active tab
func (t *Tabs) GetActiveTab() Tab {
	if t.active >= 0 && t.active < len(t.tabs) {
		return t.tabs[t.active]
	}
	return Tab{}
}

// Next moves to the next tab
func (t *Tabs) Next() {
	t.active = (t.active + 1) % len(t.tabs)
}

// Previous moves to the previous tab
func (t *Tabs) Previous() {
	t.active = (t.active - 1 + len(t.tabs)) % len(t.tabs)
}

// SetWidth sets the width for the tabs
func (t *Tabs) SetWidth(width int) {
	t.width = width
}

// SetStyle sets the style for inactive tabs
func (t *Tabs) SetStyle(style lipgloss.Style) {
	t.style = style
}

// SetActiveStyle sets the style for the active tab
func (t *Tabs) SetActiveStyle(style lipgloss.Style) {
	t.activeStyle = style
}

// Update handles tab navigation updates
func (t *Tabs) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRight, tea.KeyTab:
			t.Next()
		case tea.KeyLeft:
			t.Previous()
			// TODO: Add direct tab selection with Ctrl+number when key constants are available
		}
	}
	return nil
}

// View renders the tabs
func (t *Tabs) View() string {
	var renderedTabs []string

	for i, tab := range t.tabs {
		tabText := tab.Icon + " " + tab.Name

		if i == t.active {
			if t.colors.Bold != "" {
				// Use ANSI colors if available
				tabText = t.colors.BoldText(tabText)
			} else {
				// Use lipgloss styling
				tabText = t.activeStyle.Render(tabText)
			}
		} else {
			if t.colors.Gray != "" {
				// Use ANSI colors if available
				tabText = t.colors.GrayText(tabText)
			} else {
				// Use lipgloss styling
				tabText = t.style.Render(tabText)
			}
		}

		renderedTabs = append(renderedTabs, tabText)
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...)
}
