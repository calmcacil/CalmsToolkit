package streams

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/core"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{
		CommonConfig: core.FromToolkit(tk),
		ServerType:   "both",
	}
	if tk == nil {
		cfg.HistoryDuration = 15 * time.Minute
		return cfg
	}
	cfg.PlexURL = strings.TrimSuffix(tk.MediaStreams.PlexURL, "/")
	cfg.PlexToken = tk.MediaStreams.PlexToken
	cfg.JellyfinURL = strings.TrimSuffix(tk.MediaStreams.JellyfinURL, "/")
	cfg.JellyfinToken = tk.MediaStreams.JellyfinToken
	cfg.ServerType = tk.MediaStreams.ServerType
	if cfg.ServerType == "" {
		cfg.ServerType = "both"
	}
	cfg.WatchSeconds = tk.MediaStreams.WatchInterval
	if cfg.WatchSeconds <= 0 {
		cfg.WatchSeconds = 10
	}
	dur, _ := time.ParseDuration(tk.MediaStreams.HistoryDuration)
	if dur > 0 {
		cfg.HistoryDuration = dur
	} else {
		cfg.HistoryDuration = 15 * time.Minute
	}
	return cfg
}

// Run executes the media streams monitor tool.
func Run(ctx context.Context, cfg ToolConfig) error {
	p := colors.GetPalette(cfg.Theme)

	if cfg.Watch {
		if !cfg.JSONOutput && !cfg.PlainOutput {
			fmt.Print(colors.HideCursor)
			defer fmt.Print(colors.ShowCursor)
		}

		history := &SessionHistory{
			Records: make(map[string]*SessionRecord),
		}

		if !cfg.JSONOutput && !cfg.PlainOutput {
			fmt.Print(colors.ClearScreen + colors.HomeCursor)
		}

		var lastHash string

		for {
			if err := displayAllSessionsWithHistory(ctx, cfg, history, &lastHash, p); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				if cfg.Strict {
					return err
				}
			}
			select {
			case <-ctx.Done():
				fmt.Fprintln(os.Stderr, "\nShutting down.")
				return ctx.Err()
			case <-time.After(time.Duration(cfg.WatchSeconds) * time.Second):
			}
		}
	}

	if err := displayAllSessions(ctx, cfg, p); err != nil {
		return err
	}
	return nil
}

func displayAllSessionsWithHistory(ctx context.Context, cfg ToolConfig, history *SessionHistory, lastHash *string, p *colors.Palette) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int
	var plexErr, jellyfinErr error

	client := httputil.NewClient(cfg.Timeout)

	if cfg.ServerType == ServerPlex || cfg.ServerType == ServerBoth {
		streams, err := fetchPlexStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		} else {
			plexErr = err
		}
	}

	if cfg.ServerType == ServerJellyfin || cfg.ServerType == ServerBoth {
		streams, err := fetchJellyfinStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		} else {
			jellyfinErr = err
		}
	}

	updateHistory(history, allStreams, cfg.HistoryDuration)

	if allFailed(cfg.ServerType, plexErr, jellyfinErr) {
		return fmt.Errorf("all servers failed: plex: %v, jellyfin: %v", maybeError(plexErr), maybeError(jellyfinErr))
	}
	partial, warnings := partialFailures(plexErr, jellyfinErr)

	newHash := computeStreamsHash(history) + strings.Join(warnings, "|")
	if *lastHash == newHash {
		return nil
	}
	*lastHash = newHash
	if cfg.JSONOutput {
		if err := displayJSONOutput(allStreams, plexCount, jellyfinCount, partial, warnings); err != nil {
			return err
		}
		if partial && cfg.Strict {
			return &core.PartialError{Warnings: warnings}
		}
		return nil
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
	}
	if partial && cfg.Strict {
		return &core.PartialError{Warnings: warnings}
	}

	return displayTerminalOutputWithHistory(allStreams, history, plexCount, jellyfinCount, cfg.NoColor, cfg.PlainOutput, p)
}

func displayAllSessions(ctx context.Context, cfg ToolConfig, p *colors.Palette) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int
	var plexErr, jellyfinErr error

	client := httputil.NewClient(cfg.Timeout)

	if cfg.ServerType == ServerPlex || cfg.ServerType == ServerBoth {
		streams, err := fetchPlexStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		} else {
			plexErr = err
		}
	}

	if cfg.ServerType == ServerJellyfin || cfg.ServerType == ServerBoth {
		streams, err := fetchJellyfinStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		} else {
			jellyfinErr = err
		}
	}

	if allFailed(cfg.ServerType, plexErr, jellyfinErr) {
		return fmt.Errorf("all servers failed: plex: %v, jellyfin: %v", maybeError(plexErr), maybeError(jellyfinErr))
	}
	partial, warnings := partialFailures(plexErr, jellyfinErr)

	if cfg.JSONOutput {
		if err := displayJSONOutput(allStreams, plexCount, jellyfinCount, partial, warnings); err != nil {
			return err
		}
		if partial && cfg.Strict {
			return &core.PartialError{Warnings: warnings}
		}
		return nil
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
	}
	if partial && cfg.Strict {
		return &core.PartialError{Warnings: warnings}
	}

	return displayTerminalOutput(allStreams, plexCount, jellyfinCount, cfg.NoColor, p)
}

func partialFailures(plexErr, jellyfinErr error) (bool, []string) {
	warnings := make([]string, 0, 2)
	if plexErr != nil {
		warnings = append(warnings, "Plex: "+plexErr.Error())
	}
	if jellyfinErr != nil {
		warnings = append(warnings, "Jellyfin: "+jellyfinErr.Error())
	}
	return len(warnings) > 0, warnings
}

func allFailed(serverType string, plexErr, jellyfinErr error) bool {
	switch serverType {
	case ServerBoth:
		return plexErr != nil && jellyfinErr != nil
	case ServerPlex:
		return plexErr != nil
	case ServerJellyfin:
		return jellyfinErr != nil
	default:
		return true
	}
}

func maybeError(err error) string {
	if err != nil {
		return err.Error()
	}
	return "not configured"
}
