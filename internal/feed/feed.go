package feed

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"slices"
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
		ShowGrabbed:  true,
		ShowImported: true,
		ShowFailed:   true,
		MaxEvents:    50,
	}
	if tk == nil {
		cfg.PollInterval = 5 * time.Second
		cfg.HistoryWindow = 1 * time.Hour
		cfg.Watch = false
		return cfg
	}
	cfg.SonarrInstances = slices.Clone(tk.Sonarr)
	cfg.RadarrInstances = slices.Clone(tk.Radarr)
	dur, _ := time.ParseDuration(tk.ArrFeed.PollInterval)
	if dur > 0 {
		cfg.PollInterval = dur
	} else {
		cfg.PollInterval = 5 * time.Second
	}
	dur, _ = time.ParseDuration(tk.ArrFeed.HistoryWindow)
	if dur > 0 {
		cfg.HistoryWindow = dur
	} else {
		cfg.HistoryWindow = 1 * time.Hour
	}
	cfg.ShowGrabbed = tk.ArrFeed.ShowGrabbed
	cfg.ShowImported = tk.ArrFeed.ShowImported
	cfg.ShowFailed = tk.ArrFeed.ShowFailed
	cfg.ShowDeleted = tk.ArrFeed.ShowDeleted
	cfg.ShowIgnored = tk.ArrFeed.ShowIgnored
	cfg.ShowSubtitles = tk.ArrFeed.ShowSubtitles
	if tk.ArrFeed.MaxEvents > 0 {
		cfg.MaxEvents = tk.ArrFeed.MaxEvents
	} else {
		cfg.MaxEvents = 50
	}
	return cfg
}

// Run executes the Arr event feed tool.
func Run(ctx context.Context, cfg ToolConfig) error {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		return fmt.Errorf("no Sonarr or Radarr instances configured")
	}

	client := httputil.NewClient(cfg.Timeout)
	p := colors.GetPalette(cfg.Theme)

	if cfg.Watch {
		return runWatchMode(ctx, cfg, client, p)
	} else {
		if err := runSingleMode(ctx, cfg, client, p); err != nil {
			return err
		}
	}
	return nil
}

func runSingleMode(ctx context.Context, cfg ToolConfig, client *httputil.Client, p *colors.Palette) error {
	since := time.Now().Add(-cfg.HistoryWindow)
	events, warnings, err := fetchAllHistoryDetailed(ctx, client, cfg, since)
	if err != nil {
		return err
	}

	events = filterEvents(events, cfg)

	if cfg.MaxEvents > 0 && len(events) > cfg.MaxEvents {
		events = events[:cfg.MaxEvents]
	}

	if cfg.JSONOutput {
		renderJSON(events, len(warnings) > 0, warnings)
	} else {
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
		}
		renderTable(events, cfg, p)
	}
	if len(warnings) > 0 && cfg.Strict {
		return &core.PartialError{Warnings: warnings}
	}
	return nil
}

func runWatchMode(ctx context.Context, cfg ToolConfig, client *httputil.Client, p *colors.Palette) error {
	if !cfg.JSONOutput && !cfg.PlainOutput {
		fmt.Print(colors.HideCursor)
		defer fmt.Print(colors.ShowCursor)
	}

	eventCache := make([]HistoryEvent, 0, 100)
	lastFetch := time.Now().Add(-cfg.HistoryWindow)

	if !cfg.JSONOutput && !cfg.PlainOutput {
		fmt.Print(colors.ClearScreen + colors.HomeCursor)
	}

	var lastHash string

	for {
		newEvents, warnings, err := fetchAllHistoryDetailed(ctx, client, cfg, lastFetch)
		if err != nil {
			if !cfg.JSONOutput && !cfg.PlainOutput {
				fmt.Print(colors.HomeCursor)
				clr := getColorFunc(cfg, p)
				fmt.Fprintf(os.Stderr, "%sERROR: %v%s\n", clr(p.Error), err, clr(p.Reset))
				fmt.Fprintf(os.Stderr, "Retrying in %v...\n", cfg.PollInterval)
				fmt.Print(colors.EraseDown)
			}
		} else {
			newCount := 0
			newlyAdded := make([]HistoryEvent, 0, len(newEvents))
			for _, event := range newEvents {
				dup := false
				for i := len(eventCache) - 1; i >= 0 && i >= len(eventCache)-50; i-- {
					if eventCache[i].ID == event.ID {
						dup = true
						break
					}
				}
				if !dup {
					eventCache = append(eventCache, event)
					newlyAdded = append(newlyAdded, event)
					newCount++
				}
			}

			if len(eventCache) > 100 {
				eventCache = eventCache[len(eventCache)-100:]
			}

			filteredEvents := filterEvents(eventCache, cfg)

			if cfg.MaxEvents > 0 && len(filteredEvents) > cfg.MaxEvents {
				filteredEvents = filteredEvents[:cfg.MaxEvents]
			}

			if cfg.JSONOutput {
				newlyAdded = filterEvents(newlyAdded, cfg)
				if len(newlyAdded) > 0 || len(warnings) > 0 {
					renderJSON(newlyAdded, len(warnings) > 0, warnings)
				}
			} else {
				newHash := computeFeedHash(filteredEvents)
				if newHash == lastHash && newCount == 0 {
					select {
					case <-ctx.Done():
						if !cfg.PlainOutput {
							fmt.Print(colors.ShowCursor)
						}
						return ctx.Err()
					case <-time.After(cfg.PollInterval):
					}
					continue
				}
				lastHash = newHash
				if !cfg.PlainOutput {
					fmt.Print(colors.HomeCursor)
				}
				renderTable(filteredEvents, cfg, p)
				if !cfg.PlainOutput {
					fmt.Print(colors.EraseDown)
				}
			}

			if newCount > 0 {
				lastFetch = time.Now()
			}
			if len(warnings) > 0 && cfg.Strict {
				return &core.PartialError{Warnings: warnings}
			}
		}

		select {
		case <-ctx.Done():
			if !cfg.JSONOutput && !cfg.PlainOutput {
				fmt.Print(colors.ShowCursor)
			}
			return ctx.Err()
		case <-time.After(cfg.PollInterval):
		}
	}
}

func computeFeedHash(events []HistoryEvent) string {
	data, _ := json.Marshal(events)
	h := sha256.Sum256(data)
	return string(h[:])
}
