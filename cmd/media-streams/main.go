package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/cmdutil"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/streams"
)

func main() {
	tk := cmdutil.LoadAndValidate()
	cfg := streams.BuildToolConfig(tk)

	server := flag.String("server", cfg.ServerType, "Server type: plex, jellyfin, or both")
	plexURL := flag.String("plex-url", cfg.PlexURL, "Plex server URL")
	plexToken := flag.String("plex-token", config.TokenFromEnv("PLEX_TOKEN", cfg.PlexToken), "Plex authentication token")
	jellyfinURL := flag.String("jellyfin-url", cfg.JellyfinURL, "Jellyfin server URL")
	jellyfinToken := flag.String("jellyfin-token", config.TokenFromEnv("JELLYFIN_TOKEN", cfg.JellyfinToken), "Jellyfin API token")
	historyDuration := flag.Duration("history-duration", cfg.HistoryDuration, "How long to keep session history in watch mode")

	cu := cmdutil.RegisterCommonFlags(flag.CommandLine, tk, cmdutil.Options{
		IncludeWatch: true,
		IncludeQuiet: true,
	})
	flag.Parse()
	cu.Apply()

	cfg.ServerType = *server
	cfg.PlexURL = strings.TrimSuffix(*plexURL, "/")
	cfg.PlexToken = *plexToken
	cfg.JellyfinURL = strings.TrimSuffix(*jellyfinURL, "/")
	cfg.JellyfinToken = *jellyfinToken
	cfg.Timeout = cu.Timeout
	cfg.NoColor = cu.NoColor
	cfg.Theme = cu.Theme
	cfg.JSONOutput = cu.JSONFlag()
	cfg.Watch = cu.Watch
	cfg.WatchSeconds = cu.WatchSeconds
	cfg.HistoryDuration = *historyDuration
	cfg.Quiet = cu.Quiet

	validServers := map[string]bool{"plex": true, "jellyfin": true, "both": true}
	if cfg.ServerType != "" && !validServers[cfg.ServerType] {
		fmt.Fprintf(os.Stderr, "ERROR: invalid -server value %q: must be 'plex', 'jellyfin', or 'both'\n", cfg.ServerType)
		os.Exit(1)
	}

	if cfg.WatchSeconds < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: -interval must be >= 1\n")
		os.Exit(1)
	}

	if cfg.ServerType == streams.ServerPlex || cfg.ServerType == streams.ServerBoth {
		if cfg.PlexToken == "" {
			fmt.Fprintf(os.Stderr, "ERROR: PLEX_TOKEN is not set\n")
			os.Exit(1)
		}
	}
	if cfg.ServerType == streams.ServerJellyfin || cfg.ServerType == streams.ServerBoth {
		if cfg.JellyfinToken == "" {
			fmt.Fprintf(os.Stderr, "ERROR: JELLYFIN_TOKEN is not set\n")
			os.Exit(1)
		}
	}

	streams.Run(cfg)
}
