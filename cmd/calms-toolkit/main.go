package main

import (
	"github.com/calmcacil/CalmsToolkit/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(app.InitialModel())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
