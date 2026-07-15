package core

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

type CommonConfig struct {
	Timeout      time.Duration
	NoColor      bool
	Theme        string
	JSONOutput   bool
	PlainOutput  bool
	Watch        bool
	WatchSeconds int
	Debug        bool
	Quiet        bool
	Strict       bool
	Logger       *slog.Logger
	Palette      *colors.Palette
}

// PartialError reports usable data accompanied by source failures.
type PartialError struct{ Warnings []string }

func (e *PartialError) Error() string       { return fmt.Sprintf("partial result: %v", e.Warnings) }
func (e *PartialError) PartialResult() bool { return true }

func FromToolkit(tk *config.ToolkitConfig) CommonConfig {
	c := CommonConfig{}
	if tk == nil {
		c.Timeout = 10 * time.Second
		return c
	}
	dur, err := time.ParseDuration(tk.General.Timeout)
	if err != nil || dur <= 0 {
		dur = 10 * time.Second
	}
	c.Timeout = dur
	c.NoColor = tk.General.NoColor
	c.Theme = tk.General.Theme
	return c
}

func pickPositive(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}
