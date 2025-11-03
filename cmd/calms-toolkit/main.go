package main

import (
	"fmt"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Check for help flag
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println("CalmsToolkit TUI - Unified Terminal User Interface")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  calms-toolkit")
		fmt.Println()
		fmt.Println("Navigation:")
		fmt.Println("  Tab, →    : Next tab")
		fmt.Println("  Shift+Tab, ← : Previous tab")
		fmt.Println("  Ctrl+C     : Quit")
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  NO_COLOR=1 : Disable colors")
		fmt.Println("  DEBUG=1    : Enable debug mode")
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  The application will automatically load configuration from:")
		fmt.Println("  - /opt/apps/compose/.env (if exists)")
		fmt.Println("  - Environment variables")
		fmt.Println("  - Default values")
		os.Exit(0)
	}

	p := tea.NewProgram(app.InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running CalmsToolkit TUI: %v\n", err)
		os.Exit(1)
	}
}
