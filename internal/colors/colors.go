package colors

type Colorizer struct {
	NoColor bool
}

func New(noColor bool) *Colorizer {
	return &Colorizer{NoColor: noColor}
}

func (c *Colorizer) Apply(code string) string {
	if c.NoColor {
		return ""
	}
	return code
}

const (
	Reset   = "\033[0m"
	Red     = "\033[0;31m"
	Green   = "\033[0;32m"
	Yellow  = "\033[0;33m"
	Blue    = "\033[0;34m"
	Magenta = "\033[0;35m"
	Cyan    = "\033[0;36m"
	Gray    = "\033[0;90m"
	Bold    = "\033[1m"
	Orange  = "\033[38;5;208m"
)

const (
	ClearScreen = "\033[2J"
	HomeCursor  = "\033[H"
	HideCursor  = "\033[?25l"
	ShowCursor  = "\033[?25h"
)
