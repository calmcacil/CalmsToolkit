package colors

import (
	"os"
)

// Colors represents the color scheme for the TUI
type Colors struct {
	Reset   string
	Red     string
	Green   string
	Yellow  string
	Blue    string
	Magenta string
	Cyan    string
	Gray    string
	Bold    string
	Dim     string
}

// New creates a new color scheme, respecting NO_COLOR environment variable
func New() Colors {
	if os.Getenv("NO_COLOR") != "" {
		return Colors{}
	}

	return Colors{
		Reset:   "\033[0m",
		Red:     "\033[0;31m",
		Green:   "\033[0;32m",
		Yellow:  "\033[0;33m",
		Blue:    "\033[0;34m",
		Magenta: "\033[0;35m",
		Cyan:    "\033[0;36m",
		Gray:    "\033[0;90m",
		Bold:    "\033[1m",
		Dim:     "\033[2m",
	}
}
