package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/airtime"
	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func main() {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := airtime.BuildToolConfig(tk)

	searchType := flag.String("type", "auto", "Search scope: auto, series, movie")
	limit := flag.Int("limit", cfg.Limit, "Maximum candidates to list")
	exact := flag.Bool("exact", false, "Require exact title match")
	season := flag.Int("season", 0, "Override season number (0 = auto-detect)")
	pastDays := flag.Int("past", cfg.PastDays, "Past days for last-aired lookup")
	futureDays := flag.Int("future", cfg.FutureDays, "Future days for next-upcoming lookup")
	timeout := flag.Duration("timeout", cfg.Timeout, "HTTP connection timeout")
	theme := flag.String("theme", cfg.Theme, "Color theme (default, catppuccin-mocha, catppuccin-latte)")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	debug := flag.Bool("debug", cfg.Debug, "Enable debug logging")
	noBanner := flag.Bool("no-banner", false, "Suppress decorative banner")
	fullSeason := flag.Bool("full-season", false, "Show all episodes of the current season")
	flag.Parse()

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
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *jsonOutput
	cfg.Theme = *theme
	if cfg.Theme != "" && !colors.ValidateTheme(cfg.Theme) {
		fmt.Fprintf(os.Stderr, "Warning: unknown theme %q, falling back to default (valid: %s)\n",
			cfg.Theme, strings.Join(colors.ValidThemes(), ", "))
		cfg.Theme = "default"
	}
	cfg.JSONOutput = *jsonOutput
	cfg.Debug = *debug
	cfg.NoBanner = *noBanner
	cfg.FullSeason = *fullSeason

	if tk != nil {
		if err := tk.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: config validation: %v\n", err)
		}
	}

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
