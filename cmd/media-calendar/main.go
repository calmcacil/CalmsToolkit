package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/calendar"
	"github.com/calmcacil/CalmsToolkit/internal/cmdutil"
)

func main() {
	tk := cmdutil.LoadAndValidate()
	cfg := calendar.BuildToolConfig(tk)

	days := flag.Int("days", cfg.Days, "Number of days to display")
	daysPast := flag.Int("days-past", cfg.DaysPast, "Number of past days to include")
	noBanner := flag.Bool("no-banner", false, "Suppress the banner header")
	filter := flag.String("filter", "", "Filter: missing,available,premieres,monitored (comma-separated)")
	monitoredOnly := flag.Bool("monitored-only", false, "Only show monitored items")

	cu := cmdutil.RegisterCommonFlags(flag.CommandLine, tk, cmdutil.Options{
		IncludeWatch: true,
		IncludeDebug: true,
		IncludeQuiet: true,
	})
	flag.Parse()
	cu.Apply()

	cfg.Days = *days
	cfg.DaysPast = *daysPast
	cfg.Timeout = cu.Timeout
	cfg.NoColor = cu.NoColor
	cfg.Theme = cu.Theme
	cfg.JSONOutput = cu.JSONFlag()
	cfg.WatchMode = cu.Watch
	cfg.WatchSeconds = cu.WatchSeconds
	cfg.Debug = cu.Debug
	cfg.NoBanner = *noBanner
	cfg.Quiet = cu.Quiet
	cfg.Filter = *filter
	cfg.MonitoredOnly = *monitoredOnly

	if cfg.Days < 0 {
		fmt.Fprintf(os.Stderr, "ERROR: -days must be >= 0\n")
		os.Exit(1)
	}
	if cfg.DaysPast < 0 {
		fmt.Fprintf(os.Stderr, "ERROR: -days-past must be >= 0\n")
		os.Exit(1)
	}
	if cfg.WatchSeconds < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: -interval must be >= 1\n")
		os.Exit(1)
	}
	if cfg.Filter != "" {
		validFilters := map[string]bool{"missing": true, "available": true, "premieres": true, "monitored": true}
		for _, f := range strings.Split(cfg.Filter, ",") {
			f = strings.TrimSpace(f)
			if f != "" && !validFilters[f] {
				fmt.Fprintf(os.Stderr, "Warning: unknown filter value %q (valid: missing, available, premieres, monitored)\n", f)
			}
		}
	}

	calendar.Run(cfg)
}
