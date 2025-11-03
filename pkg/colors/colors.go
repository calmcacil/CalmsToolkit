package colors

import "os"

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
}

func New(noColor bool) Colors {
	if noColor || os.Getenv("NO_COLOR") != "" {
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
	}
}

func (c Colors) Colorize(text, color string) string {
	if c.Reset == "" || color == "" {
		return text
	}
	return color + text + c.Reset
}

func (c Colors) RedText(text string) string {
	return c.Colorize(text, c.Red)
}

func (c Colors) GreenText(text string) string {
	return c.Colorize(text, c.Green)
}

func (c Colors) YellowText(text string) string {
	return c.Colorize(text, c.Yellow)
}

func (c Colors) BlueText(text string) string {
	return c.Colorize(text, c.Blue)
}

func (c Colors) MagentaText(text string) string {
	return c.Colorize(text, c.Magenta)
}

func (c Colors) CyanText(text string) string {
	return c.Colorize(text, c.Cyan)
}

func (c Colors) GrayText(text string) string {
	return c.Colorize(text, c.Gray)
}

func (c Colors) BoldText(text string) string {
	return c.Colorize(text, c.Bold)
}
