package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/cmdutil"
	"github.com/calmcacil/CalmsToolkit/internal/feed"
)

func main() {
	tk := cmdutil.LoadAndValidate()
	cfg := feed.BuildToolConfig(tk)

	poll := flag.Duration("poll", cfg.PollInterval, "Poll interval for watch mode")
	duration := flag.Duration("duration", cfg.HistoryWindow, "History lookback window")
	showGrabbed := flag.Bool("show-grabbed", cfg.ShowGrabbed, "Show grabbed events")
	showImported := flag.Bool("show-imported", cfg.ShowImported, "Show imported events")
	showFailed := flag.Bool("show-failed", cfg.ShowFailed, "Show failed events")
	showDeleted := flag.Bool("show-deleted", cfg.ShowDeleted, "Show deleted events")
	showIgnored := flag.Bool("show-ignored", cfg.ShowIgnored, "Show ignored events")
	showSubtitles := flag.Bool("show-subtitles", cfg.ShowSubtitles, "Show subtitle info for imported events")
	maxEvents := flag.Int("events", cfg.MaxEvents, "Maximum number of events to display (1-100)")
	watch := flag.Bool("watch", false, "Continuous monitoring mode")

	cu := cmdutil.RegisterCommonFlags(flag.CommandLine, tk, cmdutil.Options{
		IncludeQuiet: true,
	})
	flag.Parse()
	cu.Apply()

	cfg.PollInterval = *poll
	cfg.HistoryWindow = *duration
	cfg.Timeout = cu.Timeout
	cfg.NoColor = cu.NoColor
	cfg.Theme = cu.Theme
	cfg.JSONOutput = cu.JSONFlag()
	cfg.Watch = *watch
	cfg.ShowGrabbed = *showGrabbed
	cfg.ShowImported = *showImported
	cfg.ShowFailed = *showFailed
	cfg.ShowDeleted = *showDeleted
	cfg.ShowIgnored = *showIgnored
	cfg.ShowSubtitles = *showSubtitles
	cfg.MaxEvents = *maxEvents
	cfg.Quiet = cu.Quiet

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
