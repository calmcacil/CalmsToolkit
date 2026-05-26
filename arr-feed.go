//go:build arrfeed

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type FeedToolConfig struct {
	SonarrInstances []ArrInstance
	RadarrInstances []ArrInstance
	PollInterval    time.Duration
	HistoryWindow   time.Duration
	Timeout         time.Duration
	NoColor         bool
	JSON            bool
	Watch           bool
	ShowGrabbed     bool
	ShowImported    bool
	ShowFailed      bool
	ShowDeleted     bool
	ShowIgnored     bool
	MaxEvents       int
	Quiet           bool
}

type HistoryEvent struct {
	Server       string    `json:"server"`
	When         time.Time `json:"when"`
	Action       string    `json:"action"`
	Title        string    `json:"title"`
	Episode      string    `json:"episode,omitempty"`
	EpisodeTitle string    `json:"episodeTitle,omitempty"`
	Quality      string    `json:"quality"`
	Formats      []string  `json:"formats,omitempty"`
	SourceTitle  string    `json:"sourceTitle,omitempty"`
	EventType    string    `json:"eventType"`
	ID           int       `json:"id"`
}

type SonarrHistory struct {
	EpisodeID     int                    `json:"episodeId"`
	SeriesID      int                    `json:"seriesId"`
	SourceTitle   string                 `json:"sourceTitle"`
	Quality       SonarrQuality          `json:"quality"`
	CustomFormats []CustomFormat         `json:"customFormats,omitempty"`
	QualityCutoff bool                   `json:"qualityCutoffNotMet"`
	Date          string                 `json:"date"`
	EventType     string                 `json:"eventType"`
	Data          map[string]interface{} `json:"data"`
	Episode       *SonarrEpisode         `json:"episode,omitempty"`
	Series        *SonarrSeries          `json:"series,omitempty"`
	ID            int                    `json:"id"`
}

type SonarrQuality struct {
	Quality       SonarrQualityItem `json:"quality"`
	CustomFormats []CustomFormat    `json:"customFormats"`
	Revision      QualityRevision   `json:"revision,omitempty"`
}

type SonarrQualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Resolution int    `json:"resolution,omitempty"`
}

type CustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type QualityRevision struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

type SonarrEpisode struct {
	ID            int    `json:"id"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
}

type SonarrSeries struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type RadarrHistory struct {
	MovieID       int                    `json:"movieId"`
	SourceTitle   string                 `json:"sourceTitle"`
	Quality       RadarrQuality          `json:"quality"`
	CustomFormats []CustomFormat         `json:"customFormats,omitempty"`
	QualityCutoff bool                   `json:"qualityCutoffNotMet"`
	Date          string                 `json:"date"`
	EventType     string                 `json:"eventType"`
	Data          map[string]interface{} `json:"data"`
	Movie         *RadarrMovie           `json:"movie,omitempty"`
	ID            int                    `json:"id"`
}

type RadarrQuality struct {
	Quality       RadarrQualityItem `json:"quality"`
	CustomFormats []CustomFormat    `json:"customFormats"`
	Revision      QualityRevision   `json:"revision,omitempty"`
}

type RadarrQualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Resolution int    `json:"resolution,omitempty"`
}

type RadarrMovie struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

func BuildFeedToolConfig(tk *ToolkitConfig) FeedToolConfig {
	cfg := FeedToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.PollInterval = 5 * time.Second
		cfg.HistoryWindow = 1 * time.Hour
		cfg.MaxEvents = 50
		cfg.ShowGrabbed = true
		cfg.ShowImported = true
		cfg.ShowFailed = true
		return cfg
	}

	dur, _ := time.ParseDuration(tk.General.Timeout)
	if dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor
	cfg.SonarrInstances = slices.Clone(tk.Sonarr)
	cfg.RadarrInstances = slices.Clone(tk.Radarr)

	dur, _ = time.ParseDuration(tk.ArrFeed.PollInterval)
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

	cfg.MaxEvents = tk.ArrFeed.MaxEvents
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = 50
	}
	if cfg.MaxEvents > 100 {
		cfg.MaxEvents = 100
	}
	cfg.ShowGrabbed = tk.ArrFeed.ShowGrabbed
	cfg.ShowImported = tk.ArrFeed.ShowImported
	cfg.ShowFailed = tk.ArrFeed.ShowFailed
	cfg.ShowDeleted = tk.ArrFeed.ShowDeleted
	cfg.ShowIgnored = tk.ArrFeed.ShowIgnored
	return cfg
}

func main() {
	tk, err := LoadToolkitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := BuildFeedToolConfig(tk)

	poll := flag.Duration("poll", cfg.PollInterval, "Poll interval for watch mode")
	duration := flag.Duration("duration", cfg.HistoryWindow, "History lookback window")
	timeout := flag.Duration("timeout", cfg.Timeout, "HTTP request timeout")
	noColor := flag.Bool("no-color", cfg.NoColor, "Disable colored output")
	json := flag.Bool("json", false, "Output JSON instead of table")
	watch := flag.Bool("watch", false, "Continuous monitoring mode")
	showGrabbed := flag.Bool("show-grabbed", cfg.ShowGrabbed, "Show grabbed events")
	showImported := flag.Bool("show-imported", cfg.ShowImported, "Show imported events")
	showFailed := flag.Bool("show-failed", cfg.ShowFailed, "Show failed events")
	showDeleted := flag.Bool("show-deleted", cfg.ShowDeleted, "Show deleted events")
	showIgnored := flag.Bool("show-ignored", cfg.ShowIgnored, "Show ignored events")
	maxEvents := flag.Int("events", cfg.MaxEvents, "Maximum number of events to display (1-100)")
	quiet := flag.Bool("quiet", false, "Suppress error output in watch mode")
	flag.Parse()

	cfg.PollInterval = *poll
	cfg.HistoryWindow = *duration
	cfg.Timeout = *timeout
	cfg.NoColor = *noColor || *json
	cfg.JSON = *json
	cfg.Watch = *watch
	cfg.ShowGrabbed = *showGrabbed
	cfg.ShowImported = *showImported
	cfg.ShowFailed = *showFailed
	cfg.ShowDeleted = *showDeleted
	cfg.ShowIgnored = *showIgnored
	cfg.MaxEvents = *maxEvents
	cfg.Quiet = *quiet

	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No Sonarr or Radarr instances configured\n")
		fmt.Fprintf(os.Stderr, "Run 'make setup' or edit ~/.config/calmstoolkit/config.json\n")
		os.Exit(1)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if cfg.Watch {
		runWatchMode(ctx, cfg, client)
	} else {
		runSingleMode(ctx, cfg, client)
	}
}

func validateConfig(cfg FeedToolConfig) error {
	if len(cfg.SonarrInstances) == 0 && len(cfg.RadarrInstances) == 0 {
		return fmt.Errorf("at least one Sonarr or Radarr instance must be configured")
	}
	for i, inst := range cfg.SonarrInstances {
		if inst.URL == "" {
			return fmt.Errorf("Sonarr instance %d: url is required", i)
		}
		if inst.APIKey == "" {
			return fmt.Errorf("Sonarr instance %d: api_key is required", i)
		}
	}
	for i, inst := range cfg.RadarrInstances {
		if inst.URL == "" {
			return fmt.Errorf("Radarr instance %d: url is required", i)
		}
		if inst.APIKey == "" {
			return fmt.Errorf("Radarr instance %d: api_key is required", i)
		}
	}
	return nil
}

func runSingleMode(ctx context.Context, cfg FeedToolConfig, client *http.Client) {
	since := time.Now().Add(-cfg.HistoryWindow)
	events, err := fetchAllHistory(ctx, client, cfg, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	events = filterEvents(events, cfg)

	if cfg.JSON {
		renderJSON(events)
	} else {
		renderTable(events, cfg)
	}
}

func runWatchMode(ctx context.Context, cfg FeedToolConfig, client *http.Client) {
	if !cfg.JSON {
		fmt.Print(AnsiHideCursor)
		defer fmt.Print(AnsiShowCursor)
	}

	eventCache := make([]HistoryEvent, 0, 100)
	lastFetch := time.Now().Add(-cfg.HistoryWindow)

	for {
		newEvents, err := fetchAllHistory(ctx, client, cfg, lastFetch)
		if err != nil {
			if !cfg.JSON {
				fmt.Print(AnsiClearScreen + AnsiHomeCursor)
				color := getColorFunc(cfg)
				fmt.Printf("%sERROR: %v%s\n", color(ColorRed), err, color(ColorReset))
				fmt.Printf("Retrying in %v...\n", cfg.PollInterval)
			}
		} else {
			for _, event := range newEvents {
				eventCache = append(eventCache, event)
			}

			if len(eventCache) > 100 {
				eventCache = eventCache[len(eventCache)-100:]
			}

			filteredEvents := filterEvents(eventCache, cfg)

			if len(filteredEvents) > cfg.MaxEvents {
				filteredEvents = filteredEvents[:cfg.MaxEvents]
			}

			if cfg.JSON {
				renderJSON(filteredEvents)
			} else {
				fmt.Print(AnsiClearScreen + AnsiHomeCursor)
				renderTable(filteredEvents, cfg)
			}

			if len(newEvents) > 0 {
				lastFetch = time.Now()
			}
		}

		select {
		case <-ctx.Done():
			if !cfg.JSON {
				fmt.Print(AnsiShowCursor)
			}
			return
		case <-time.After(cfg.PollInterval):
		}
	}
}

func fetchAllHistory(ctx context.Context, client *http.Client, cfg FeedToolConfig, since time.Time) ([]HistoryEvent, error) {
	var wg sync.WaitGroup
	eventsChan := make(chan []HistoryEvent, len(cfg.SonarrInstances)+len(cfg.RadarrInstances))
	errorsChan := make(chan error, len(cfg.SonarrInstances)+len(cfg.RadarrInstances))

	for _, inst := range cfg.SonarrInstances {
		wg.Add(1)
		go func(inst ArrInstance) {
			defer wg.Done()
			events, err := fetchSonarrHistory(ctx, client, inst, since)
			if err != nil {
				errorsChan <- fmt.Errorf("Sonarr %s: %v", inst.Name, err)
				return
			}
			eventsChan <- events
		}(inst)
	}

	for _, inst := range cfg.RadarrInstances {
		wg.Add(1)
		go func(inst ArrInstance) {
			defer wg.Done()
			events, err := fetchRadarrHistory(ctx, client, inst, since)
			if err != nil {
				errorsChan <- fmt.Errorf("Radarr %s: %v", inst.Name, err)
				return
			}
			eventsChan <- events
		}(inst)
	}

	wg.Wait()
	close(eventsChan)
	close(errorsChan)

	var allEvents []HistoryEvent
	for events := range eventsChan {
		allEvents = append(allEvents, events...)
	}

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(allEvents) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all instances failed: %v", errors)
	}

	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].When.After(allEvents[j].When)
	})

	return allEvents, nil
}

func fetchSonarrHistory(ctx context.Context, client *http.Client, inst ArrInstance, since time.Time) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeEpisode=true&includeSeries=true", inst.URL, sinceStr)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", inst.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Records []SonarrHistory `json:"records"`
	}
	var history []SonarrHistory
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Records != nil {
		history = wrapper.Records
	} else if err := json.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, sonarrToHistoryEvent(h))
	}

	return events, nil
}

func fetchRadarrHistory(ctx context.Context, client *http.Client, inst ArrInstance, since time.Time) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeMovie=true", inst.URL, sinceStr)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", inst.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Records []RadarrHistory `json:"records"`
	}
	var history []RadarrHistory
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Records != nil {
		history = wrapper.Records
	} else if err := json.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, radarrToHistoryEvent(h))
	}

	return events, nil
}

func sonarrToHistoryEvent(sh SonarrHistory) HistoryEvent {
	when, _ := time.Parse(time.RFC3339, sh.Date)

	event := HistoryEvent{
		Server:      "sonarr",
		When:        when,
		Action:      mapSonarrEventType(sh.EventType),
		Quality:     sh.Quality.Quality.Name,
		SourceTitle: sh.SourceTitle,
		EventType:   sh.EventType,
		ID:          sh.ID,
	}

	if sh.Series != nil {
		event.Title = sh.Series.Title
	}

	if sh.Episode != nil {
		event.Episode = formatEpisode(sh.Episode.SeasonNumber, sh.Episode.EpisodeNumber)
		event.EpisodeTitle = sh.Episode.Title
	}

	var cf []CustomFormat
	if len(sh.CustomFormats) > 0 {
		cf = sh.CustomFormats
	} else {
		cf = sh.Quality.CustomFormats
	}
	if len(cf) > 0 {
		event.Formats = make([]string, len(cf))
		for i, f := range cf {
			event.Formats[i] = f.Name
		}
	}

	return event
}

func radarrToHistoryEvent(rh RadarrHistory) HistoryEvent {
	when, _ := time.Parse(time.RFC3339, rh.Date)

	event := HistoryEvent{
		Server:      "radarr",
		When:        when,
		Action:      mapRadarrEventType(rh.EventType),
		Quality:     rh.Quality.Quality.Name,
		SourceTitle: rh.SourceTitle,
		EventType:   rh.EventType,
		ID:          rh.ID,
	}

	if rh.Movie != nil {
		event.Title = rh.Movie.Title
		if rh.Movie.Year > 0 {
			event.Title = fmt.Sprintf("%s (%d)", rh.Movie.Title, rh.Movie.Year)
		}
	}

	var cf []CustomFormat
	if len(rh.CustomFormats) > 0 {
		cf = rh.CustomFormats
	} else {
		cf = rh.Quality.CustomFormats
	}
	if len(cf) > 0 {
		event.Formats = make([]string, len(cf))
		for i, f := range cf {
			event.Formats[i] = f.Name
		}
	}

	return event
}

func mapSonarrEventType(eventType string) string {
	switch eventType {
	case "grabbed":
		return "Grabbed"
	case "downloadFolderImported":
		return "Imported"
	case "downloadFailed":
		return "Failed"
	case "episodeFileDeleted":
		return "Deleted"
	case "episodeFileRenamed":
		return "Renamed"
	case "downloadIgnored":
		return "Ignored"
	case "seriesFolderImported":
		return "Bulk Import"
	default:
		return eventType
	}
}

func mapRadarrEventType(eventType string) string {
	switch eventType {
	case "grabbed":
		return "Grabbed"
	case "downloadFolderImported":
		return "Imported"
	case "downloadFailed":
		return "Failed"
	case "movieFileDeleted":
		return "Deleted"
	case "movieFileRenamed":
		return "Renamed"
	case "downloadIgnored":
		return "Ignored"
	default:
		return eventType
	}
}

func formatEpisode(season, episode int) string {
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "Just now"
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	return t.Format("2006-01-02 15:04")
}

func filterEvents(events []HistoryEvent, cfg FeedToolConfig) []HistoryEvent {
	filtered := make([]HistoryEvent, 0, len(events))

	for _, event := range events {
		switch event.Action {
		case "Grabbed":
			if !cfg.ShowGrabbed {
				continue
			}
		case "Imported", "Bulk Import":
			if !cfg.ShowImported {
				continue
			}
		case "Failed":
			if !cfg.ShowFailed {
				continue
			}
		case "Deleted":
			if !cfg.ShowDeleted {
				continue
			}
		case "Ignored":
			if !cfg.ShowIgnored {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	return filtered
}

func renderTable(events []HistoryEvent, cfg FeedToolConfig) {
	color := getColorFunc(cfg)

	fmt.Printf("%s%-15s | %-10s | %-30s | %-10s | %-40s | %-15s | %-20s%s\n",
		color(ColorBold),
		"When", "Action", "Series/Movie", "Episode", "Episode Title", "Quality", "Formats",
		color(ColorReset))

	fmt.Printf("%s%s%s\n",
		color(ColorBold),
		strings.Repeat("-", 160),
		color(ColorReset))

	if len(events) == 0 {
		fmt.Printf("%sNo events found%s\n", color(ColorGray), color(ColorReset))
		return
	}

	for _, event := range events {
		actionColor := getActionColor(event.Action)
		timeStr := formatRelativeTime(event.When)
		title := truncate(event.Title, 30)
		episodeTitle := truncate(event.EpisodeTitle, 40)
		quality := truncate(event.Quality, 15)
		formats := truncate(strings.Join(event.Formats, ", "), 20)

		fmt.Printf("%-15s | %s%-10s%s | %-30s | %-10s | %-40s | %-15s | %-20s\n",
			timeStr,
			color(actionColor),
			center(event.Action, 10),
			color(ColorReset),
			center(title, 30),
			center(event.Episode, 10),
			center(episodeTitle, 40),
			center(quality, 15),
			center(formats, 20))
	}

	fmt.Printf("\n%sTotal events: %d%s\n", color(ColorBold), len(events), color(ColorReset))
}

func renderJSON(events []HistoryEvent) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(events)
}

func getActionColor(action string) string {
	switch action {
	case "Imported", "Bulk Import":
		return ColorGreen
	case "Grabbed":
		return ColorCyan
	case "Failed":
		return ColorRed
	case "Deleted":
		return ColorYellow
	case "Ignored":
		return ColorGray
	case "Renamed":
		return ColorBlue
	default:
		return ColorReset
	}
}

func getColorFunc(cfg FeedToolConfig) func(string) string {
	if cfg.NoColor || cfg.JSON {
		return func(s string) string { return "" }
	}
	return func(s string) string { return s }
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func center(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	padding := width - len(s)
	leftPad := padding / 2
	rightPad := padding - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
