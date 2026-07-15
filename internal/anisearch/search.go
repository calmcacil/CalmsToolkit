package anisearch

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/console"
	"github.com/calmcacil/CalmsToolkit/internal/core"
)

// BuildToolConfig constructs ToolConfig from the global toolkit config.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{
		CommonConfig: core.FromToolkit(tk),
		Limit:        5,
		MappingURL:   "https://github.com/anibridge/anibridge-mappings/releases/download/v3/mappings.json.zst",
		Page:         1,
	}
	if tk == nil {
		return cfg
	}
	if tk.AniSearch.Limit > 0 {
		cfg.Limit = tk.AniSearch.Limit
	}
	if tk.AniSearch.MappingURL != "" {
		cfg.MappingURL = tk.AniSearch.MappingURL
	}
	if tk.AniSearch.MappingPath != "" {
		cfg.MappingPath = tk.AniSearch.MappingPath
	}
	return cfg
}

// Run executes the anisearch tool with the given query and configuration.
func Run(ctx context.Context, query string, cfg ToolConfig) error {
	// Load the anibridge mapping for TVDB cross-references.
	var mapping *AnibridgeMapping
	if !cfg.NoTVDB {
		mapping = loadMapping(ctx, cfg.MappingPath, cfg.MappingURL, cfg.ForceRefresh)
		if mapping == nil && !cfg.JSONOutput {
			fmt.Fprintln(os.Stderr, "WARN: no TVDB mapping available (TVDB IDs will not be shown)")
		}
	}

	// Create the AniList client.
	client := NewAniListClient(cfg.Timeout)

	// Perform the initial search.
	result, err := client.SearchPage(ctx, query, 1, cfg.Limit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(result.Media) == 0 {
		if cfg.JSONOutput {
			return console.WriteEnvelope(os.Stdout, "anime", map[string]interface{}{
				"query":   query,
				"results": []Show{},
				"error":   "no results found",
			}, false, nil, time.Now())
		}
		return fmt.Errorf("no results found for %q", query)
	}

	// JSON mode: output and exit.
	if cfg.JSONOutput {
		type showOut struct {
			AniListID   int      `json:"anilist_id"`
			MALID       *int     `json:"mal_id"`
			Title       Title    `json:"title"`
			Format      string   `json:"format"`
			Episodes    *int     `json:"episodes"`
			Status      string   `json:"status"`
			Season      string   `json:"season"`
			SeasonYear  *int     `json:"season_year"`
			Genres      []string `json:"genres"`
			Tags        []Tag    `json:"tags"`
			AverageRank *int     `json:"average_score"`
			Popularity  *int     `json:"popularity"`
			Description string   `json:"description"`
			Studios     []string `json:"studios"`
			TVDBID      int      `json:"tvdb_id"`
		}

		results := make([]showOut, len(result.Media))
		for i, show := range result.Media {
			tvdbID := 0
			if mapping != nil {
				tvdbID, _ = mapping.LookupByAniList(show.ID)
			}
			results[i] = showOut{
				AniListID:   show.ID,
				MALID:       show.IDMal,
				Title:       show.Title,
				Format:      show.Format,
				Episodes:    show.Episodes,
				Status:      show.Status,
				Season:      show.Season,
				SeasonYear:  show.SeasonYear,
				Genres:      show.Genres,
				Tags:        show.Tags,
				AverageRank: show.AverageRank,
				Popularity:  show.Popularity,
				Description: show.Description,
				Studios:     show.StudioNames(),
				TVDBID:      tvdbID,
			}
		}

		return console.WriteEnvelope(os.Stdout, "anime", map[string]interface{}{
			"query":    query,
			"results":  results,
			"page":     result.PageInfo.CurrentPage,
			"has_next": result.PageInfo.HasNextPage,
			"total":    result.PageInfo.Total,
		}, false, nil, time.Now())
	}

	// Interactive mode.
	runInteractive(ctx, client, query, result, cfg, mapping)
	return nil
}

// runInteractive manages the interactive TUI loop.
func runInteractive(ctx context.Context, client *AniListClient, query string, currentResult *SearchResult, cfg ToolConfig, mapping *AnibridgeMapping) {
	// Clear screen once at the start.
	fmt.Fprint(os.Stdout, "\033[H\033[J")

	selected := 0
	currentPage := 1

	runInRawTerminal(func() {
		for {
			// Render search results.
			output := renderSearchResults(query, currentResult, selected, mapping, cfg)
			fmt.Fprint(os.Stdout, "\033[H\033[J"+output)

			// Read key.
			key := readKey()

			switch key {
			case KeyUp:
				if selected > 0 {
					selected--
				}
			case KeyDown:
				if selected < len(currentResult.Media)-1 {
					selected++
				}
			case KeyEnter:
				if selected >= 0 && selected < len(currentResult.Media) {
					runDetailView(currentResult.Media[selected], mapping, cfg)
				}
			case KeyNext:
				if currentResult.PageInfo.HasNextPage {
					next, err := client.SearchPage(ctx, query, currentPage+1, cfg.Limit)
					if err == nil && len(next.Media) > 0 {
						currentResult = next
						currentPage++
						selected = 0
					}
				}
			case KeyPrev:
				if currentPage > 1 {
					prev, err := client.SearchPage(ctx, query, currentPage-1, cfg.Limit)
					if err == nil && len(prev.Media) > 0 {
						currentResult = prev
						currentPage--
						selected = 0
					}
				}
			case KeyQuit, KeyCtrlC:
				return
			case KeyEsc:
				return
			}
		}
	})

	// Clean up: clear screen and move cursor to bottom.
	fmt.Fprint(os.Stdout, "\033[H\033[J")
	fmt.Fprint(os.Stdout, "\n")
}

// runDetailView shows the detail for a single show until the user presses B or Q.
func runDetailView(show Show, mapping *AnibridgeMapping, cfg ToolConfig) {
	tvdbID := 0
	if mapping != nil {
		tvdbID, _ = mapping.LookupByAniList(show.ID)
	}

	output := renderDetail(show, tvdbID, cfg)
	fmt.Fprint(os.Stdout, "\033[H\033[J"+output)

	for {
		key := readKey()

		switch key {
		case KeyBack, KeyEsc:
			return
		case KeyQuit, KeyCtrlC:
			fmt.Fprint(os.Stdout, "\033[H\033[J")
			return
		}
	}
}
