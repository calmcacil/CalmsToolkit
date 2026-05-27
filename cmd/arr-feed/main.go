package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/feed"
)

func main() {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := feed.BuildToolConfig(tk)

	poll := flag.Duration("poll", cfg.PollInterval, "Poll interval for watch mode")
	duration := flag.Duration("duration", cfg.HistoryWindow, "History lookback window")
	timeout := flag.Duration("timeout", cfg.Timeout, "HTTP request timeout")
	theme := flag.String("theme", cfg.Theme, "Color theme (default, catppuccin-mocha, catppuccin-latte)")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	json := flag.Bool("json", false, "Output JSON instead of table")
	watch := flag.Bool("watch", false, "Continuous monitoring mode")
	showGrabbed := flag.Bool("show-grabbed", cfg.ShowGrabbed, "Show grabbed events")
	showImported := flag.Bool("show-imported", cfg.ShowImported, "Show imported events")
	showFailed := flag.Bool("show-failed", cfg.ShowFailed, "Show failed events")
	showDeleted := flag.Bool("show-deleted", cfg.ShowDeleted, "Show deleted events")
	showIgnored := flag.Bool("show-ignored", cfg.ShowIgnored, "Show ignored events")
	showSubtitles := flag.Bool("show-subtitles", cfg.ShowSubtitles, "Show subtitle info for imported events")
	maxEvents := flag.Int("events", cfg.MaxEvents, "Maximum number of events to display (1-100)")
	quiet := flag.Bool("quiet", false, "Suppress error output in watch mode")
	flag.Parse()

	cfg.PollInterval = *poll
	cfg.HistoryWindow = *duration
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *json
	cfg.Theme = *theme
	cfg.JSON = *json
	cfg.Watch = *watch
	cfg.ShowGrabbed = *showGrabbed
	cfg.ShowImported = *showImported
	cfg.ShowFailed = *showFailed
	cfg.ShowDeleted = *showDeleted
	cfg.ShowIgnored = *showIgnored
	cfg.ShowSubtitles = *showSubtitles
	cfg.MaxEvents = *maxEvents
	cfg.Quiet = *quiet

	if tk != nil {
		if err := tk.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: config validation: %v\n", err)
		}
	}
	if cfg.MaxEvents < 0 || cfg.MaxEvents > 100 {
		fmt.Fprintf(os.Stderr, "ERROR: -events must be between 0 and 100\n")
		os.Exit(1)
	}
	if cfg.PollInterval <= 0 {
		fmt.Fprintf(os.Stderr, "ERROR: -poll must be positive\n")
		os.Exit(1)
	}

	feed.Run(cfg)
}
