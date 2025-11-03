//go:build streams
// +build streams

package main

import (
	"log"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/tools/streams"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create and run the TUI
	model := streams.NewModel(cfg)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
		os.Exit(1)
	}
}
