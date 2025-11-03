package colors

import (
	"os"
)

// Colors represents the ANSI color scheme for the application
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
	Orange  string
}

// New creates a new color scheme, respecting NO_COLOR environment variable
func New() Colors {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
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
		Orange:  "\033[0;33m", // Using yellow as orange approximation
	}
}

// NewWithOverride creates a color scheme with explicit noColor override
func NewWithOverride(noColor bool) Colors {
	if noColor {
		return Colors{}
	}
	return New()
}

// Colorize applies color to text if colors are enabled
func (c Colors) Colorize(color, text string) string {
	if color == "" || text == "" {
		return text
	}
	return color + text + c.Reset
}

// Red colors text red
func (c Colors) RedText(text string) string {
	return c.Colorize(c.Red, text)
}

// Green colors text green
func (c Colors) GreenText(text string) string {
	return c.Colorize(c.Green, text)
}

// Yellow colors text yellow
func (c Colors) YellowText(text string) string {
	return c.Colorize(c.Yellow, text)
}

// Blue colors text blue
func (c Colors) BlueText(text string) string {
	return c.Colorize(c.Blue, text)
}

// Magenta colors text magenta
func (c Colors) MagentaText(text string) string {
	return c.Colorize(c.Magenta, text)
}

// Cyan colors text cyan
func (c Colors) CyanText(text string) string {
	return c.Colorize(c.Cyan, text)
}

// Gray colors text gray
func (c Colors) GrayText(text string) string {
	return c.Colorize(c.Gray, text)
}

// Bold makes text bold
func (c Colors) BoldText(text string) string {
	return c.Colorize(c.Bold, text)
}

// Orange colors text orange
func (c Colors) OrangeText(text string) string {
	return c.Colorize(c.Orange, text)
}
