package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/airtime"
	"github.com/calmcacil/CalmsToolkit/internal/cmdutil"
)

func main() {
	tk := cmdutil.LoadAndValidate()
	cfg := airtime.BuildToolConfig(tk)

	searchType := flag.String("type", "auto", "Search scope: auto, series, movie")
	limit := flag.Int("limit", cfg.Limit, "Maximum candidates to list")
	exact := flag.Bool("exact", false, "Require exact title match")
	season := flag.Int("season", 0, "Override season number (0 = auto-detect)")
	pastDays := flag.Int("past", cfg.PastDays, "Past days for last-aired lookup")
	futureDays := flag.Int("future", cfg.FutureDays, "Future days for next-upcoming lookup")
	noBanner := flag.Bool("no-banner", false, "Suppress decorative banner")
	fullSeason := flag.Bool("full-season", false, "Show all episodes of the current season")

	cu := cmdutil.RegisterCommonFlags(flag.CommandLine, tk, cmdutil.Options{
		IncludeDebug: true,
	})
	flag.Parse()
	cu.Apply()

	query := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if query == "" {
		fmt.Fprintf(os.Stderr, "Usage: media-airtime <query> [flags]\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg.SearchType = *searchType
	cfg.Limit = *limit
	cfg.Exact = *exact
	cfg.Season = *season
	cfg.PastDays = *pastDays
	cfg.FutureDays = *futureDays
	cfg.Timeout = cu.Timeout
	cfg.NoColor = cu.NoColor
	cfg.Theme = cu.Theme
	cfg.JSONOutput = cu.JSONFlag()
	cfg.Debug = cu.Debug
	cfg.NoBanner = *noBanner
	cfg.FullSeason = *fullSeason

	if searchType := *searchType; searchType != "auto" && searchType != "series" && searchType != "movie" {
		fmt.Fprintf(os.Stderr, "ERROR: -type must be auto, series, or movie\n")
		os.Exit(1)
	}
	if cfg.Limit < 1 || cfg.Limit > 50 {
		fmt.Fprintf(os.Stderr, "ERROR: -limit must be between 1 and 50\n")
		os.Exit(1)
	}
	if cfg.Timeout <= 0 {
		fmt.Fprintf(os.Stderr, "ERROR: -timeout must be positive\n")
		os.Exit(1)
	}

	airtime.Run(query, cfg)
}
