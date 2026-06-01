package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/core"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

// SonarrEpisode represents a Sonarr calendar episode.
type SonarrEpisode struct {
	SeriesID      int       `json:"seriesId"`
	EpisodeID     int       `json:"id"`
	Title         string    `json:"title"`
	AirDate       string    `json:"airDate"`
	AirDateUtc    time.Time `json:"airDateUtc"`
	SeasonNumber  int       `json:"seasonNumber"`
	EpisodeNumber int       `json:"episodeNumber"`
	Monitored     bool      `json:"monitored"`
	HasFile       bool      `json:"hasFile"`
	Series        *Series   `json:"series"`
}

// Series represents a TV series from Sonarr.
type Series struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SeasonCount int    `json:"seasonCount"`
}

// RadarrMovie represents a movie from the Radarr calendar.
type RadarrMovie struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Year        int    `json:"year"`
	ReleaseDate string `json:"physicalRelease"`
	DigitalDate string `json:"digitalRelease"`
	InCinemas   string `json:"inCinemas"`
	Monitored   bool   `json:"monitored"`
	HasFile     bool   `json:"hasFile"`
}

// QueueResponse is the wrapper for download queue responses from Sonarr/Radarr.
type QueueResponse struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// QueueItem represents a single item in the download queue.
type QueueItem struct {
	ID             int             `json:"id"`
	Status         string          `json:"status"`
	TrackedState   string          `json:"trackedDownloadState"`
	StatusMessages []StatusMessage `json:"statusMessages"`
}

// StatusMessage describes a queue item status message.
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// CalendarItem is a unified display item representing an episode or movie.
type CalendarItem struct {
	Type           string
	Title          string
	ShowTitle      string
	Year           int
	Season         int
	Episode        int
	AirTime        time.Time
	HasFile        bool
	Monitored      bool
	IsPremiere     bool
	SourceInstance string
}

// ToolConfig holds configuration for the media calendar tool.
type ToolConfig struct {
	core.CommonConfig
	SonarrInstances []config.ArrInstance
	RadarrInstances []config.ArrInstance
	Days            int
	DaysPast        int
	NoBanner        bool
	Filter          string
	MonitoredOnly   bool
}

// QueueIssue represents a warning about a queue item needing intervention.
type QueueIssue struct {
	ServiceName string
	URL         string
	Count       int
}

// Summary provides an overview of calendar items.
type Summary struct {
	StartDate   string         `json:"start_date"`
	EndDate     string         `json:"end_date"`
	TotalItems  int            `json:"total_items"`
	Episodes    int            `json:"episodes"`
	Movies      int            `json:"movies"`
	Available   int            `json:"available"`
	Missing     int            `json:"missing"`
	Timestamp   time.Time      `json:"timestamp"`
	QueueIssues []QueueIssue   `json:"queue_issues,omitempty"`
	Items       []CalendarItem `json:"items"`
}

type fetchEpisodeResult struct {
	Instance string
	Episodes []SonarrEpisode
	Err      error
}

type fetchMovieResult struct {
	Instance string
	Movies   []RadarrMovie
	Err      error
}

type fetchQueueResult struct {
	Instance string
	Queue    *QueueResponse
	Err      error
}

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{
		CommonConfig: core.FromToolkit(tk),
	}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.Days = 1
		cfg.WatchSeconds = 300
		cfg.Watch = false
		return cfg
	}

	cfg.SonarrInstances = slices.Clone(tk.Sonarr)
	cfg.RadarrInstances = slices.Clone(tk.Radarr)

	if tk.MediaCalendar.Days > 0 {
		cfg.Days = tk.MediaCalendar.Days
	} else {
		cfg.Days = 1
	}
	cfg.DaysPast = tk.MediaCalendar.DaysPast
	if tk.MediaCalendar.WatchInterval > 0 {
		cfg.WatchSeconds = tk.MediaCalendar.WatchInterval
	} else {
		cfg.WatchSeconds = 300
	}
	cfg.Debug = tk.MediaCalendar.Debug

	return cfg
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

func calculateDateRange(days, daysPast int) (start, end time.Time) {
	now := time.Now()
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if daysPast > 0 {
		start = start.AddDate(0, 0, -daysPast)
	}
	end = start.AddDate(0, 0, days+daysPast)
	return start, end
}

// Run executes the media calendar tool.
func Run(cfg ToolConfig) {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	if cfg.JSONOutput {
		ctx := context.Background()
		items, issues, err := aggregateCalendar(ctx, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		if err := displayJSON(items, issues, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		return
	}

	p := colors.GetPalette(cfg.Theme)
	ctx := context.Background()
	if err := runWithSubagents(ctx, cfg, p); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func aggregateCalendar(ctx context.Context, cfg ToolConfig) ([]CalendarItem, []QueueIssue, error) {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)

	client := httputil.NewTransportClient(cfg.Timeout)

	items := make([]CalendarItem, 0)
	queueIssues := make([]QueueIssue, 0)
	seenEpisodes := make(map[string]bool)
	seenMovies := make(map[string]bool)
	var mu sync.Mutex
	var qMu sync.Mutex

	var g errgroup.Group

	totalSources := len(cfg.SonarrInstances) + len(cfg.RadarrInstances)
	var successes int
	var successMu sync.Mutex

	for _, inst := range cfg.SonarrInstances {
		inst := inst
		g.Go(func() error {
			err := fetchSonarrInstance(ctx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenEpisodes)
			if err == nil {
				successMu.Lock()
				successes++
				successMu.Unlock()
			}
			return err
		})
	}

	for _, inst := range cfg.RadarrInstances {
		inst := inst
		g.Go(func() error {
			err := fetchRadarrInstance(ctx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenMovies)
			if err == nil {
				successMu.Lock()
				successes++
				successMu.Unlock()
			}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}

	if successes == 0 && totalSources > 0 {
		return items, queueIssues, fmt.Errorf("all %d source(s) failed to return data", totalSources)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AirTime.Before(items[j].AirTime)
	})

	return items, queueIssues, nil
}

func fetchSonarrInstance(ctx context.Context, client *httputil.Client, inst config.ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
	episodes, err := fetchSonarrCalendar(ctx, client, inst, start, end, debug)
	if err != nil {
		return fmt.Errorf("Sonarr %s: %w", inst.Name, err)
	}

	mu.Lock()
	for _, ep := range episodes {
		if ep.Series == nil {
			continue
		}
		key := fmt.Sprintf("%s-S%02dE%02d", ep.Series.Title, ep.SeasonNumber, ep.EpisodeNumber)
		if seen[key] {
			continue
		}
		seen[key] = true

		item := CalendarItem{
			Type:           "episode",
			Title:          ep.Title,
			ShowTitle:      ep.Series.Title,
			Year:           ep.Series.Year,
			Season:         ep.SeasonNumber,
			Episode:        ep.EpisodeNumber,
			AirTime:        ep.AirDateUtc.Local(),
			HasFile:        ep.HasFile,
			Monitored:      ep.Monitored,
			IsPremiere:     ep.SeasonNumber == 1 && ep.EpisodeNumber == 1,
			SourceInstance: inst.Name,
		}
		*items = append(*items, item)
	}
	mu.Unlock()

	queue, err := fetchQueue(ctx, client, inst, debug)
	if err != nil {
		return nil
	}

	if queue != nil {
		errorCount := 0
		for _, qItem := range queue.Records {
			if qItem.TrackedState == "importFailed" || qItem.TrackedState == "importPending" ||
				len(qItem.StatusMessages) > 0 {
				errorCount++
			}
		}
		if errorCount > 0 {
			qMu.Lock()
			*queueIssues = append(*queueIssues, QueueIssue{
				ServiceName: inst.Name,
				URL:         inst.URL + "/activity/queue",
				Count:       errorCount,
			})
			qMu.Unlock()
		}
	}

	return nil
}

func fetchRadarrInstance(ctx context.Context, client *httputil.Client, inst config.ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
	movies, err := fetchRadarrCalendar(ctx, client, inst, start, end, debug)
	if err != nil {
		return fmt.Errorf("Radarr %s: %w", inst.Name, err)
	}

	mu.Lock()
	for _, movie := range movies {
		key := fmt.Sprintf("%s-%d", movie.Title, movie.Year)
		if seen[key] {
			continue
		}
		seen[key] = true

		var releaseTime time.Time
		if movie.DigitalDate != "" {
			releaseTime, err = parseDateFlexible(movie.DigitalDate)
		}
		if releaseTime.IsZero() && movie.ReleaseDate != "" {
			releaseTime, err = parseDateFlexible(movie.ReleaseDate)
		}
		if releaseTime.IsZero() && movie.InCinemas != "" {
			releaseTime, err = parseDateFlexible(movie.InCinemas)
		}
		if releaseTime.IsZero() {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG: Skipping %s (%d) - no valid release date\n", movie.Title, movie.Year)
			}
			continue
		}

		item := CalendarItem{
			Type:           "movie",
			Title:          movie.Title,
			Year:           movie.Year,
			AirTime:        releaseTime.Local(),
			HasFile:        movie.HasFile,
			Monitored:      movie.Monitored,
			IsPremiere:     false,
			SourceInstance: inst.Name,
		}
		*items = append(*items, item)
	}
	mu.Unlock()

	queue, err := fetchQueue(ctx, client, inst, debug)
	if err != nil {
		return nil
	}

	if queue != nil {
		errorCount := 0
		for _, qItem := range queue.Records {
			if qItem.TrackedState == "importFailed" || qItem.TrackedState == "importPending" ||
				len(qItem.StatusMessages) > 0 {
				errorCount++
			}
		}
		if errorCount > 0 {
			qMu.Lock()
			*queueIssues = append(*queueIssues, QueueIssue{
				ServiceName: inst.Name,
				URL:         inst.URL + "/activity/queue",
				Count:       errorCount,
			})
			qMu.Unlock()
		}
	}

	return nil
}

func fetchSonarrCalendar(ctx context.Context, client *httputil.Client, inst config.ArrInstance, start, end time.Time, debug bool) ([]SonarrEpisode, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: GET %s\n", inst.Name, apiURL)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var episodes []SonarrEpisode
	if err := client.DoJSON(ctx, "GET", apiURL, headers, nil, &episodes); err != nil {
		return nil, err
	}
	return episodes, nil
}

func fetchRadarrCalendar(ctx context.Context, client *httputil.Client, inst config.ArrInstance, start, end time.Time, debug bool) ([]RadarrMovie, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Radarr %s: GET %s\n", inst.Name, apiURL)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var movies []RadarrMovie
	if err := client.DoJSON(ctx, "GET", apiURL, headers, nil, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func fetchQueue(ctx context.Context, client *httputil.Client, inst config.ArrInstance, debug bool) (*QueueResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v3/queue?pageSize=100", inst.URL)

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Queue %s: GET %s\n", inst.Name, apiURL)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	var queue QueueResponse
	if err := client.DoJSON(ctx, "GET", apiURL, headers, nil, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

func displayJSON(items []CalendarItem, queueIssues []QueueIssue, cfg ToolConfig) error {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)
	now := time.Now()

	items = applyFilters(items, cfg)

	episodes := 0
	movies := 0
	available := 0
	missing := 0

	for _, item := range items {
		if item.Type == "episode" {
			episodes++
		} else {
			movies++
		}
		if item.HasFile {
			available++
		} else if item.AirTime.Before(now) {
			missing++
		}
	}

	summary := Summary{
		StartDate:   start.Format("2006-01-02"),
		EndDate:     end.Format("2006-01-02"),
		TotalItems:  len(items),
		Episodes:    episodes,
		Movies:      movies,
		Available:   available,
		Missing:     missing,
		Timestamp:   now,
		QueueIssues: queueIssues,
		Items:       items,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

func applyFilters(items []CalendarItem, cfg ToolConfig) []CalendarItem {
	if cfg.MonitoredOnly {
		filtered := make([]CalendarItem, 0, len(items))
		for _, item := range items {
			if item.Monitored {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	if cfg.Filter != "" {
		filters := strings.Split(cfg.Filter, ",")
		filtered := make([]CalendarItem, 0, len(items))
		for _, item := range items {
			match := false
			for _, f := range filters {
				switch strings.TrimSpace(strings.ToLower(f)) {
				case "missing":
					if !item.HasFile && item.AirTime.Before(time.Now()) {
						match = true
					}
				case "available":
					if item.HasFile {
						match = true
					}
				case "premieres":
					if item.IsPremiere {
						match = true
					}
				case "monitored":
					if item.Monitored {
						match = true
					}
				}
				if match {
					break
				}
			}
			if match {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	return items
}

func getStatusColor(item CalendarItem, now time.Time, p *colors.Palette) string {
	if item.HasFile {
		return p.Success
	}

	if item.AirTime.Before(now) {
		return p.Error
	}

	if item.IsPremiere {
		return p.Premiere
	}

	return p.Info
}
