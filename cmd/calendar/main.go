//go:build calendar
// +build calendar

package main

import (
	"fmt"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/tools/calendar"
	"github.com/charmbracelet/bubbletea"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Validate configuration
	if len(cfg.SonarrURLs) == 0 && len(cfg.RadarrURLs) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Please set SONARR_URLS/SONARR_TOKENS or RADARR_URLS/RADARR_TOKENS environment variables\n")
		os.Exit(1)
	}

	// Create and run calendar model
	model := calendar.NewModel(cfg)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running calendar: %v\n", err)
		os.Exit(1)
	}
}
