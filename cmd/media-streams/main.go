package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/streams"
)

func main() {
	tk, err := config.LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := streams.BuildToolConfig(tk)

	server := flag.String("server", cfg.ServerType, "Server type: plex, jellyfin, or both")
	plexURL := flag.String("plex-url", cfg.PlexURL, "Plex server URL")
	plexToken := flag.String("plex-token", cfg.PlexToken, "Plex authentication token")
	jellyfinURL := flag.String("jellyfin-url", cfg.JellyfinURL, "Jellyfin server URL")
	jellyfinToken := flag.String("jellyfin-token", cfg.JellyfinToken, "Jellyfin API token")
	timeout := flag.Duration("timeout", cfg.Timeout, "Connection timeout")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	watchMode := flag.Bool("watch", false, "Continuously monitor streams")
	watchSeconds := flag.Int("interval", cfg.WatchSeconds, "Watch mode refresh interval in seconds")
	historyDuration := flag.Duration("history-duration", cfg.HistoryDuration, "How long to keep session history in watch mode")
	quiet := flag.Bool("quiet", false, "Suppress non-error output")
	flag.Parse()

	cfg.ServerType = *server
	cfg.PlexURL = *plexURL
	cfg.PlexToken = *plexToken
	cfg.JellyfinURL = *jellyfinURL
	cfg.JellyfinToken = *jellyfinToken
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *jsonOutput
	cfg.JSONOutput = *jsonOutput
	cfg.WatchMode = *watchMode
	cfg.WatchSeconds = *watchSeconds
	cfg.HistoryDuration = *historyDuration
	cfg.Quiet = *quiet

	if tk != nil {
		if err := tk.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: config validation: %v\n", err)
		}
	}

	validServers := map[string]bool{"plex": true, "jellyfin": true, "both": true}
	if cfg.ServerType != "" && !validServers[cfg.ServerType] {
		fmt.Fprintf(os.Stderr, "ERROR: invalid -server value %q: must be 'plex', 'jellyfin', or 'both'\n", cfg.ServerType)
		os.Exit(1)
	}

	cfg.PlexURL = strings.TrimSuffix(cfg.PlexURL, "/")
	cfg.JellyfinURL = strings.TrimSuffix(cfg.JellyfinURL, "/")

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
