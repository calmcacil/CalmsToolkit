//go:build arrfeed

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[0;33m"
	ColorBlue   = "\033[0;34m"
	ColorCyan   = "\033[0;36m"
	ColorGray   = "\033[0;90m"
	ColorBold   = "\033[1m"
)

const (
	AnsiClearScreen = "\033[2J"
	AnsiHomeCursor  = "\033[H"
	AnsiHideCursor  = "\033[?25l"
	AnsiShowCursor  = "\033[?25h"
)

type Config struct {
	SonarrURLs    []string
	SonarrTokens  []string
	RadarrURLs    []string
	RadarrTokens  []string
	PollInterval  time.Duration
	HistoryWindow time.Duration
	Timeout       time.Duration
	NoColor       bool
	JSON          bool
	Watch         bool
	ShowGrabbed   bool
	ShowImported  bool
	ShowFailed    bool
	ShowDeleted   bool
	ShowIgnored   bool
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

func main() {
	var (
		sonarrURLs    = flag.String("sonarr-urls", "", "Sonarr URLs (comma-separated)")
		sonarrTokens  = flag.String("sonarr-tokens", "", "Sonarr API tokens (comma-separated)")
		radarrURLs    = flag.String("radarr-urls", "", "Radarr URLs (comma-separated)")
		radarrTokens  = flag.String("radarr-tokens", "", "Radarr API tokens (comma-separated)")
		pollInterval  = flag.Duration("poll", 5*time.Second, "Poll interval for watch mode")
		historyWindow = flag.Duration("duration", 1*time.Hour, "History lookback window")
		timeout       = flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
		noColor       = flag.Bool("no-color", false, "Disable colored output")
		jsonOutput    = flag.Bool("json", false, "Output JSON instead of table")
		watch         = flag.Bool("watch", false, "Continuous monitoring mode")
		showGrabbed   = flag.Bool("show-grabbed", true, "Show grabbed events")
		showImported  = flag.Bool("show-imported", true, "Show imported events")
		showFailed    = flag.Bool("show-failed", true, "Show failed events")
		showDeleted   = flag.Bool("show-deleted", true, "Show deleted events")
		showIgnored   = flag.Bool("show-ignored", false, "Show ignored events")
	)
	flag.Parse()

	config := loadConfig(*sonarrURLs, *sonarrTokens, *radarrURLs, *radarrTokens, *pollInterval, *historyWindow, *timeout, *noColor, *jsonOutput, *watch, *showGrabbed, *showImported, *showFailed, *showDeleted, *showIgnored)

	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if config.Watch {
		runWatchMode(config)
	} else {
		runSingleMode(config)
	}
}

func loadConfig(sonarrURLs, sonarrTokens, radarrURLs, radarrTokens string, pollInterval, historyWindow, timeout time.Duration, noColor, jsonOutput, watch, showGrabbed, showImported, showFailed, showDeleted, showIgnored bool) Config {
	config := Config{
		SonarrURLs:    []string{},
		SonarrTokens:  []string{},
		RadarrURLs:    []string{},
		RadarrTokens:  []string{},
		PollInterval:  pollInterval,
		HistoryWindow: historyWindow,
		Timeout:       timeout,
		NoColor:       noColor,
		JSON:          jsonOutput,
		Watch:         watch,
		ShowGrabbed:   showGrabbed,
		ShowImported:  showImported,
		ShowFailed:    showFailed,
		ShowDeleted:   showDeleted,
		ShowIgnored:   showIgnored,
	}

	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, &config)
	}

	if envSonarrURLs := os.Getenv("SONARR_URLS"); envSonarrURLs != "" {
		config.SonarrURLs = strings.Split(envSonarrURLs, ",")
		for i := range config.SonarrURLs {
			config.SonarrURLs[i] = strings.TrimSpace(config.SonarrURLs[i])
		}
	}
	if envSonarrTokens := os.Getenv("SONARR_TOKENS"); envSonarrTokens != "" {
		config.SonarrTokens = strings.Split(envSonarrTokens, ",")
		for i := range config.SonarrTokens {
			config.SonarrTokens[i] = strings.TrimSpace(config.SonarrTokens[i])
		}
	}
	if envRadarrURLs := os.Getenv("RADARR_URLS"); envRadarrURLs != "" {
		config.RadarrURLs = strings.Split(envRadarrURLs, ",")
		for i := range config.RadarrURLs {
			config.RadarrURLs[i] = strings.TrimSpace(config.RadarrURLs[i])
		}
	}
	if envRadarrTokens := os.Getenv("RADARR_TOKENS"); envRadarrTokens != "" {
		config.RadarrTokens = strings.Split(envRadarrTokens, ",")
		for i := range config.RadarrTokens {
			config.RadarrTokens[i] = strings.TrimSpace(config.RadarrTokens[i])
		}
	}

	if envPoll := os.Getenv("ARR_FEED_POLL_INTERVAL"); envPoll != "" {
		if d, err := time.ParseDuration(envPoll); err == nil {
			config.PollInterval = d
		}
	}
	if envDuration := os.Getenv("ARR_FEED_HISTORY_DURATION"); envDuration != "" {
		if d, err := time.ParseDuration(envDuration); err == nil {
			config.HistoryWindow = d
		}
	}
	if envTimeout := os.Getenv("ARR_FEED_TIMEOUT"); envTimeout != "" {
		if d, err := time.ParseDuration(envTimeout); err == nil {
			config.Timeout = d
		}
	}

	if sonarrURLs != "" {
		config.SonarrURLs = strings.Split(sonarrURLs, ",")
		for i := range config.SonarrURLs {
			config.SonarrURLs[i] = strings.TrimSpace(config.SonarrURLs[i])
		}
	}
	if sonarrTokens != "" {
		config.SonarrTokens = strings.Split(sonarrTokens, ",")
		for i := range config.SonarrTokens {
			config.SonarrTokens[i] = strings.TrimSpace(config.SonarrTokens[i])
		}
	}
	if radarrURLs != "" {
		config.RadarrURLs = strings.Split(radarrURLs, ",")
		for i := range config.RadarrURLs {
			config.RadarrURLs[i] = strings.TrimSpace(config.RadarrURLs[i])
		}
	}
	if radarrTokens != "" {
		config.RadarrTokens = strings.Split(radarrTokens, ",")
		for i := range config.RadarrTokens {
			config.RadarrTokens[i] = strings.TrimSpace(config.RadarrTokens[i])
		}
	}

	for i := range config.SonarrURLs {
		config.SonarrURLs[i] = strings.TrimSuffix(config.SonarrURLs[i], "/")
	}
	for i := range config.RadarrURLs {
		config.RadarrURLs[i] = strings.TrimSuffix(config.RadarrURLs[i], "/")
	}

	return config
}

func loadEnvFile(path string, config *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "SONARR_URLS":
			config.SonarrURLs = strings.Split(value, ",")
			for i := range config.SonarrURLs {
				config.SonarrURLs[i] = strings.TrimSpace(config.SonarrURLs[i])
			}
		case "SONARR_TOKENS":
			config.SonarrTokens = strings.Split(value, ",")
			for i := range config.SonarrTokens {
				config.SonarrTokens[i] = strings.TrimSpace(config.SonarrTokens[i])
			}
		case "RADARR_URLS":
			config.RadarrURLs = strings.Split(value, ",")
			for i := range config.RadarrURLs {
				config.RadarrURLs[i] = strings.TrimSpace(config.RadarrURLs[i])
			}
		case "RADARR_TOKENS":
			config.RadarrTokens = strings.Split(value, ",")
			for i := range config.RadarrTokens {
				config.RadarrTokens[i] = strings.TrimSpace(config.RadarrTokens[i])
			}
		case "ARR_FEED_POLL_INTERVAL":
			if d, err := time.ParseDuration(value); err == nil {
				config.PollInterval = d
			}
		case "ARR_FEED_HISTORY_DURATION":
			if d, err := time.ParseDuration(value); err == nil {
				config.HistoryWindow = d
			}
		case "ARR_FEED_TIMEOUT":
			if d, err := time.ParseDuration(value); err == nil {
				config.Timeout = d
			}
		}
	}
}

func validateConfig(config Config) error {
	if len(config.SonarrURLs) == 0 && len(config.RadarrURLs) == 0 {
		return fmt.Errorf("at least one Sonarr or Radarr instance must be configured")
	}

	if len(config.SonarrURLs) != len(config.SonarrTokens) {
		return fmt.Errorf("number of Sonarr URLs (%d) must match number of tokens (%d)", len(config.SonarrURLs), len(config.SonarrTokens))
	}

	if len(config.RadarrURLs) != len(config.RadarrTokens) {
		return fmt.Errorf("number of Radarr URLs (%d) must match number of tokens (%d)", len(config.RadarrURLs), len(config.RadarrTokens))
	}

	return nil
}

func runSingleMode(config Config) {
	since := time.Now().Add(-config.HistoryWindow)
	events, err := fetchAllHistory(config, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	events = filterEvents(events, config)

	if config.JSON {
		renderJSON(events)
	} else {
		renderTable(events, config)
	}
}

func runWatchMode(config Config) {
	if !config.JSON {
		fmt.Print(AnsiHideCursor)
		defer fmt.Print(AnsiShowCursor)
	}

	eventCache := make([]HistoryEvent, 0, 100)
	lastFetch := time.Now().Add(-config.HistoryWindow)

	for {
		newEvents, err := fetchAllHistory(config, lastFetch)
		if err != nil {
			if !config.JSON {
				clearScreen()
				color := getColorFunc(config)
				fmt.Printf("%sERROR: %v%s\n", color(ColorRed), err, color(ColorReset))
				fmt.Printf("Retrying in %v...\n", config.PollInterval)
			}
		} else {
			for _, event := range newEvents {
				eventCache = append(eventCache, event)
			}

			if len(eventCache) > 100 {
				eventCache = eventCache[len(eventCache)-100:]
			}

			filteredEvents := filterEvents(eventCache, config)

			if config.JSON {
				renderJSON(filteredEvents)
			} else {
				clearScreen()
				renderTable(filteredEvents, config)
			}

			if len(newEvents) > 0 {
				lastFetch = time.Now()
			}
		}

		time.Sleep(config.PollInterval)
	}
}

func fetchAllHistory(config Config, since time.Time) ([]HistoryEvent, error) {
	var wg sync.WaitGroup
	eventsChan := make(chan []HistoryEvent, len(config.SonarrURLs)+len(config.RadarrURLs))
	errorsChan := make(chan error, len(config.SonarrURLs)+len(config.RadarrURLs))

	for i := range config.SonarrURLs {
		wg.Add(1)
		go func(url, token string) {
			defer wg.Done()
			events, err := fetchSonarrHistory(config, url, token, since)
			if err != nil {
				errorsChan <- fmt.Errorf("Sonarr (%s): %v", url, err)
				return
			}
			eventsChan <- events
		}(config.SonarrURLs[i], config.SonarrTokens[i])
	}

	for i := range config.RadarrURLs {
		wg.Add(1)
		go func(url, token string) {
			defer wg.Done()
			events, err := fetchRadarrHistory(config, url, token, since)
			if err != nil {
				errorsChan <- fmt.Errorf("Radarr (%s): %v", url, err)
				return
			}
			eventsChan <- events
		}(config.RadarrURLs[i], config.RadarrTokens[i])
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

func fetchSonarrHistory(config Config, url, token string, since time.Time) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeEpisode=true&includeSeries=true", url, sinceStr)

	client := &http.Client{Timeout: config.Timeout}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var history []SonarrHistory
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, sonarrToHistoryEvent(h))
	}

	return events, nil
}

func fetchRadarrHistory(config Config, url, token string, since time.Time) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeMovie=true", url, sinceStr)

	client := &http.Client{Timeout: config.Timeout}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var history []RadarrHistory
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
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

	if len(sh.Quality.CustomFormats) > 0 {
		event.Formats = make([]string, len(sh.Quality.CustomFormats))
		for i, f := range sh.Quality.CustomFormats {
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

	if len(rh.Quality.CustomFormats) > 0 {
		event.Formats = make([]string, len(rh.Quality.CustomFormats))
		for i, f := range rh.Quality.CustomFormats {
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

func filterEvents(events []HistoryEvent, config Config) []HistoryEvent {
	filtered := make([]HistoryEvent, 0, len(events))

	for _, event := range events {
		switch event.Action {
		case "Grabbed":
			if !config.ShowGrabbed {
				continue
			}
		case "Imported", "Bulk Import":
			if !config.ShowImported {
				continue
			}
		case "Failed":
			if !config.ShowFailed {
				continue
			}
		case "Deleted":
			if !config.ShowDeleted {
				continue
			}
		case "Ignored":
			if !config.ShowIgnored {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	return filtered
}

func renderTable(events []HistoryEvent, config Config) {
	color := getColorFunc(config)

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

func getColorFunc(config Config) func(string) string {
	if config.NoColor || config.JSON {
		return func(s string) string { return "" }
	}
	return func(s string) string { return s }
}

func clearScreen() {
	fmt.Print(AnsiHomeCursor)
	fmt.Print(AnsiClearScreen)
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
