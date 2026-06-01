package airtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

// SonarrSeries represents a single series from Sonarr /api/v3/series.
type SonarrSeries struct {
	ID           int            `json:"id"`
	Title        string         `json:"title"`
	Year         int            `json:"year"`
	TVDBID       int            `json:"tvdbId"`
	Monitored    bool           `json:"monitored"`
	Status       string         `json:"status"`
	Seasons      []SonarrSeason `json:"seasons"`
	SeasonCount  int            `json:"seasonCount"`
	Genres       []string       `json:"genres"`
	Runtime      int            `json:"runtime"`
	TitleSlug    string         `json:"titleSlug"`
	Images       []SonarrImage  `json:"images"`
	RemotePoster string         `json:"remotePoster,omitempty"`
}

type SonarrSeason struct {
	SeasonNumber int          `json:"seasonNumber"`
	Monitored    bool         `json:"monitored"`
	Statistics   *SonarrStats `json:"statistics,omitempty"`
}

type SonarrStats struct {
	EpisodeCount      int `json:"episodeCount"`
	EpisodeFileCount  int `json:"episodeFileCount"`
	TotalEpisodeCount int `json:"totalEpisodeCount"`
}

type SonarrImage struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
}

// RadarrMovie represents a movie from Radarr /api/v3/movie.
type RadarrMovie struct {
	ID              int      `json:"id"`
	Title           string   `json:"title"`
	Year            int      `json:"year"`
	TMDBID          int      `json:"tmdbId"`
	Monitored       bool     `json:"monitored"`
	HasFile         bool     `json:"hasFile"`
	IsAvailable     bool     `json:"isAvailable"`
	Status          string   `json:"status"`
	DigitalRelease  string   `json:"digitalRelease"`
	PhysicalRelease string   `json:"physicalRelease"`
	InCinemas       string   `json:"inCinemas"`
	Runtime         int      `json:"runtime"`
	Genres          []string `json:"genres"`
	TitleSlug       string   `json:"titleSlug"`
}

// SonarrEpisode represents a single episode with airtime.
type SonarrEpisode struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	AirDateUtc    time.Time `json:"airDateUtc"`
	SeasonNumber  int       `json:"seasonNumber"`
	EpisodeNumber int       `json:"episodeNumber"`
	Monitored     bool      `json:"monitored"`
	HasFile       bool      `json:"hasFile"`
}

// SeriesOrMovie is a unified candidate for searching.
type SeriesOrMovie struct {
	Type            string // "series" or "movie"
	Title           string
	Year            int
	TVDBID          int
	TMDBID          int
	Monitored       bool
	HasFile         bool
	Status          string
	Seasons         int
	Source          string // instance name
	Instance        config.ArrInstance
	RawID           int // series or movie ID for follow-up queries
	DigitalRelease  string
	PhysicalRelease string
	InCinemas       string
}

// EpisodeInfo holds a single episode for full-season display.
type EpisodeInfo struct {
	EpisodeNumber int
	Title         string
	AirDateUtc    time.Time
	HasFile       bool
	Aired         bool
}

// AirtimeInfo holds the resolved airtime for a matched item.
type AirtimeInfo struct {
	Title  string
	Year   int
	Source string
	Type   string // "series" or "movie"

	Status         string // "ongoing", "ended", "released", "announced", "tba"
	SeasonCount    int
	Season         int // current season number
	EpisodesOnDisk int
	EpisodesTotal  int
	Monitored      bool

	LastAir   *time.Time
	LastLabel string
	NextAir   *time.Time
	NextLabel string
	TVDBID    int
	TMDBID    int

	SeasonEpisodes []EpisodeInfo // populated when FullSeason is true
}

// SelectionEvent is emitted when the user picks a candidate.
type SelectionEvent struct {
	Index int
	Query string
}

// ToolConfig holds configuration for the media-airtime tool.
type ToolConfig struct {
	SonarrInstances []config.ArrInstance
	RadarrInstances []config.ArrInstance
	SearchType      string // "auto", "series", "movie"
	Limit           int
	Exact           bool
	Season          int
	PastDays        int
	FutureDays      int
	Timeout         time.Duration
	NoColor         bool
	Theme           string
	JSONOutput      bool
	Debug           bool
	NoBanner        bool
	FullSeason      bool
}

// BuildToolConfig constructs ToolConfig from the global config.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.Limit = 10
		cfg.PastDays = 7
		cfg.FutureDays = 30
		cfg.SearchType = "auto"
		return cfg
	}
	dur, err := time.ParseDuration(tk.General.Timeout)
	if err != nil || dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor
	cfg.Theme = tk.General.Theme
	cfg.SonarrInstances = slices.Clone(tk.Sonarr)
	cfg.RadarrInstances = slices.Clone(tk.Radarr)
	cfg.SearchType = "auto"

	if tk.MediaAirtime.Limit > 0 {
		cfg.Limit = tk.MediaAirtime.Limit
	} else {
		cfg.Limit = 10
	}
	if tk.MediaAirtime.PastDays > 0 {
		cfg.PastDays = tk.MediaAirtime.PastDays
	} else {
		cfg.PastDays = 7
	}
	if tk.MediaAirtime.FutureDays > 0 {
		cfg.FutureDays = tk.MediaAirtime.FutureDays
	} else {
		cfg.FutureDays = 30
	}
	cfg.Debug = tk.MediaAirtime.Debug
	return cfg
}

// Run executes the media-airtime tool.
func Run(query string, cfg ToolConfig) {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client := httputil.NewTransportClient(cfg.Timeout)

	candidates := searchLibrary(ctx, client, query, cfg)
	if len(candidates) == 0 {
		if cfg.JSONOutput {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			encoder.Encode(map[string]interface{}{
				"query":      query,
				"candidates": []map[string]interface{}{},
				"selected":   nil,
				"airtime":    nil,
				"error":      "no matches found",
			})
			return
		}
		fmt.Fprintf(os.Stderr, "No matches found for %q\n", query)
		os.Exit(2)
	}

	if cfg.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: %d candidate(s) found\n", len(candidates))
		for _, c := range candidates {
			fmt.Fprintf(os.Stderr, "DEBUG:   [%d] %s (%d) type=%s source=%s score=%d\n",
				c.Candidate.RawID, c.Candidate.Title, c.Candidate.Year,
				c.Candidate.Type, c.Candidate.Source, c.Score)
		}
	}

	selected := pickCandidate(ctx, candidates, query, cfg)
	if selected == nil {
		os.Exit(3)
	}

	info := resolveAirtime(ctx, client, *selected, cfg)
	if cfg.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		candOut := make([]map[string]interface{}, len(candidates))
		for i, sm := range candidates {
			candOut[i] = candidateToMap(sm.Candidate, sm.Score)
		}
		encoder.Encode(map[string]interface{}{
			"query":      query,
			"candidates": candOut,
			"selected":   candidateToMap(selected.Candidate, selected.Score),
			"airtime":    infoToMap(info),
		})
		return
	}

	renderCard(info, cfg)
}

func searchLibrary(ctx context.Context, client *httputil.Client, query string, cfg ToolConfig) []scoredMatch {
	var mu sync.Mutex
	all := make([]scoredMatch, 0, 100)
	var g sync.WaitGroup

	includeSeries := cfg.SearchType == "auto" || cfg.SearchType == "series"
	includeMovies := cfg.SearchType == "auto" || cfg.SearchType == "movie"

	if includeSeries {
		for _, inst := range cfg.SonarrInstances {
			inst := inst
			g.Add(1)
			go func() {
				defer g.Done()
				series, err := fetchSonarrLibrary(ctx, client, inst, cfg.Debug)
				if err != nil {
					if cfg.Debug {
						fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: %v\n", inst.Name, err)
					}
					return
				}
				mu.Lock()
				for _, s := range series {
					c := SeriesOrMovie{
						Type:      "series",
						Title:     s.Title,
						Year:      s.Year,
						TVDBID:    s.TVDBID,
						Monitored: s.Monitored,
						Status:    s.Status,
						Seasons:   s.SeasonCount,
						Source:    inst.Name,
						Instance:  inst,
						RawID:     s.ID,
					}
					score := scoreCandidate(query, c)
					if score > 0 {
						all = append(all, scoredMatch{Candidate: c, Score: score})
					}
				}
				mu.Unlock()
			}()
		}
	}

	if includeMovies {
		for _, inst := range cfg.RadarrInstances {
			inst := inst
			g.Add(1)
			go func() {
				defer g.Done()
				movies, err := fetchRadarrLibrary(ctx, client, inst, cfg.Debug)
				if err != nil {
					if cfg.Debug {
						fmt.Fprintf(os.Stderr, "DEBUG: Radarr %s: %v\n", inst.Name, err)
					}
					return
				}
				mu.Lock()
				for _, m := range movies {
					c := SeriesOrMovie{
						Type:            "movie",
						Title:           m.Title,
						Year:            m.Year,
						TMDBID:          m.TMDBID,
						Monitored:       m.Monitored,
						HasFile:         m.HasFile,
						Status:          m.Status,
						Source:          inst.Name,
						Instance:        inst,
						RawID:           m.ID,
						DigitalRelease:  m.DigitalRelease,
						PhysicalRelease: m.PhysicalRelease,
						InCinemas:       m.InCinemas,
					}
					score := scoreCandidate(query, c)
					if score > 0 {
						all = append(all, scoredMatch{Candidate: c, Score: score})
					}
				}
				mu.Unlock()
			}()
		}
	}

	g.Wait()

	if cfg.Exact && len(all) > 0 {
		q := strings.ToLower(query)
		var filtered []scoredMatch
		for _, sm := range all {
			if strings.EqualFold(strings.ToLower(sm.Candidate.Title), q) {
				filtered = append(filtered, sm)
			}
		}
		all = filtered
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		if all[i].Candidate.Year != all[j].Candidate.Year {
			return all[i].Candidate.Year > all[j].Candidate.Year
		}
		return all[i].Candidate.Title < all[j].Candidate.Title
	})

	if len(all) > cfg.Limit {
		all = all[:cfg.Limit]
	}

	if cfg.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: search returned %d candidate(s)\n", len(all))
		for i, sm := range all {
			fmt.Fprintf(os.Stderr, "DEBUG:   %d. %s (%d) score=%d type=%s src=%s\n",
				i+1, sm.Candidate.Title, sm.Candidate.Year, sm.Score,
				sm.Candidate.Type, sm.Candidate.Source)
		}
	}

	return all
}

func fetchSonarrLibrary(ctx context.Context, client *httputil.Client, inst config.ArrInstance, debug bool) ([]SonarrSeries, error) {
	url := inst.URL + "/api/v3/series"
	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: GET %s\n", inst.Name, url)
	}
	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var series []SonarrSeries
	if err := client.DoJSON(ctx, "GET", url, headers, nil, &series); err != nil {
		return nil, err
	}
	return series, nil
}

func fetchRadarrLibrary(ctx context.Context, client *httputil.Client, inst config.ArrInstance, debug bool) ([]RadarrMovie, error) {
	url := inst.URL + "/api/v3/movie"
	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Radarr %s: GET %s\n", inst.Name, url)
	}
	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var movies []RadarrMovie
	if err := client.DoJSON(ctx, "GET", url, headers, nil, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func fetchSonarrEpisodes(ctx context.Context, client *httputil.Client, inst config.ArrInstance, seriesID int, debug bool) ([]SonarrEpisode, error) {
	url := fmt.Sprintf("%s/api/v3/episode?seriesId=%d", inst.URL, seriesID)
	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: GET %s\n", inst.Name, url)
	}
	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var episodes []SonarrEpisode
	if err := client.DoJSON(ctx, "GET", url, headers, nil, &episodes); err != nil {
		return nil, err
	}
	for i := range episodes {
		episodes[i].AirDateUtc = episodes[i].AirDateUtc.Local()
	}
	return episodes, nil
}

func resolveAirtime(ctx context.Context, client *httputil.Client, selected scoredMatch, cfg ToolConfig) AirtimeInfo {
	c := selected.Candidate
	info := AirtimeInfo{
		Title:     c.Title,
		Year:      c.Year,
		Source:    c.Source,
		Type:      c.Type,
		Monitored: c.Monitored,
		TVDBID:    c.TVDBID,
		TMDBID:    c.TMDBID,
	}

	if c.Type == "series" {
		episodes, err := fetchSonarrEpisodes(ctx, client, c.Instance, c.RawID, cfg.Debug)
		if err != nil {
			if cfg.Debug {
				fmt.Fprintf(os.Stderr, "DEBUG: error fetching episodes for %q: %v\n", c.Title, err)
			}
			info.Status = "unknown"
			return info
		}

		if len(episodes) == 0 {
			info.Status = "tba"
			info.SeasonCount = c.Seasons
			return info
		}

		seasonEpisodes := groupEpisodesBySeason(episodes)

		if cfg.Season > 0 {
			info.Season = cfg.Season
		} else {
			info.Season = findCurrentSeason(seasonEpisodes)
		}

		if season, ok := seasonEpisodes[info.Season]; ok {
			info.SeasonCount = len(seasonEpisodes)
			info.EpisodesOnDisk = countOnDisk(season)
			info.EpisodesTotal = len(season)

			sort.Slice(season, func(i, j int) bool {
				return season[i].AirDateUtc.Before(season[j].AirDateUtc)
			})

			var (
				lastAir, nextAir     *time.Time
				lastLabel, nextLabel string
				now                  = time.Now()
				epInfos              []EpisodeInfo
			)

			for _, ep := range season {
				if cfg.FullSeason {
					epInfos = append(epInfos, EpisodeInfo{
						EpisodeNumber: ep.EpisodeNumber,
						Title:         ep.Title,
						AirDateUtc:    ep.AirDateUtc,
						HasFile:       ep.HasFile,
						Aired:         !ep.AirDateUtc.IsZero() && !ep.AirDateUtc.After(now),
					})
				}
				if ep.AirDateUtc.IsZero() {
					continue
				}
				if !ep.AirDateUtc.After(now) {
					t := ep.AirDateUtc
					lastAir = &t
					lastLabel = fmt.Sprintf("S%02dE%02d %q", ep.SeasonNumber, ep.EpisodeNumber, ep.Title)
				} else if nextAir == nil {
					t := ep.AirDateUtc
					nextAir = &t
					nextLabel = fmt.Sprintf("S%02dE%02d %q", ep.SeasonNumber, ep.EpisodeNumber, ep.Title)
				}
			}

			if cfg.FullSeason {
				info.SeasonEpisodes = epInfos
			}

			info.LastAir = lastAir
			info.LastLabel = lastLabel
			info.NextAir = nextAir
			info.NextLabel = nextLabel

			if info.NextAir == nil && info.LastAir != nil {
				info.Status = "ended"
			} else if info.NextAir != nil {
				info.Status = "ongoing"
			} else {
				info.Status = "tba"
			}
		} else {
			info.Status = "tba"
		}
	} else {
		info.Status = resolveMovieStatus(c, cfg)
		now := time.Now()

		var releaseTime time.Time
		if c.DigitalRelease != "" {
			releaseTime, _ = parseDateFlexible(c.DigitalRelease)
		}
		if releaseTime.IsZero() && c.PhysicalRelease != "" {
			releaseTime, _ = parseDateFlexible(c.PhysicalRelease)
		}
		if releaseTime.IsZero() && c.InCinemas != "" {
			releaseTime, _ = parseDateFlexible(c.InCinemas)
		}
		if !releaseTime.IsZero() {
			releaseTime = releaseTime.Local()
			if releaseTime.After(now) {
				info.NextAir = &releaseTime
				info.NextLabel = "Release date"
			} else {
				info.LastAir = &releaseTime
				info.LastLabel = "Release date"
			}
		}
	}

	return info
}

type episodeBySeason map[int][]SonarrEpisode

func groupEpisodesBySeason(episodes []SonarrEpisode) episodeBySeason {
	m := make(episodeBySeason)
	for _, ep := range episodes {
		m[ep.SeasonNumber] = append(m[ep.SeasonNumber], ep)
	}
	return m
}

func findCurrentSeason(seasons episodeBySeason) int {
	now := time.Now()
	bestSeason := 1
	bestDiff := time.Duration(1<<63 - 1)

	for sn, episodes := range seasons {
		for _, ep := range episodes {
			if ep.AirDateUtc.IsZero() {
				continue
			}
			diff := ep.AirDateUtc.Sub(now)
			if diff < 0 {
				diff = -diff
			}
			if diff < bestDiff {
				bestDiff = diff
				bestSeason = sn
			}
		}
	}
	return bestSeason
}

func countOnDisk(episodes []SonarrEpisode) int {
	n := 0
	for _, ep := range episodes {
		if ep.HasFile {
			n++
		}
	}
	return n
}

func resolveMovieStatus(c SeriesOrMovie, cfg ToolConfig) string {
	if c.Status == "released" || c.HasFile {
		return "released"
	}
	return "announced"
}

func parseDateFlexible(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %s", s)
}

func getMovieReleaseTime(m RadarrMovie) time.Time {
	var releaseTime time.Time
	if m.DigitalRelease != "" {
		releaseTime, _ = parseDateFlexible(m.DigitalRelease)
	}
	if releaseTime.IsZero() && m.PhysicalRelease != "" {
		releaseTime, _ = parseDateFlexible(m.PhysicalRelease)
	}
	if releaseTime.IsZero() && m.InCinemas != "" {
		releaseTime, _ = parseDateFlexible(m.InCinemas)
	}
	return releaseTime
}

func candidateToMap(c SeriesOrMovie, score int) map[string]interface{} {
	return map[string]interface{}{
		"type":      c.Type,
		"title":     c.Title,
		"year":      c.Year,
		"tvdb_id":   c.TVDBID,
		"tmdb_id":   c.TMDBID,
		"monitored": c.Monitored,
		"has_file":  c.HasFile,
		"status":    c.Status,
		"seasons":   c.Seasons,
		"source":    c.Source,
		"raw_id":    c.RawID,
		"score":     score,
	}
}

func infoToMap(i AirtimeInfo) map[string]interface{} {
	m := map[string]interface{}{
		"title":            i.Title,
		"year":             i.Year,
		"source":           i.Source,
		"type":             i.Type,
		"status":           i.Status,
		"season_count":     i.SeasonCount,
		"season":           i.Season,
		"episodes_on_disk": i.EpisodesOnDisk,
		"episodes_total":   i.EpisodesTotal,
		"monitored":        i.Monitored,
		"tvdb_id":          i.TVDBID,
		"tmdb_id":          i.TMDBID,
	}
	if i.LastAir != nil {
		m["last_air"] = i.LastAir.Format(time.RFC3339)
		m["last_label"] = i.LastLabel
	}
	if i.NextAir != nil {
		m["next_air"] = i.NextAir.Format(time.RFC3339)
		m["next_label"] = i.NextLabel
	}
	return m
}

func pickCandidate(ctx context.Context, candidates []scoredMatch, query string, cfg ToolConfig) *scoredMatch {
	if len(candidates) == 0 {
		return nil
	}

	if len(candidates) == 1 {
		return &candidates[0]
	}

	if cfg.JSONOutput {
		return &candidates[0]
	}

	return interactiveSelect(ctx, candidates, query, cfg)
}
