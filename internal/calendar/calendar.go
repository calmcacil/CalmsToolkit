package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

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

type Series struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SeasonCount int    `json:"seasonCount"`
}

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

type QueueResponse struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

type QueueItem struct {
	ID             int             `json:"id"`
	Status         string          `json:"status"`
	TrackedState   string          `json:"trackedDownloadState"`
	StatusMessages []StatusMessage `json:"statusMessages"`
}

type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

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

type ToolConfig struct {
	SonarrInstances []config.ArrInstance
	RadarrInstances []config.ArrInstance
	Timeout         time.Duration
	Days            int
	DaysPast        int
	NoColor         bool
	JSONOutput      bool
	WatchMode       bool
	WatchSeconds    int
	Debug           bool
	NoBanner        bool
	Quiet           bool
	Filter          string
	MonitoredOnly   bool
}

type QueueIssue struct {
	ServiceName string
	URL         string
	Count       int
}

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

func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.Days = 1
		cfg.WatchSeconds = 300
		return cfg
	}

	dur, err := time.ParseDuration(tk.General.Timeout)
	if err != nil || dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor

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

func calculateDateRange(days, daysPast int) (start, end time.Time) {
	now := time.Now()
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if daysPast > 0 {
		start = start.AddDate(0, 0, -daysPast)
	}
	end = start.AddDate(0, 0, days+daysPast)
	return start, end
}

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

	ctx := context.Background()
	if err := runWithSubagents(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func aggregateCalendar(ctx context.Context, cfg ToolConfig) ([]CalendarItem, []QueueIssue, error) {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        20,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
		},
	}

	items := make([]CalendarItem, 0)
	queueIssues := make([]QueueIssue, 0)
	seenEpisodes := make(map[string]bool)
	seenMovies := make(map[string]bool)
	var mu sync.Mutex
	var qMu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)

	for _, inst := range cfg.SonarrInstances {
		inst := inst
		g.Go(func() error {
			return fetchSonarrInstance(gCtx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenEpisodes)
		})
	}

	for _, inst := range cfg.RadarrInstances {
		inst := inst
		g.Go(func() error {
			return fetchRadarrInstance(gCtx, client, inst, start, end, cfg.Debug, &mu, &qMu, &items, &queueIssues, seenMovies)
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AirTime.Before(items[j].AirTime)
	})

	return items, queueIssues, nil
}

func fetchSonarrInstance(ctx context.Context, client *http.Client, inst config.ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
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
			AirTime:        ep.AirDateUtc,
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

func fetchRadarrInstance(ctx context.Context, client *http.Client, inst config.ArrInstance, start, end time.Time, debug bool, mu, qMu *sync.Mutex, items *[]CalendarItem, queueIssues *[]QueueIssue, seen map[string]bool) error {
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
			releaseTime, err = time.Parse("2006-01-02T15:04:05Z", movie.DigitalDate)
		}
		if releaseTime.IsZero() && movie.ReleaseDate != "" {
			releaseTime, err = time.Parse("2006-01-02T15:04:05Z", movie.ReleaseDate)
		}
		if releaseTime.IsZero() && movie.InCinemas != "" {
			releaseTime, err = time.Parse("2006-01-02T15:04:05Z", movie.InCinemas)
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
			AirTime:        releaseTime,
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

func fetchSonarrCalendar(ctx context.Context, client *http.Client, inst config.ArrInstance, start, end time.Time, debug bool) ([]SonarrEpisode, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Sonarr %s: GET %s\n", inst.Name, apiURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", inst.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var episodes []SonarrEpisode
	if err := json.Unmarshal(body, &episodes); err != nil {
		return nil, err
	}

	return episodes, nil
}

func fetchRadarrCalendar(ctx context.Context, client *http.Client, inst config.ArrInstance, start, end time.Time, debug bool) ([]RadarrMovie, error) {
	apiURL := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s",
		inst.URL, start.Format("2006-01-02"), end.Format("2006-01-02"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Radarr %s: GET %s\n", inst.Name, apiURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", inst.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var movies []RadarrMovie
	if err := json.Unmarshal(body, &movies); err != nil {
		return nil, err
	}

	return movies, nil
}

func fetchQueue(ctx context.Context, client *http.Client, inst config.ArrInstance, debug bool) (*QueueResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v3/queue?pageSize=1", inst.URL)

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Queue %s: GET %s\n", inst.Name, apiURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", inst.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var queue QueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, err
	}

	return &queue, nil
}

func displayJSON(items []CalendarItem, queueIssues []QueueIssue, cfg ToolConfig) error {
	start, end := calculateDateRange(cfg.Days, cfg.DaysPast)
	now := time.Now()

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

func getStatusColor(item CalendarItem, now time.Time, noColor bool) string {
	if noColor {
		return ""
	}

	if item.HasFile {
		return colors.Green
	}

	if item.AirTime.Before(now) {
		return colors.Red
	}

	if item.IsPremiere {
		return colors.Orange
	}

	return colors.Blue
}
