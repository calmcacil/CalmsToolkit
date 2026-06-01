package feed

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"
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
func Run(cfg ToolConfig) {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	client := httputil.NewClient(cfg.Timeout)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	p := colors.GetPalette(cfg.Theme)

	if cfg.Watch {
		runWatchMode(ctx, cfg, client, p)
	} else {
		runSingleMode(ctx, cfg, client, p)
	}
}

func runSingleMode(ctx context.Context, cfg ToolConfig, client *httputil.Client, p *colors.Palette) {
	since := time.Now().Add(-cfg.HistoryWindow)
	events, err := fetchAllHistory(ctx, client, cfg, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	events = filterEvents(events, cfg)

	if cfg.MaxEvents > 0 && len(events) > cfg.MaxEvents {
		events = events[:cfg.MaxEvents]
	}

	if cfg.JSONOutput {
		renderJSON(events)
	} else {
		renderTable(events, cfg, p)
	}
}

func runWatchMode(ctx context.Context, cfg ToolConfig, client *httputil.Client, p *colors.Palette) {
	if !cfg.JSONOutput {
		fmt.Print(colors.HideCursor)
		defer fmt.Print(colors.ShowCursor)
	}

	eventCache := make([]HistoryEvent, 0, 100)
	lastFetch := time.Now().Add(-cfg.HistoryWindow)

	fmt.Print(colors.ClearScreen + colors.HomeCursor)

	var lastHash string

	for {
		newEvents, err := fetchAllHistory(ctx, client, cfg, lastFetch)
		if err != nil {
			if !cfg.JSONOutput {
				fmt.Print(colors.HomeCursor)
				clr := getColorFunc(cfg, p)
				fmt.Fprintf(os.Stderr, "%sERROR: %v%s\n", clr(p.Error), err, clr(p.Reset))
				fmt.Fprintf(os.Stderr, "Retrying in %v...\n", cfg.PollInterval)
				fmt.Print(colors.EraseDown)
			}
		} else {
			newCount := 0
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
				renderJSON(filteredEvents)
			} else {
				newHash := computeFeedHash(filteredEvents)
				if newHash == lastHash && newCount == 0 {
					select {
					case <-ctx.Done():
						fmt.Print(colors.ShowCursor)
						return
					case <-time.After(cfg.PollInterval):
					}
					continue
				}
				lastHash = newHash
				fmt.Print(colors.HomeCursor)
				renderTable(filteredEvents, cfg, p)
				fmt.Print(colors.EraseDown)
			}

			if newCount > 0 {
				lastFetch = time.Now()
			}
		}

		select {
		case <-ctx.Done():
			if !cfg.JSONOutput {
				fmt.Print(colors.ShowCursor)
			}
			return
		case <-time.After(cfg.PollInterval):
		}
	}
}

func computeFeedHash(events []HistoryEvent) string {
	data, _ := json.Marshal(events)
	h := sha256.Sum256(data)
	return string(h[:])
}
