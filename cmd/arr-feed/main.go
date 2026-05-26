package main

import (
	"flag"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/feed"
)

func main() {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		println("Warning:", err.Error())
	}
	cfg := feed.BuildToolConfig(tk)

	poll := flag.Duration("poll", cfg.PollInterval, "Poll interval for watch mode")
	duration := flag.Duration("duration", cfg.HistoryWindow, "History lookback window")
	timeout := flag.Duration("timeout", cfg.Timeout, "HTTP request timeout")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	json := flag.Bool("json", false, "Output JSON instead of table")
	watch := flag.Bool("watch", false, "Continuous monitoring mode")
	showGrabbed := flag.Bool("show-grabbed", cfg.ShowGrabbed, "Show grabbed events")
	showImported := flag.Bool("show-imported", cfg.ShowImported, "Show imported events")
	showFailed := flag.Bool("show-failed", cfg.ShowFailed, "Show failed events")
	showDeleted := flag.Bool("show-deleted", cfg.ShowDeleted, "Show deleted events")
	showIgnored := flag.Bool("show-ignored", cfg.ShowIgnored, "Show ignored events")
	maxEvents := flag.Int("events", cfg.MaxEvents, "Maximum number of events to display (1-100)")
	quiet := flag.Bool("quiet", false, "Suppress error output in watch mode")
	flag.Parse()

	cfg.PollInterval = *poll
	cfg.HistoryWindow = *duration
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *json
	cfg.JSON = *json
	cfg.Watch = *watch
	cfg.ShowGrabbed = *showGrabbed
	cfg.ShowImported = *showImported
	cfg.ShowFailed = *showFailed
	cfg.ShowDeleted = *showDeleted
	cfg.ShowIgnored = *showIgnored
	cfg.MaxEvents = *maxEvents
	cfg.Quiet = *quiet

	feed.Run(cfg)
}
