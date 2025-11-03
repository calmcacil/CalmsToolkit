package main

import (
	"log"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/tools/queue"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate queue configuration
	if err := cfg.ValidateQueueConfig(); err != nil {
		log.Fatalf("Invalid queue configuration: %v", err)
	}

	// Create initial model
	model := queue.NewModel(cfg)
	model.SetConfig(cfg)

	// Create and run program
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
		os.Exit(1)
	}
}
