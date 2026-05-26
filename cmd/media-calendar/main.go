package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/calendar"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func main() {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := calendar.BuildToolConfig(tk)

	days := flag.Int("days", cfg.Days, "Number of days to display")
	daysPast := flag.Int("days-past", cfg.DaysPast, "Number of past days to include")
	timeout := flag.Duration("timeout", cfg.Timeout, "HTTP connection timeout")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	watchMode := flag.Bool("watch", false, "Continuously monitor calendar")
	watchSeconds := flag.Int("interval", cfg.WatchSeconds, "Watch mode refresh interval in seconds")
	debug := flag.Bool("debug", cfg.Debug, "Enable debug logging")
	noBanner := flag.Bool("no-banner", false, "Suppress the banner header")
	quiet := flag.Bool("quiet", false, "Suppress queue warnings")
	filter := flag.String("filter", "", "Filter: missing,available,premieres,monitored (comma-separated)")
	monitoredOnly := flag.Bool("monitored-only", false, "Only show monitored items")
	flag.Parse()

	cfg.Days = *days
	cfg.DaysPast = *daysPast
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *jsonOutput
	cfg.JSONOutput = *jsonOutput
	cfg.WatchMode = *watchMode
	cfg.WatchSeconds = *watchSeconds
	cfg.Debug = *debug
	cfg.NoBanner = *noBanner
	cfg.Quiet = *quiet
	cfg.Filter = *filter
	cfg.MonitoredOnly = *monitoredOnly

	calendar.Run(cfg)
}
