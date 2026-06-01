package core

import (
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
	Watch        bool
	WatchSeconds int
	Debug        bool
	Quiet        bool
	Logger       *slog.Logger
	Palette      *colors.Palette
}

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
